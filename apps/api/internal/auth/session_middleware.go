package auth

import (
    "context"
    "log"
    "net/http"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// SessionMiddleware reads the session cookie, resolves the tenant and user
// via the SECURITY DEFINER function, then loads roles and permissions inside
// a short transaction. If no valid cookie is present it does nothing and
// lets the next middleware decide (typically DevAuthMiddleware).
func SessionMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Respect an already-authenticated context.
            if _, ok := FromContext(r.Context()); ok {
                next.ServeHTTP(w, r)
                return
            }

            c, err := r.Cookie(SessionCookieName)
            if err != nil || c.Value == "" {
                next.ServeHTTP(w, r)
                return
            }

            sess, err := ResolveSessionFromToken(r.Context(), pool, c.Value)
            if err != nil {
                next.ServeHTTP(w, r)
                return
            }

            tx, err := pool.BeginTx(r.Context(), pgx.TxOptions{})
            if err != nil {
                log.Printf("auth: session begin tx: %v", err)
                next.ServeHTTP(w, r)
                return
            }
            defer tx.Rollback(context.Background())

            if _, err := tx.Exec(r.Context(),
                `SELECT set_config('app.tenant_id', $1, true)`,
                sess.TenantID.String(),
            ); err != nil {
                log.Printf("auth: session set_config: %v", err)
                next.ServeHTTP(w, r)
                return
            }

            var tenantSlug string
            if err := tx.QueryRow(r.Context(),
                `SELECT slug FROM tenants WHERE id = $1`, sess.TenantID,
            ).Scan(&tenantSlug); err != nil {
                log.Printf("auth: session tenant slug: %v", err)
                next.ServeHTTP(w, r)
                return
            }

            roles, err := loadRoles(r.Context(), tx, sess.UserID)
            if err != nil {
                log.Printf("auth: session roles: %v", err)
                next.ServeHTTP(w, r)
                return
            }
            perms, err := loadPermissions(r.Context(), tx, sess.UserID)
            if err != nil {
                log.Printf("auth: session permissions: %v", err)
                next.ServeHTTP(w, r)
                return
            }

            claims := &Claims{
                TenantID:    sess.TenantID,
                TenantSlug:  tenantSlug,
                UserID:      sess.UserID,
                UserEmail:   sess.Email,
                UserRoles:   roles,
                Permissions: perms,
            }
            next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
        })
    }
}