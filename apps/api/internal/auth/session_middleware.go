package auth

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// SessionMiddleware resolves the session cookie, enforces an idle timeout
// in addition to the absolute expiry the DB already checks, and populates
// claims with roles and permissions.
func SessionMiddleware(pool *pgxpool.Pool, idleTimeout time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

            if idleTimeout > 0 && time.Since(sess.LastSeenAt) > idleTimeout {
                log.Printf("auth: session %s idle for %s, revoking",
                    sess.SessionID, time.Since(sess.LastSeenAt))
                _, _ = pool.Exec(r.Context(),
                    `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`,
                    sess.SessionID)
                next.ServeHTTP(w, r)
                return
            }

            // Best-effort touch, rate-limited server-side to once per 5 min.
            _, _ = pool.Exec(r.Context(),
                `SELECT touch_session($1)`, sess.SessionID)

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
                SessionID:   sess.SessionID,
                UserRoles:   roles,
                Permissions: perms,
            }
            next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
        })
    }
}