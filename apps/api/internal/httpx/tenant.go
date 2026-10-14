package httpx

import (
    "context"
    "log"
    "net/http"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
)

type txKeyType int

const (
    txKey      txKeyType = 1
    queriesKey txKeyType = 2
)

func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
    tx, ok := ctx.Value(txKey).(pgx.Tx)
    return tx, ok
}

func QueriesFromContext(ctx context.Context) (*db.Queries, bool) {
    q, ok := ctx.Value(queriesKey).(*db.Queries)
    return q, ok
}

func MustQueries(ctx context.Context) *db.Queries {
    q, ok := QueriesFromContext(ctx)
    if !ok {
        panic("httpx: no queries in context")
    }
    return q
}

// WithTenantTx opens a transaction, sets app.tenant_id and app.user_id as
// transaction-local settings, and stashes a *db.Queries bound to that tx.
// The transaction commits if the handler returns a status < 400.
func WithTenantTx(pool *pgxpool.Pool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := auth.FromContext(r.Context())
            if !ok {
                Error(w, http.StatusUnauthorized, "unauthorized", "no claims in context")
                return
            }

            tx, err := pool.BeginTx(r.Context(), pgx.TxOptions{})
            if err != nil {
                log.Printf("httpx: begin tx: %v", err)
                Error(w, http.StatusServiceUnavailable, "db_unavailable", "")
                return
            }
            committed := false
            defer func() {
                if !committed {
                    _ = tx.Rollback(context.Background())
                }
            }()

            if _, err := tx.Exec(r.Context(),
                "SELECT set_config('app.tenant_id', $1, true), set_config('app.user_id', $2, true)",
                claims.TenantID.String(), claims.UserID.String(),
            ); err != nil {
                log.Printf("httpx: set_config: %v", err)
                Error(w, http.StatusInternalServerError, "tenant_setup_failed", "")
                return
            }

            q := db.New(tx)
            ctx := context.WithValue(r.Context(), txKey, tx)
            ctx = context.WithValue(ctx, queriesKey, q)

            sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(sw, r.WithContext(ctx))

            if sw.status < 400 {
                if err := tx.Commit(context.Background()); err != nil {
                    log.Printf("httpx: commit: %v", err)
                    return
                }
                committed = true
            }
        })
    }
}