package handlers

import (
    "context"
    "log"
    "net/http"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

// Logout revokes the current session server-side and clears the cookie.
// Always returns 200, even if the cookie is missing or already invalid.
func Logout(pool *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        c, err := r.Cookie(auth.SessionCookieName)
        if err == nil && c.Value != "" {
            if sess, err := auth.ResolveSessionFromToken(r.Context(), pool, c.Value); err == nil {
                tx, err := pool.Begin(r.Context())
                if err == nil {
                    if _, err := tx.Exec(r.Context(),
                        `SELECT set_config('app.tenant_id', $1, true)`,
                        sess.TenantID.String(),
                    ); err == nil {
                        q := db.New(tx)
                        if err := q.RevokeSession(r.Context(), sess.SessionID); err != nil {
                            log.Printf("logout: revoke: %v", err)
                        }
                    }
                    _ = tx.Commit(context.Background())
                }
            }
        }
        auth.ClearSessionCookie(w)
        httpx.JSON(w, http.StatusOK, map[string]any{"logged_out": true})
    }
}