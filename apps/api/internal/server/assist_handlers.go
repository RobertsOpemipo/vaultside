package server

import (
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/assist"
    "github.com/vaultside/vaultside/apps/api/internal/auth"
)

// AssistHandlers serves the AI guide: ask, status, and the per-tenant switch.
type AssistHandlers struct {
    pool        *pgxpool.Pool
    service     *assist.Service
    idleTimeout time.Duration
    devAuth     bool
}

// NewAssistHandlers builds an AssistHandlers. Unknown provider names fall
// back to noop so a misconfigured env var does not break the server.
func NewAssistHandlers(pool *pgxpool.Pool, opts Options) *AssistHandlers {
    p, err := assist.New(opts.AssistProvider)
    if err != nil {
        slog.Warn("assist: unknown provider, falling back to noop",
            "provider", opts.AssistProvider, "err", err)
        p, _ = assist.New("noop")
    }
    return &AssistHandlers{
        pool:        pool,
        service:     assist.NewService(p),
        idleTimeout: opts.IdleTimeout,
        devAuth:     opts.DevAuth,
    }
}

// Mount registers the routes on the top-level router. Called once from
// server.go: assistHandlers.Mount(r).
func (h *AssistHandlers) Mount(r chi.Router) {
    r.Group(func(r chi.Router) {
        r.Use(auth.SessionMiddleware(h.pool, h.idleTimeout))
        if h.devAuth {
            r.Use(auth.DevAuthMiddleware(h.pool))
        }
        r.Get("/v1/assist/status", h.Status)
        r.Post("/v1/assist/ask", h.Ask)
        r.Get("/v1/admin/assist", h.AdminGet)
        r.Patch("/v1/admin/assist", h.AdminPatch)
    })
}

// Status tells the frontend whether to show the side panel. Any authenticated
// user may call it.
func (h *AssistHandlers) Status(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }
    enabled, err := h.tenantAssistEnabled(ctx, tx)
    if err != nil {
        slog.Error("assist status: flag check", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "flag check failed")
        return
    }
    if err := tx.Commit(ctx); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, map[string]any{
        "enabled":  enabled,
        "provider": h.service.ProviderName(),
        "model":    h.service.ModelName(),
    })
}

// Ask answers a question about a lesson. Requires the tenant flag on.
func (h *AssistHandlers) Ask(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    var body struct {
        LessonID string `json:"lesson_id"`
        Question string `json:"question"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        scormHTTPError(w, http.StatusBadRequest, "invalid json")
        return
    }
    if body.Question == "" {
        scormHTTPError(w, http.StatusBadRequest, "question required")
        return
    }
    lessonID, err := uuid.Parse(body.LessonID)
    if err != nil {
        scormHTTPError(w, http.StatusBadRequest, "invalid lesson_id")
        return
    }

    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }
    enabled, err := h.tenantAssistEnabled(ctx, tx)
    if err != nil {
        slog.Error("assist ask: flag check", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "flag check failed")
        return
    }
    if !enabled {
        scormHTTPError(w, http.StatusForbidden, "assist is not enabled for this tenant")
        return
    }

    res, err := h.service.Ask(ctx, tx, assist.AskInput{
        LessonID: lessonID,
        Question: body.Question,
    })
    if err != nil {
        slog.Error("assist ask", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "ask failed")
        return
    }

    detail, _ := json.Marshal(map[string]any{
        "lesson_id": lessonID.String(),
        "question":  body.Question,
        "citations": res.Citations,
        "provider":  res.Provider,
    })
    if _, err := tx.Exec(ctx, `
        INSERT INTO audit_log (tenant_id, actor_id, action, entity_type, entity_id, detail)
        VALUES ($1, $2, 'assist.ask', 'lesson', $3, $4)
    `, claims.TenantID, claims.UserID, lessonID, detail); err != nil {
        slog.Warn("assist ask: audit log", "err", err)
    }

    if err := tx.Commit(ctx); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, res)
}

// AdminGet returns the current flag. Tenant owner only.
func (h *AssistHandlers) AdminGet(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }
    allowed, err := h.hasPermission(ctx, tx, claims.UserID, "tenant.manage")
    if err != nil {
        slog.Error("assist admin get: perm check", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "perm check failed")
        return
    }
    if !allowed {
        scormHTTPError(w, http.StatusForbidden, "tenant owner only")
        return
    }
    enabled, err := h.tenantAssistEnabled(ctx, tx)
    if err != nil {
        slog.Error("assist admin get: flag check", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "flag check failed")
        return
    }
    if err := tx.Commit(ctx); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, map[string]any{
        "enabled":  enabled,
        "provider": h.service.ProviderName(),
        "model":    h.service.ModelName(),
    })
}

// AdminPatch flips the flag. Tenant owner only.
func (h *AssistHandlers) AdminPatch(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    var body struct {
        Enabled *bool `json:"enabled"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        scormHTTPError(w, http.StatusBadRequest, "invalid json")
        return
    }
    if body.Enabled == nil {
        scormHTTPError(w, http.StatusBadRequest, "enabled required")
        return
    }

    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }
    allowed, err := h.hasPermission(ctx, tx, claims.UserID, "tenant.manage")
    if err != nil {
        slog.Error("assist admin patch: perm check", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "perm check failed")
        return
    }
    if !allowed {
        scormHTTPError(w, http.StatusForbidden, "tenant owner only")
        return
    }

    if _, err := tx.Exec(ctx,
        `UPDATE tenants SET assist_enabled = $1 WHERE id = $2`,
        *body.Enabled, claims.TenantID); err != nil {
        slog.Error("assist admin patch: update", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "update failed")
        return
    }

    detail, _ := json.Marshal(map[string]any{"enabled": *body.Enabled})
    if _, err := tx.Exec(ctx, `
        INSERT INTO audit_log (tenant_id, actor_id, action, entity_type, entity_id, detail)
        VALUES ($1, $2, 'assist.toggle', 'tenant', $3, $4)
    `, claims.TenantID, claims.UserID, claims.TenantID, detail); err != nil {
        slog.Warn("assist admin patch: audit log", "err", err)
    }

    if err := tx.Commit(ctx); err != nil {
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, map[string]any{"enabled": *body.Enabled})
}

// ── helpers ───────────────────────────────────────────────────────────────

func (h *AssistHandlers) tenantAssistEnabled(ctx context.Context, tx pgx.Tx) (bool, error) {
    var enabled bool
    err := tx.QueryRow(ctx, `SELECT assist_enabled FROM tenants LIMIT 1`).Scan(&enabled)
    if errors.Is(err, pgx.ErrNoRows) {
        return false, nil
    }
    return enabled, err
}

func (h *AssistHandlers) hasPermission(ctx context.Context, tx pgx.Tx, userID uuid.UUID, perm string) (bool, error) {
    var exists bool
    err := tx.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM user_roles ur
            JOIN role_permissions rp ON rp.role_id = ur.role_id
            WHERE ur.user_id = $1 AND rp.permission = $2
        )
    `, userID, perm).Scan(&exists)
    return exists, err
}