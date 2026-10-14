package server_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/server"
)

const harbourlineAdaID = "44444444-0000-7000-8000-000000000001"

// newAdminPool returns a pool connecting as the schema owner (superuser).
// This bypasses RLS and does not hit the append-only revokes on xp_ledger
// and audit_log, so it can clean up test state.
func newAdminPool(t *testing.T) *pgxpool.Pool {
    t.Helper()
    url := os.Getenv("DATABASE_URL_ADMIN")
    if url == "" {
        t.Fatalf("DATABASE_URL_ADMIN is required for gamification tests")
    }
    p, err := pgxpool.New(context.Background(), url)
    if err != nil {
        t.Fatalf("admin pool: %v", err)
    }
    t.Cleanup(func() { p.Close() })
    return p
}

// clearGamificationState removes every xp, badge, quest, and streak row for
// a user. Runs as the admin role so append-only tables can be reset.
func clearGamificationState(t *testing.T, admin *pgxpool.Pool, userID string) {
    t.Helper()
    ctx := context.Background()
    stmts := []string{
        `DELETE FROM xp_ledger   WHERE user_id = $1`,
        `DELETE FROM user_badges WHERE user_id = $1`,
        `DELETE FROM user_quests WHERE user_id = $1`,
        `DELETE FROM streaks     WHERE user_id = $1`,
    }
    for _, s := range stmts {
        if _, err := admin.Exec(ctx, s, userID); err != nil {
            t.Fatalf("clear state %q: %v", s, err)
        }
    }
}

// seedXPForUser inserts n XP awards for a user via the admin role.
func seedXPForUser(t *testing.T, admin *pgxpool.Pool, tenantID, userID string, n int, delta int32) {
    t.Helper()
    ctx := context.Background()
    for i := 0; i < n; i++ {
        if _, err := admin.Exec(ctx,
            `INSERT INTO xp_ledger (tenant_id, user_id, delta, reason)
             VALUES ($1, $2, $3, 'test.seed')`,
            tenantID, userID, delta,
        ); err != nil {
            t.Fatalf("insert xp: %v", err)
        }
    }
}

type xpResponse struct {
    Level          int   `json:"level"`
    TotalXP        int64 `json:"total_xp"`
    XPIntoLevel    int64 `json:"xp_into_level"`
    XPForNextLevel int64 `json:"xp_for_next_level"`
    ProgressPct    int   `json:"progress_pct"`
}

func TestGamificationCrossTenantIsolation(t *testing.T) {
    pool := newTestPool(t)
    admin := newAdminPool(t)
    seedVolta(t, pool)

    // Wipe both users (via admin role, bypassing RLS + append-only), then seed.
    clearGamificationState(t, admin, harbourlineAdaID)
    clearGamificationState(t, admin, voltaUserID)
    t.Cleanup(func() {
        clearGamificationState(t, admin, harbourlineAdaID)
        clearGamificationState(t, admin, voltaUserID)
    })

    seedXPForUser(t, admin, harbourlineTenantID, harbourlineAdaID, 1, 10)
    seedXPForUser(t, admin, voltaTenantID, voltaUserID, 1, 100)

    ts := httptest.NewServer(server.New(pool, server.Options{DevAuth: true}))
    defer ts.Close()

    harbourlineHeaders := map[string]string{
        "X-Dev-Tenant":     "harbourline",
        "X-Dev-User-Email": "ada.okafor@harbourline.test",
    }
    voltaHeaders := map[string]string{
        "X-Dev-Tenant":     "volta",
        "X-Dev-User-Email": voltaEmail,
    }

    // ---------------- /me/xp ----------------

    t.Run("harbourline user sees only harbourline xp", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/xp", nil)
        for k, v := range harbourlineHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            t.Fatalf("status = %d", resp.StatusCode)
        }
        var x xpResponse
        if err := json.NewDecoder(resp.Body).Decode(&x); err != nil {
            t.Fatalf("decode: %v", err)
        }
        if x.TotalXP != 10 {
            t.Errorf("harbourline xp = %d, want 10", x.TotalXP)
        }
    })

    t.Run("volta user sees only volta xp", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/xp", nil)
        for k, v := range voltaHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        var x xpResponse
        if err := json.NewDecoder(resp.Body).Decode(&x); err != nil {
            t.Fatalf("decode: %v", err)
        }
        if x.TotalXP != 100 {
            t.Errorf("volta xp = %d, want 100", x.TotalXP)
        }
    })

    // ---------------- /me/xp/ledger ----------------

    t.Run("harbourline ledger has only harbourline entries", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/xp/ledger", nil)
        for k, v := range harbourlineHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        var body struct {
            Items []struct {
                Delta int32 `json:"delta"`
            } `json:"items"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        var sum int32
        for _, it := range body.Items {
            sum += it.Delta
        }
        if sum != 10 {
            t.Errorf("harbourline ledger sum = %d, want 10", sum)
        }
    })

    t.Run("volta ledger has only volta entries", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/xp/ledger", nil)
        for k, v := range voltaHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        var body struct {
            Items []struct {
                Delta int32 `json:"delta"`
            } `json:"items"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        var sum int32
        for _, it := range body.Items {
            sum += it.Delta
        }
        if sum != 100 {
            t.Errorf("volta ledger sum = %d, want 100", sum)
        }
    })

    // ---------------- /me/badges ----------------

    t.Run("harbourline badges list has no volta-keyed badges", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/badges", nil)
        for k, v := range harbourlineHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            t.Fatalf("status = %d", resp.StatusCode)
        }
        var body struct {
            Badges []struct {
                Key    string `json:"key"`
                Earned bool   `json:"earned"`
            } `json:"badges"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        for _, b := range body.Badges {
            if b.Earned {
                t.Errorf("harbourline user has badge %q earned unexpectedly", b.Key)
            }
        }
    })

    t.Run("volta badges list is empty (no volta badges seeded)", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/badges", nil)
        for k, v := range voltaHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            t.Fatalf("status = %d", resp.StatusCode)
        }
        var body struct {
            Badges []struct {
                Key string `json:"key"`
            } `json:"badges"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        if len(body.Badges) != 0 {
            t.Errorf("volta badges list = %d, want 0 (harbourline badges must not leak)", len(body.Badges))
        }
    })

    // ---------------- /team/leaderboard RBAC ----------------

    t.Run("volta user without team.view gets 403 on leaderboard", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/team/leaderboard", nil)
        for k, v := range voltaHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 403 {
            t.Errorf("volta leaderboard status = %d, want 403", resp.StatusCode)
        }
    })

    // ---------------- /me/quests lazy generation is tenant-scoped ----------------

    t.Run("volta user quests only reference volta user's rows", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/quests", nil)
        for k, v := range voltaHeaders {
            req.Header.Set(k, v)
        }
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            t.Fatalf("status = %d", resp.StatusCode)
        }
        var body struct {
            Active []struct {
                ID string `json:"id"`
            } `json:"active"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }

        ctx := context.Background()
        tx, err := pool.Begin(ctx)
        if err != nil {
            t.Fatalf("begin verify: %v", err)
        }
        defer tx.Rollback(ctx)
        _, _ = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID)

        for _, q := range body.Active {
            id, err := uuid.Parse(q.ID)
            if err != nil {
                t.Errorf("bad uuid in response: %q", q.ID)
                continue
            }
            var userID uuid.UUID
            if err := tx.QueryRow(ctx,
                `SELECT user_id FROM user_quests WHERE id = $1`, id,
            ).Scan(&userID); err != nil {
                t.Errorf("quest %s not found under volta: %v", id, err)
                continue
            }
            if userID.String() != voltaUserID {
                t.Errorf("cross-tenant leak: quest %s owned by %s, not volta user", id, userID)
            }
        }
        _ = tx.Commit(ctx)
    })

    // ---------------- unauthenticated guard ----------------

    t.Run("no headers", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me/xp", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 401 {
            t.Errorf("status = %d, want 401", resp.StatusCode)
        }
    })
}