package auth

import (
    "context"
    "net/http"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// DevAuthMiddleware authenticates from X-Dev-Tenant (slug) and
// X-Dev-User-Email headers. It runs its own short-lived transaction so that
// RLS on users is satisfied: it resolves the tenant via the SECURITY DEFINER
// function resolve_tenant_slug, then sets app.tenant_id for the user lookup.
// Only mounted when cfg.DevAuth is true.
//
// If claims have already been set by an earlier middleware (e.g. the session
// middleware resolved a valid session cookie), this middleware is a no-op.
func DevAuthMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if _, ok := FromContext(r.Context()); ok {
                next.ServeHTTP(w, r)
                return
            }

            slug := r.Header.Get("X-Dev-Tenant")
            email := r.Header.Get("X-Dev-User-Email")
            if slug == "" || email == "" {
                writeUnauthorized(w, "missing X-Dev-Tenant or X-Dev-User-Email")
                return
            }

            tx, err := pool.BeginTx(r.Context(), pgx.TxOptions{})
            if err != nil {
                writeUnauthorized(w, "db unavailable")
                return
            }
            defer tx.Rollback(context.Background())

            var tenantID uuid.UUID
            var tenantSlug string
            if err := tx.QueryRow(r.Context(),
                `SELECT id, slug FROM resolve_tenant_slug($1)`, slug,
            ).Scan(&tenantID, &tenantSlug); err != nil {
                writeUnauthorized(w, "unknown tenant")
                return
            }

            if _, err := tx.Exec(r.Context(),
                `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String(),
            ); err != nil {
                writeUnauthorized(w, "tenant setup failed")
                return
            }

            var userID uuid.UUID
            var userEmail string
            if err := tx.QueryRow(r.Context(),
                `SELECT id, email FROM users WHERE email = $1 AND deleted_at IS NULL`,
                email,
            ).Scan(&userID, &userEmail); err != nil {
                writeUnauthorized(w, "unknown user")
                return
            }

            roles, err := loadRoles(r.Context(), tx, userID)
            if err != nil {
                writeUnauthorized(w, "roles load failed")
                return
            }
            perms, err := loadPermissions(r.Context(), tx, userID)
            if err != nil {
                writeUnauthorized(w, "permissions load failed")
                return
            }

            claims := &Claims{
                TenantID:    tenantID,
                TenantSlug:  tenantSlug,
                UserID:      userID,
                UserEmail:   userEmail,
                UserRoles:   roles,
                Permissions: perms,
            }
            next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
        })
    }
}

func loadRoles(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]string, error) {
    rows, err := tx.Query(ctx,
        `SELECT r.key
         FROM user_roles ur
         JOIN roles r ON r.id = ur.role_id
         WHERE ur.user_id = $1
         ORDER BY r.key`, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []string
    for rows.Next() {
        var k string
        if err := rows.Scan(&k); err != nil {
            return nil, err
        }
        out = append(out, k)
    }
    return out, rows.Err()
}

func loadPermissions(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]string, error) {
    rows, err := tx.Query(ctx,
        `SELECT DISTINCT rp.permission
         FROM user_roles ur
         JOIN role_permissions rp ON rp.role_id = ur.role_id
         WHERE ur.user_id = $1
         ORDER BY rp.permission`, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []string
    for rows.Next() {
        var p string
        if err := rows.Scan(&p); err != nil {
            return nil, err
        }
        out = append(out, p)
    }
    return out, rows.Err()
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusUnauthorized)
    _, _ = w.Write([]byte(`{"error":"unauthorized","message":"` + msg + `"}`))
}