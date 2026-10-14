package server_test

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/server"
)

// seedSCIMToken inserts a fresh SCIM token for the given tenant and returns
// the raw value the client would send as a Bearer token.
func seedSCIMToken(t *testing.T, pool *pgxpool.Pool, tenantID string) string {
    t.Helper()
    ctx := context.Background()

    raw, hash, prefix, err := auth.GenerateSCIMToken()
    if err != nil {
        t.Fatalf("generate scim token: %v", err)
    }

    tx, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("begin: %v", err)
    }
    if _, err := tx.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, true)", tenantID,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("set_config: %v", err)
    }

    tokenID := uuid.Must(uuid.NewV7())
    if _, err := tx.Exec(ctx,
        `INSERT INTO scim_tokens (id, tenant_id, name, token_hash, token_prefix)
         VALUES ($1, $2, 'test-token', $3, $4)`,
        tokenID, tenantID, hash, prefix,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("insert scim token: %v", err)
    }
    if err := tx.Commit(ctx); err != nil {
        t.Fatalf("commit: %v", err)
    }

    t.Cleanup(func() {
        c := context.Background()
        tx, err := pool.Begin(c)
        if err != nil {
            return
        }
        defer tx.Rollback(c)
        _, _ = tx.Exec(c, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
        _, _ = tx.Exec(c, `DELETE FROM scim_tokens WHERE id = $1`, tokenID)
        _ = tx.Commit(c)
    })

    return raw
}

// scimListResult is the ListResponse envelope.
type scimListResult struct {
    Schemas      []string          `json:"schemas"`
    TotalResults int               `json:"totalResults"`
    ItemsPerPage int               `json:"itemsPerPage"`
    Resources    []json.RawMessage `json:"Resources"`
}

func TestSCIMCrossTenantIsolation(t *testing.T) {
    pool := newTestPool(t)
    seedVolta(t, pool)

    harbourlineToken := seedSCIMToken(t, pool, harbourlineTenantID)
    voltaToken := seedSCIMToken(t, pool, voltaTenantID)

    ts := httptest.NewServer(server.New(pool, server.Options{DevAuth: true}))
    defer ts.Close()

    harbourlineAdaID := "44444444-0000-7000-8000-000000000001"
    voltaAuditorID := voltaUserID

    // ---------------- listing ----------------

    t.Run("volta token lists only volta users", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Users", nil)
        req.Header.Set("Authorization", "Bearer "+voltaToken)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, body=%s", resp.StatusCode, b)
        }

        var lr scimListResult
        if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
            t.Fatalf("decode: %v", err)
        }

        // Volta has exactly 3 users (auditor, manager, report)
        if lr.TotalResults != 3 {
            t.Errorf("volta totalResults = %d, want 3", lr.TotalResults)
        }
        // No harbourline email may appear in the response body.
        for _, raw := range lr.Resources {
            if strings.Contains(strings.ToLower(string(raw)), "harbourline") {
                t.Errorf("cross-tenant leak in volta scim listing: %s", raw)
            }
        }
    })

    t.Run("harbourline token lists only harbourline users", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Users?count=100", nil)
        req.Header.Set("Authorization", "Bearer "+harbourlineToken)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()

        var lr scimListResult
        if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
            t.Fatalf("decode: %v", err)
        }
        if lr.TotalResults == 0 {
            t.Errorf("harbourline totalResults = 0")
        }
        for _, raw := range lr.Resources {
            if strings.Contains(strings.ToLower(string(raw)), "volta.test") {
                t.Errorf("cross-tenant leak in harbourline scim listing: %s", raw)
            }
        }
    })

    // ---------------- get across tenant ----------------

    t.Run("volta token cannot get harbourline user", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Users/"+harbourlineAdaID, nil)
        req.Header.Set("Authorization", "Bearer "+voltaToken)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 404 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, want 404. body=%s", resp.StatusCode, b)
        }
    })

    t.Run("harbourline token cannot get volta user", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Users/"+voltaAuditorID, nil)
        req.Header.Set("Authorization", "Bearer "+harbourlineToken)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 404 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, want 404. body=%s", resp.StatusCode, b)
        }
    })

    // ---------------- patch across tenant ----------------

    t.Run("volta token cannot patch harbourline user", func(t *testing.T) {
        body := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}`
        req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/scim/v2/Users/"+harbourlineAdaID, strings.NewReader(body))
        req.Header.Set("Authorization", "Bearer "+voltaToken)
        req.Header.Set("Content-Type", "application/scim+json")

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 404 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, want 404. body=%s", resp.StatusCode, b)
        }

        // Confirm Ada is still active in the DB. Must run inside a
        // tenant-scoped transaction because the test pool connects as app_rw
        // (RLS enforced).
        ctx := context.Background()
        verifyTx, err := pool.Begin(ctx)
        if err != nil {
            t.Fatalf("verify tx: %v", err)
        }
        defer verifyTx.Rollback(ctx)
        if _, err := verifyTx.Exec(ctx,
            "SELECT set_config('app.tenant_id', $1, true)", harbourlineTenantID,
        ); err != nil {
            t.Fatalf("verify set_config: %v", err)
        }
        var status string
        if err := verifyTx.QueryRow(ctx,
            `SELECT status FROM users WHERE id = $1`, harbourlineAdaID,
        ).Scan(&status); err != nil {
            t.Fatalf("verify ada: %v", err)
        }
        if status != "active" {
            t.Errorf("cross-tenant mutation: ada status = %q, want active", status)
        }
        _ = verifyTx.Commit(ctx)
    })

    // ---------------- groups ----------------

    t.Run("volta token cannot get harbourline group", func(t *testing.T) {
        // Create a Harbourline team via SCIM so we have a real group ID.
        createBody := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"h-leak-test","externalId":"h-leak-001"}`
        createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/scim/v2/Groups", strings.NewReader(createBody))
        createReq.Header.Set("Authorization", "Bearer "+harbourlineToken)
        createReq.Header.Set("Content-Type", "application/scim+json")
        createResp, err := http.DefaultClient.Do(createReq)
        if err != nil {
            t.Fatalf("create group: %v", err)
        }
        defer createResp.Body.Close()
        if createResp.StatusCode != 200 && createResp.StatusCode != 201 {
            b, _ := io.ReadAll(createResp.Body)
            t.Fatalf("create group status = %d, body=%s", createResp.StatusCode, b)
        }
        var grp struct {
            ID string `json:"id"`
        }
        _ = json.NewDecoder(createResp.Body).Decode(&grp)

        // Clean up regardless of test outcome.
        t.Cleanup(func() {
            req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/scim/v2/Groups/"+grp.ID, nil)
            req.Header.Set("Authorization", "Bearer "+harbourlineToken)
            _, _ = http.DefaultClient.Do(req)
        })

        // Volta token tries to fetch it.
        getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Groups/"+grp.ID, nil)
        getReq.Header.Set("Authorization", "Bearer "+voltaToken)
        getResp, err := http.DefaultClient.Do(getReq)
        if err != nil {
            t.Fatalf("get group: %v", err)
        }
        defer getResp.Body.Close()
        if getResp.StatusCode != 404 {
            b, _ := io.ReadAll(getResp.Body)
            t.Fatalf("status = %d, want 404. body=%s", getResp.StatusCode, b)
        }
    })

    // ---------------- auth guard ----------------

    t.Run("no bearer token", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Users", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 401 {
            t.Errorf("status = %d, want 401", resp.StatusCode)
        }
    })

    t.Run("bogus bearer token", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/scim/v2/Users", nil)
        req.Header.Set("Authorization", "Bearer vscim_totally_bogus")
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

// Small guard to ensure the pgx import is used in case the test file gets
// trimmed by formatters.
var _ = pgx.ErrNoRows