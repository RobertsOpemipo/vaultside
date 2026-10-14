package server

import (
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/scorm"
)

// ScormHandlers wires the SCORM runtime into the HTTP surface.
type ScormHandlers struct {
    pool     *pgxpool.Pool
    runtime  *scorm.Runtime
    secret   []byte
    tokenTTL time.Duration
    now      func() time.Time
}

// NewScormHandlers builds a ScormHandlers.
func NewScormHandlers(pool *pgxpool.Pool, secret []byte) *ScormHandlers {
    return &ScormHandlers{
        pool:     pool,
        runtime:  &scorm.Runtime{},
        secret:   secret,
        tokenTTL: 4 * time.Hour,
        now:      time.Now,
    }
}

type scormLaunchRequest struct {
    LessonID string        `json:"lesson_id"`
    Version  scorm.Version `json:"version,omitempty"`
}

type scormCommitRequest struct {
    CMI map[string]string `json:"cmi"`
}

// ── session-authenticated ──────────────────────────────────────────────────

// Launch opens or resumes a SCORM attempt. Requires the session middleware
// (auth.FromContext must be populated).
func (h *ScormHandlers) Launch(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    var req scormLaunchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        scormHTTPError(w, http.StatusBadRequest, "invalid json")
        return
    }
    lessonID, err := uuid.Parse(req.LessonID)
    if err != nil {
        scormHTTPError(w, http.StatusBadRequest, "invalid lesson_id")
        return
    }
    ver := req.Version
    if ver == "" {
        ver = scorm.V12
    }

    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        slog.Error("scorm launch: begin", "err", err)
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        slog.Error("scorm launch: set tenant", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }

    res, err := h.runtime.Launch(ctx, tx, scorm.LaunchInput{
        TenantID: claims.TenantID,
        UserID:   claims.UserID,
        LessonID: lessonID,
        Version:  ver,
    }, h.secret, h.now(), h.tokenTTL)
    switch {
    case errors.Is(err, scorm.ErrNotFound):
        scormHTTPError(w, http.StatusNotFound, "lesson or SCORM package not found")
        return
    case errors.Is(err, scorm.ErrNoEnrollment):
        scormHTTPError(w, http.StatusForbidden, "no active enrollment for this course")
        return
    case err != nil:
        slog.Error("scorm launch", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "launch failed")
        return
    }
    if err := tx.Commit(ctx); err != nil {
        slog.Error("scorm launch: commit", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusCreated, res)
}

// ── token-authenticated ────────────────────────────────────────────────────

// Get returns the current CMI snapshot.
func (h *ScormHandlers) Get(w http.ResponseWriter, r *http.Request) {
    tx, claims, ok := scormFromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    attemptID, err := scormAttemptFromURL(r, claims)
    if err != nil {
        scormHTTPError(w, http.StatusForbidden, "attempt mismatch")
        return
    }
    snap, err := h.runtime.Get(r.Context(), tx, attemptID)
    if errors.Is(err, scorm.ErrNotFound) {
        scormHTTPError(w, http.StatusNotFound, "not found")
        return
    }
    if err != nil {
        slog.Error("scorm get", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "load failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, snap)
}

// Commit persists a batch of CMI keys.
func (h *ScormHandlers) Commit(w http.ResponseWriter, r *http.Request) {
    h.applyBatch(w, r, false)
}

// Finish persists the final batch and marks the attempt complete.
func (h *ScormHandlers) Finish(w http.ResponseWriter, r *http.Request) {
    h.applyBatch(w, r, true)
}

// Abandon flags an in-progress attempt as abandoned.
func (h *ScormHandlers) Abandon(w http.ResponseWriter, r *http.Request) {
    tx, claims, ok := scormFromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    attemptID, err := scormAttemptFromURL(r, claims)
    if err != nil {
        scormHTTPError(w, http.StatusForbidden, "attempt mismatch")
        return
    }
    if err := h.runtime.Abandon(r.Context(), tx, attemptID); err != nil {
        slog.Error("scorm abandon", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "abandon failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *ScormHandlers) applyBatch(w http.ResponseWriter, r *http.Request, finish bool) {
    tx, claims, ok := scormFromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    attemptID, err := scormAttemptFromURL(r, claims)
    if err != nil {
        scormHTTPError(w, http.StatusForbidden, "attempt mismatch")
        return
    }
    var req scormCommitRequest
    if r.Body != nil {
        _ = json.NewDecoder(r.Body).Decode(&req) // empty body is fine
    }
    ctx := r.Context()
    var snap *scorm.Snapshot
    if finish {
        snap, err = h.runtime.Finish(ctx, tx, attemptID, req.CMI, h.now())
    } else {
        snap, err = h.runtime.Commit(ctx, tx, attemptID, req.CMI, h.now())
    }
    switch {
    case errors.Is(err, scorm.ErrNotFound):
        scormHTTPError(w, http.StatusNotFound, "not found")
        return
    case errors.Is(err, scorm.ErrBadKey):
        scormHTTPError(w, http.StatusBadRequest, err.Error())
        return
    case err != nil:
        slog.Error("scorm apply", "err", err, "finish", finish)
        scormHTTPError(w, http.StatusInternalServerError, "apply failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, snap)
}

// ── token middleware ───────────────────────────────────────────────────────

type scormCtxKey int

const (
    scormCtxTx scormCtxKey = iota
    scormCtxClaims
)

func scormFromContext(ctx context.Context) (pgx.Tx, *scorm.AttemptClaims, bool) {
    tx, _ := ctx.Value(scormCtxTx).(pgx.Tx)
    c, _ := ctx.Value(scormCtxClaims).(*scorm.AttemptClaims)
    if tx == nil || c == nil {
        return nil, nil, false
    }
    return tx, c, true
}

// RequireToken validates the short-lived SCORM attempt token, opens a
// tenant-scoped transaction, and stashes both on the context.
func (h *ScormHandlers) RequireToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        raw := scormBearerToken(r)
        if raw == "" {
            scormHTTPError(w, http.StatusUnauthorized, "missing token")
            return
        }
        claims, err := scorm.ParseAttemptToken(h.secret, raw, h.now())
        if err != nil {
            scormHTTPError(w, http.StatusUnauthorized, "invalid token")
            return
        }
        ctx := r.Context()
        tx, err := h.pool.Begin(ctx)
        if err != nil {
            slog.Error("scorm middleware: begin", "err", err)
            scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
            return
        }
        defer tx.Rollback(ctx)
        if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
            slog.Error("scorm middleware: set tenant", "err", err)
            scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
            return
        }
        ctx = context.WithValue(ctx, scormCtxTx, tx)
        ctx = context.WithValue(ctx, scormCtxClaims, claims)

        rec := &scormStatusRecorder{ResponseWriter: w, status: 200}
        next.ServeHTTP(rec, r.WithContext(ctx))
        if rec.status < 400 {
            _ = tx.Commit(ctx)
        }
    })
}

// ── helpers ────────────────────────────────────────────────────────────────

func scormBearerToken(r *http.Request) string {
    h := r.Header.Get("Authorization")
    if strings.HasPrefix(h, "Bearer ") {
        return strings.TrimSpace(h[7:])
    }
    return strings.TrimSpace(r.Header.Get("X-Scorm-Token"))
}

func scormAttemptFromURL(r *http.Request, claims *scorm.AttemptClaims) (uuid.UUID, error) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        return uuid.Nil, err
    }
    if id != claims.AttemptID {
        return uuid.Nil, errors.New("attempt mismatch")
    }
    return id, nil
}

func scormSetTenant(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) error {
    _, err := tx.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, true), set_config('app.user_id', $2, true)",
        tenantID.String(), userID.String())
    return err
}

type scormStatusRecorder struct {
    http.ResponseWriter
    status int
}

func (s *scormStatusRecorder) WriteHeader(code int) {
    s.status = code
    s.ResponseWriter.WriteHeader(code)
}

func (s *scormStatusRecorder) Write(b []byte) (int, error) {
    if s.status == 0 {
        s.status = 200
    }
    return s.ResponseWriter.Write(b)
}

func scormWriteJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}

func scormHTTPError(w http.ResponseWriter, status int, msg string) {
    scormWriteJSON(w, status, map[string]string{"error": msg})
}