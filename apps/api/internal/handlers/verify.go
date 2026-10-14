package handlers

import (
    "context"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type verifyResponse struct {
    Valid        bool   `json:"valid"`
    VerifyCode   string `json:"verify_code"`
    HolderName   string `json:"holder_name,omitempty"`
    CourseTitle  string `json:"course_title,omitempty"`
    CourseKind   string `json:"course_kind,omitempty"`
    IssuedAt     string `json:"issued_at,omitempty"`
    ExpiresAt    string `json:"expires_at,omitempty"`
    Revoked      bool   `json:"revoked"`
    RevokedAt    string `json:"revoked_at,omitempty"`
}

// PublicVerifyCertificate resolves the tenant from the URL slug, opens a
// transaction with app.tenant_id set, and looks up the certificate by code.
// No authentication: this is the endpoint the QR code points at.
func PublicVerifyCertificate(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
    slug := chi.URLParam(r, "tenantSlug")
    code := chi.URLParam(r, "code")
    if slug == "" || code == "" {
        httpx.Error(w, http.StatusBadRequest, "bad_request", "")
        return
    }

    ctx := r.Context()
    tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        httpx.Error(w, http.StatusServiceUnavailable, "db_unavailable", "")
        return
    }
    defer tx.Rollback(context.Background())

    var tenantID uuid.UUID
    if err := tx.QueryRow(ctx,
        `SELECT id FROM resolve_tenant_slug($1)`, slug,
    ).Scan(&tenantID); err != nil {
        httpx.Error(w, http.StatusNotFound, "not_found", "unknown tenant")
        return
    }

    if _, err := tx.Exec(ctx,
        `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String(),
    ); err != nil {
        httpx.Error(w, http.StatusInternalServerError, "tenant_setup_failed", "")
        return
    }

    q := db.New(tx)
    row, err := q.GetCertificateByVerifyCode(ctx, code)
    if err != nil {
        httpx.Error(w, http.StatusNotFound, "not_found", "certificate not found")
        return
    }

    resp := verifyResponse{
        Valid:       !row.RevokedAt.Valid,
        VerifyCode:  row.VerifyCode,
        HolderName:  row.UserFullName,
        CourseTitle: row.CourseTitle,
        CourseKind:  row.CourseKind,
        IssuedAt:    row.IssuedAt.Format("2006-01-02"),
        Revoked:     row.RevokedAt.Valid,
    }
    if row.ExpiresAt.Valid {
        resp.ExpiresAt = row.ExpiresAt.Time.Format("2006-01-02")
    }
    if row.RevokedAt.Valid {
        resp.RevokedAt = row.RevokedAt.Time.Format("2006-01-02")
    }

    httpx.JSON(w, http.StatusOK, resp)
    _ = tx.Commit(context.Background())
}