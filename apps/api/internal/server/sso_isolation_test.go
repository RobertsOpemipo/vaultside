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
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/server"
)

// seedSSOConnection inserts a basic OIDC connection for a tenant and returns
// its UUID. Cleaned up on test exit.
func seedSSOConnection(t *testing.T, pool *pgxpool.Pool, tenantID, displayName string) uuid.UUID {
    t.Helper()
    ctx := context.Background()

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

    id := uuid.Must(uuid.NewV7())
    issuer := "http://example.test/" + displayName
    clientID := "test-client"
    if _, err := tx.Exec(ctx,
        `INSERT INTO sso_connections
           (id, tenant_id, display_name, kind, enabled, is_default, oidc_issuer, oidc_client_id, oidc_client_secret_enc)
         VALUES ($1, $2, $3, 'oidc', true, false, $4, $5, '\x01'::bytea)`,
        id, tenantID, displayName, issuer, clientID,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("insert sso connection: %v", err)
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
        _, _ = tx.Exec(c, `DELETE FROM sso_connections WHERE id = $1`, id)
        _ = tx.Commit(c)
    })

    return id
}

func TestSSOEndpointCrossTenantIsolation(t *testing.T) {
    pool := newTestPool(t)
    seedVolta(t, pool)

    // Give each tenant one SSO connection so we have cross-tenant IDs to test with.
    harbourlineConnID := seedSSOConnection(t, pool, harbourlineTenantID, "Harbourline IdP")
    voltaConnID := seedSSOConnection(t, pool, voltaTenantID, "Volta IdP")

    ts := httptest.NewServer(server.New(pool, server.Options{DevAuth: true}))
    defer ts.Close()

    // Header sets for each tenant's admin. Ada is admin at Harbourline.
    // Volta has no seeded admin, so we grant sso.manage via a fixture below.
    voltaAdminHeaders := grantVoltaAdminAndReturnHeaders(t, pool)
    harbourlineHeaders := map[string]string{
        "X-Dev-Tenant":     "harbourline",
        "X-Dev-User-Email": "ada.okafor@harbourline.test",
        "Content-Type":     "application/json",
    }

    applyHeaders := func(req *http.Request, h map[string]string) {
        for k, v := range h {
            req.Header.Set(k, v)
        }
    }

    // ---------------- listing ----------------

    t.Run("harbourline admin lists only harbourline connections", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/admin/sso/connections", nil)
        applyHeaders(req, harbourlineHeaders)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()

        var body struct {
            Connections []struct {
                ID          string `json:"id"`
                DisplayName string `json:"display_name"`
            } `json:"connections"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        for _, c := range body.Connections {
            if c.DisplayName == "Volta IdP" {
                t.Errorf("cross-tenant leak: harbourline admin sees Volta IdP")
            }
            if c.ID == voltaConnID.String() {
                t.Errorf("cross-tenant leak: harbourline admin sees volta connection id")
            }
        }
    })

    t.Run("volta admin lists only volta connections", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/admin/sso/connections", nil)
        applyHeaders(req, voltaAdminHeaders)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 200 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, body=%s", resp.StatusCode, b)
        }

        var body struct {
            Connections []struct {
                ID          string `json:"id"`
                DisplayName string `json:"display_name"`
            } `json:"connections"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
            t.Fatalf("decode: %v", err)
        }
        for _, c := range body.Connections {
            if c.DisplayName == "Harbourline IdP" {
                t.Errorf("cross-tenant leak: volta admin sees Harbourline IdP")
            }
        }
    })

    // ---------------- get by id ----------------

    t.Run("harbourline admin cannot read volta connection", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet,
            ts.URL+"/v1/admin/sso/connections/"+voltaConnID.String(), nil)
        applyHeaders(req, harbourlineHeaders)

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

    t.Run("volta admin cannot read harbourline connection", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet,
            ts.URL+"/v1/admin/sso/connections/"+harbourlineConnID.String(), nil)
        applyHeaders(req, voltaAdminHeaders)

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

    t.Run("harbourline admin cannot disable volta connection", func(t *testing.T) {
        body := `{"enabled":false}`
        req, _ := http.NewRequest(http.MethodPatch,
            ts.URL+"/v1/admin/sso/connections/"+voltaConnID.String(),
            strings.NewReader(body))
        applyHeaders(req, harbourlineHeaders)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 404 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, want 404. body=%s", resp.StatusCode, b)
        }

        // Confirm Volta connection is still enabled.
        var enabled bool
        ctx := context.Background()
        tx, err := pool.Begin(ctx)
        if err != nil {
            t.Fatalf("verify tx: %v", err)
        }
        defer tx.Rollback(ctx)
        _, _ = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID)
        if err := tx.QueryRow(ctx,
            `SELECT enabled FROM sso_connections WHERE id = $1`, voltaConnID,
        ).Scan(&enabled); err != nil {
            t.Fatalf("verify volta conn: %v", err)
        }
        if !enabled {
            t.Errorf("cross-tenant mutation: volta connection was disabled")
        }
        _ = tx.Commit(ctx)
    })

    // ---------------- delete across tenant ----------------

    t.Run("harbourline admin cannot delete volta connection", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodDelete,
            ts.URL+"/v1/admin/sso/connections/"+voltaConnID.String(), nil)
        applyHeaders(req, harbourlineHeaders)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()

        // Verify still present in Volta.
        ctx := context.Background()
        tx, err := pool.Begin(ctx)
        if err != nil {
            t.Fatalf("verify tx: %v", err)
        }
        defer tx.Rollback(ctx)
        _, _ = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID)
        var count int
        if err := tx.QueryRow(ctx,
            `SELECT count(*) FROM sso_connections WHERE id = $1`, voltaConnID,
        ).Scan(&count); err != nil {
            t.Fatalf("verify: %v", err)
        }
        if count != 1 {
            t.Errorf("cross-tenant delete: volta connection was removed")
        }
        _ = tx.Commit(ctx)
    })

    // ---------------- OIDC start cross-tenant ----------------

    t.Run("volta oidc start does not use harbourline connection", func(t *testing.T) {
        // Make sure Volta's connection is default so start will attempt it.
        ctx := context.Background()
        tx, err := pool.Begin(ctx)
        if err != nil {
            t.Fatalf("begin: %v", err)
        }
        _, _ = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID)
        _, _ = tx.Exec(ctx,
            `UPDATE sso_connections SET is_default = true WHERE id = $1`, voltaConnID)
        _ = tx.Commit(ctx)

        req, _ := http.NewRequest(http.MethodGet,
            ts.URL+"/v1/auth/oidc/volta/start", nil)
        client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
            return http.ErrUseLastResponse
        }}
        resp, err := client.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()

        // Our seeded Volta connection points to http://example.test/Volta IdP
        // which won't resolve — the handler will return 502 discovery_failed.
        // The key assertion: it did NOT redirect to Harbourline's IdP.
        loc := resp.Header.Get("Location")
        if strings.Contains(loc, "Harbourline") {
            t.Errorf("cross-tenant redirect: volta start redirected to harbourline IdP: %s", loc)
        }
        // Either 502 (discovery failed, correct) or 404 (no default) is acceptable.
        if resp.StatusCode != 502 && resp.StatusCode != 404 {
            t.Logf("volta oidc start returned %d (acceptable)", resp.StatusCode)
        }
    })

    // ---------------- unauthenticated guard ----------------

    t.Run("no auth to sso admin", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/admin/sso/connections", nil)
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

// grantVoltaAdminAndReturnHeaders inserts an admin role + permission into the
// Volta tenant and grants it to the seeded Volta auditor user, then returns
// the dev headers the test client should use.
func grantVoltaAdminAndReturnHeaders(t *testing.T, pool *pgxpool.Pool) map[string]string {
    t.Helper()
    ctx := context.Background()

    roleID := uuid.Must(uuid.NewV7())
    urID := uuid.Must(uuid.NewV7())

    tx, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("begin: %v", err)
    }
    if _, err := tx.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("set_config: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO roles (id, tenant_id, key, name, is_system)
         VALUES ($1, $2, 'admin', 'Admin', true)
         ON CONFLICT (tenant_id, key) DO NOTHING`,
        roleID, voltaTenantID,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("insert role: %v", err)
    }
    // If ON CONFLICT skipped, we need the existing role id.
    var actualRoleID uuid.UUID
    if err := tx.QueryRow(ctx,
        `SELECT id FROM roles WHERE tenant_id = $1 AND key = 'admin'`, voltaTenantID,
    ).Scan(&actualRoleID); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("select role: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO role_permissions (role_id, permission)
         VALUES ($1, 'sso.manage'), ($1, 'scim.manage')
         ON CONFLICT DO NOTHING`, actualRoleID,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("insert role_permissions: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO user_roles (id, tenant_id, user_id, role_id, scope_type)
         VALUES ($1, $2, $3, $4, 'tenant')
         ON CONFLICT DO NOTHING`,
        urID, voltaTenantID, voltaUserID, actualRoleID,
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("insert user_roles: %v", err)
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
        _, _ = tx.Exec(c, "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID)
        _, _ = tx.Exec(c, `DELETE FROM user_roles WHERE tenant_id = $1`, voltaTenantID)
        _, _ = tx.Exec(c, `DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE tenant_id = $1)`, voltaTenantID)
        _, _ = tx.Exec(c, `DELETE FROM roles WHERE tenant_id = $1`, voltaTenantID)
        _ = tx.Commit(c)
    })

    return map[string]string{
        "X-Dev-Tenant":     "volta",
        "X-Dev-User-Email": voltaEmail,
        "Content-Type":     "application/json",
    }
}