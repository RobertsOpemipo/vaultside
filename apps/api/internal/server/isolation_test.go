package server_test

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "testing"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/platform/config"
    "github.com/vaultside/vaultside/apps/api/internal/platform/db"
    "github.com/vaultside/vaultside/apps/api/internal/server"
)

const (
    // Volta tenant and its original learner user
    voltaTenantID  = "aaaaaaaa-aaaa-7aaa-8aaa-aaaaaaaaaaaa"
    voltaUserID    = "bbbbbbbb-bbbb-7bbb-8bbb-bbbbbbbbbbbb"
    voltaRoleID    = "cccccccc-cccc-7ccc-8ccc-cccccccccccc"
    voltaEmail     = "audit@volta.test"

    // Volta manager + report, added for the docket test
    voltaMgrRoleID = "dddddddd-aaaa-7aaa-8aaa-000000000001"
    voltaMgrUserID = "eeeeeeee-aaaa-7aaa-8aaa-000000000001"
    voltaMgrEmail  = "manager@volta.test"
    voltaRepUserID = "eeeeeeee-aaaa-7aaa-8aaa-000000000002"
    voltaRepEmail  = "report@volta.test"

    // Harbourline IDs
    harbourlineTenantID = "11111111-1111-7111-8111-111111111111"
    harbourlineEmail    = "ada.okafor@harbourline.test"
    tundeEmail          = "tunde.adeyemi@harbourline.test"

    bodeID        = "44444444-0000-7000-8000-000000000008"
    bodeEmail     = "bode.akinyemi@harbourline.test"
    first90ID     = "66666666-0000-7000-8000-000000000001"
    first90VerID  = "77777777-0000-7000-8000-000000000001"
    first90Les1ID = "88888888-0000-7000-8000-000000000001"
    q1ID          = "aaaaaaaa-0000-7000-8000-000000000001"
    q1Opt3ID      = "bbbbbbbb-0000-7000-8000-000000000003"
)

func TestMain(m *testing.M) {
    config.LoadDotEnv()
    os.Exit(m.Run())
}

func newTestPool(t *testing.T) *pgxpool.Pool {
    t.Helper()
    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("config: %v", err)
    }
    p, err := db.New(context.Background(), cfg.DatabaseURL)
    if err != nil {
        t.Fatalf("db: %v", err)
    }
    t.Cleanup(func() { p.Close() })
    return p.Pool
}

// -----------------------------------------------------------------------------
// Volta seed: tenant, learner role, learner user, manager role, manager user,
// and one report whose manager_id points at the manager.
// -----------------------------------------------------------------------------

func seedVolta(t *testing.T, pool *pgxpool.Pool) {
    t.Helper()
    ctx := context.Background()

    tx, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("begin: %v", err)
    }
    defer tx.Rollback(ctx)

    if _, err := tx.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID,
    ); err != nil {
        t.Fatalf("set_config: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO tenants (id, slug, name, theme)
         VALUES ($1, 'volta', 'Volta Capital', 'expedition-atlas')
         ON CONFLICT (id) DO NOTHING`, voltaTenantID,
    ); err != nil {
        t.Fatalf("insert tenant: %v", err)
    }
    // learner role
    if _, err := tx.Exec(ctx,
        `INSERT INTO roles (id, tenant_id, key, name, is_system)
         VALUES ($1, $2, 'learner', 'Learner', true)
         ON CONFLICT (id) DO NOTHING`, voltaRoleID, voltaTenantID,
    ); err != nil {
        t.Fatalf("insert learner role: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO role_permissions (role_id, permission)
         VALUES ($1, 'course.view'), ($1, 'lesson.progress')
         ON CONFLICT DO NOTHING`, voltaRoleID,
    ); err != nil {
        t.Fatalf("insert learner permissions: %v", err)
    }
    // manager role
    if _, err := tx.Exec(ctx,
        `INSERT INTO roles (id, tenant_id, key, name, is_system)
         VALUES ($1, $2, 'manager', 'Manager', true)
         ON CONFLICT (id) DO NOTHING`, voltaMgrRoleID, voltaTenantID,
    ); err != nil {
        t.Fatalf("insert manager role: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO role_permissions (role_id, permission)
         VALUES ($1, 'team.view'), ($1, 'team.nudge'), ($1, 'onboarding.approve')
         ON CONFLICT DO NOTHING`, voltaMgrRoleID,
    ); err != nil {
        t.Fatalf("insert manager permissions: %v", err)
    }
    // users
    if _, err := tx.Exec(ctx,
        `INSERT INTO users (id, tenant_id, email, full_name, status)
         VALUES ($1, $2, $3, 'Volta Auditor', 'active')
         ON CONFLICT (id) DO NOTHING`, voltaUserID, voltaTenantID, voltaEmail,
    ); err != nil {
        t.Fatalf("insert auditor: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO users (id, tenant_id, email, full_name, status)
         VALUES ($1, $2, $3, 'Volta Manager', 'active')
         ON CONFLICT (id) DO NOTHING`, voltaMgrUserID, voltaTenantID, voltaMgrEmail,
    ); err != nil {
        t.Fatalf("insert manager user: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO users (id, tenant_id, email, full_name, status, manager_id)
         VALUES ($1, $2, $3, 'Volta Report', 'active', $4)
         ON CONFLICT (id) DO NOTHING`, voltaRepUserID, voltaTenantID, voltaRepEmail, voltaMgrUserID,
    ); err != nil {
        t.Fatalf("insert report user: %v", err)
    }
    // role assignments: auditor -> learner, manager -> manager
    if _, err := tx.Exec(ctx,
        `INSERT INTO user_roles (id, tenant_id, user_id, role_id, scope_type)
         VALUES (gen_random_uuid(), $1, $2, $3, 'tenant')
         ON CONFLICT DO NOTHING`, voltaTenantID, voltaUserID, voltaRoleID,
    ); err != nil {
        t.Fatalf("insert auditor role: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `INSERT INTO user_roles (id, tenant_id, user_id, role_id, scope_type)
         VALUES (gen_random_uuid(), $1, $2, $3, 'tenant')
         ON CONFLICT DO NOTHING`, voltaTenantID, voltaMgrUserID, voltaMgrRoleID,
    ); err != nil {
        t.Fatalf("insert manager role: %v", err)
    }

    if err := tx.Commit(ctx); err != nil {
        t.Fatalf("commit: %v", err)
    }

    t.Cleanup(func() {
        ctx := context.Background()
        tx, err := pool.Begin(ctx)
        if err != nil {
            return
        }
        defer tx.Rollback(ctx)
        if _, err := tx.Exec(ctx,
            "SELECT set_config('app.tenant_id', $1, true)", voltaTenantID,
        ); err != nil {
            return
        }
        // Detach manager_id before deleting users (FK constraint).
        _, _ = tx.Exec(ctx, `UPDATE users SET manager_id = NULL WHERE tenant_id = $1`, voltaTenantID)
        _, _ = tx.Exec(ctx, `DELETE FROM user_roles WHERE tenant_id = $1`, voltaTenantID)
        _, _ = tx.Exec(ctx, `DELETE FROM users      WHERE tenant_id = $1`, voltaTenantID)
        _, _ = tx.Exec(ctx, `DELETE FROM roles      WHERE tenant_id = $1`, voltaTenantID)
        _, _ = tx.Exec(ctx, `DELETE FROM tenants    WHERE id        = $1`, voltaTenantID)
        _ = tx.Commit(ctx)
    })
}

// -----------------------------------------------------------------------------
// /v1/me isolation
// -----------------------------------------------------------------------------

type meResponse struct {
    UserID      string   `json:"user_id"`
    Email       string   `json:"email"`
    FullName    string   `json:"full_name"`
    TenantID    string   `json:"tenant_id"`
    TenantSlug  string   `json:"tenant_slug"`
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
}

func TestMeEndpointCrossTenantIsolation(t *testing.T) {
    pool := newTestPool(t)
    seedVolta(t, pool)

    ts := httptest.NewServer(server.New(pool, server.Options{DevAuth: true}))
    defer ts.Close()

    cases := []struct {
        name       string
        tenant     string
        email      string
        wantStatus int
        wantEmail  string
        wantTenant string
    }{
        {"harbourline user with harbourline headers", "harbourline", harbourlineEmail, 200, harbourlineEmail, harbourlineTenantID},
        {"volta user with volta headers", "volta", voltaEmail, 200, voltaEmail, voltaTenantID},
        {"harbourline email under volta tenant", "volta", harbourlineEmail, 401, "", ""},
        {"volta email under harbourline tenant", "harbourline", voltaEmail, 401, "", ""},
        {"unknown tenant", "does-not-exist", harbourlineEmail, 401, "", ""},
        {"no headers", "", "", 401, "", ""},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/me", nil)
            if tc.tenant != "" {
                req.Header.Set("X-Dev-Tenant", tc.tenant)
            }
            if tc.email != "" {
                req.Header.Set("X-Dev-User-Email", tc.email)
            }
            resp, err := http.DefaultClient.Do(req)
            if err != nil {
                t.Fatalf("request: %v", err)
            }
            defer resp.Body.Close()
            if resp.StatusCode != tc.wantStatus {
                b, _ := io.ReadAll(resp.Body)
                t.Fatalf("status = %d, want %d. body: %s", resp.StatusCode, tc.wantStatus, b)
            }
            if tc.wantStatus != 200 {
                return
            }
            var me meResponse
            if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
                t.Fatalf("decode: %v", err)
            }
            if me.Email != tc.wantEmail {
                t.Errorf("email = %q, want %q", me.Email, tc.wantEmail)
            }
            if me.TenantID != tc.wantTenant {
                t.Errorf("tenant_id = %q, want %q", me.TenantID, tc.wantTenant)
            }
        })
    }
}

// -----------------------------------------------------------------------------
// RLS direct-query check
// -----------------------------------------------------------------------------

func TestRLSDirectQueryAsAppRole(t *testing.T) {
    pool := newTestPool(t)
    ctx := context.Background()

    conn, err := pool.Acquire(ctx)
    if err != nil {
        t.Fatalf("acquire: %v", err)
    }
    defer conn.Release()

    var count int
    if err := conn.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
        t.Fatalf("count: %v", err)
    }
    if count != 0 {
        t.Fatalf("RLS not enforced: users count = %d with no app.tenant_id", count)
    }

    if _, err := conn.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, false)", harbourlineTenantID,
    ); err != nil {
        t.Fatalf("set_config: %v", err)
    }
    if err := conn.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
        t.Fatalf("count with tenant: %v", err)
    }
    if count == 0 {
        t.Fatalf("RLS too strict: users count = 0 with app.tenant_id set")
    }

    _, _ = conn.Exec(ctx, "RESET app.tenant_id")
}

// -----------------------------------------------------------------------------
// Progress + attempt endpoint isolation
// -----------------------------------------------------------------------------

func seedHarbourlineEnrollment(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
    t.Helper()
    ctx := context.Background()
    hID := uuid.MustParse(harbourlineTenantID)
    bode := uuid.MustParse(bodeID)
    course := uuid.MustParse(first90ID)
    version := uuid.MustParse(first90VerID)

    // Delete any prior enrollment for (bode, version), plus its children.
    tx, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("begin cleanup: %v", err)
    }
    if _, err := tx.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, true)", hID.String(),
    ); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("cleanup set_config: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `DELETE FROM lesson_progress
         WHERE enrollment_id IN (
           SELECT id FROM enrollments WHERE user_id = $1 AND course_version_id = $2
         )`, bode, version); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("cleanup lesson_progress: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `DELETE FROM attempts
         WHERE enrollment_id IN (
           SELECT id FROM enrollments WHERE user_id = $1 AND course_version_id = $2
         )`, bode, version); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("cleanup attempts: %v", err)
    }
    if _, err := tx.Exec(ctx,
        `DELETE FROM enrollments WHERE user_id = $1 AND course_version_id = $2`,
        bode, version); err != nil {
        _ = tx.Rollback(ctx)
        t.Fatalf("cleanup enrollments: %v", err)
    }
    if err := tx.Commit(ctx); err != nil {
        t.Fatalf("cleanup commit: %v", err)
    }

    id := uuid.Must(uuid.NewV7())
    tx2, err := pool.Begin(ctx)
    if err != nil {
        t.Fatalf("begin insert: %v", err)
    }
    if _, err := tx2.Exec(ctx,
        "SELECT set_config('app.tenant_id', $1, true)", hID.String(),
    ); err != nil {
        _ = tx2.Rollback(ctx)
        t.Fatalf("insert set_config: %v", err)
    }
    if _, err := tx2.Exec(ctx,
        `INSERT INTO enrollments (id, tenant_id, user_id, course_id, course_version_id, source)
         VALUES ($1, $2, $3, $4, $5, 'manual')`,
        id, hID, bode, course, version,
    ); err != nil {
        _ = tx2.Rollback(ctx)
        t.Fatalf("insert enrollment: %v", err)
    }
    if err := tx2.Commit(ctx); err != nil {
        t.Fatalf("insert commit: %v", err)
    }

    t.Cleanup(func() {
        c := context.Background()
        tx, err := pool.Begin(c)
        if err != nil {
            return
        }
        defer tx.Rollback(c)
        if _, err := tx.Exec(c,
            "SELECT set_config('app.tenant_id', $1, true)", hID.String(),
        ); err != nil {
            return
        }
        _, _ = tx.Exec(c, `DELETE FROM lesson_progress WHERE enrollment_id = $1`, id)
        _, _ = tx.Exec(c, `DELETE FROM attempts        WHERE enrollment_id = $1`, id)
        _, _ = tx.Exec(c, `DELETE FROM enrollments     WHERE id = $1`, id)
        _ = tx.Commit(c)
    })

    return id
}

func TestCrossTenantEnrollmentIsolation(t *testing.T) {
    pool := newTestPool(t)
    seedVolta(t, pool)
    bodeEnrollmentID := seedHarbourlineEnrollment(t, pool)

    ts := httptest.NewServer(server.New(pool, server.Options{DevAuth: true}))
    defer ts.Close()

    cases := []struct {
        name       string
        tenant     string
        email      string
        method     string
        path       string
        body       string
        wantStatus int
    }{
        {
            name:       "volta user records progress on harbourline enrollment",
            tenant:     "volta",
            email:      voltaEmail,
            method:     http.MethodPost,
            path:       "/v1/enrollments/" + bodeEnrollmentID.String() + "/lessons/" + first90Les1ID + "/progress",
            body:       `{"status":"completed","seconds_viewed":60}`,
            wantStatus: 404,
        },
        {
            name:       "volta user starts attempt on harbourline enrollment",
            tenant:     "volta",
            email:      voltaEmail,
            method:     http.MethodPost,
            path:       "/v1/enrollments/" + bodeEnrollmentID.String() + "/attempts",
            wantStatus: 404,
        },
        {
            name:       "ada records progress on bodes enrollment",
            tenant:     "harbourline",
            email:      harbourlineEmail,
            method:     http.MethodPost,
            path:       "/v1/enrollments/" + bodeEnrollmentID.String() + "/lessons/" + first90Les1ID + "/progress",
            body:       `{"status":"completed","seconds_viewed":60}`,
            wantStatus: 403,
        },
        {
            name:       "ada starts attempt on bodes enrollment",
            tenant:     "harbourline",
            email:      harbourlineEmail,
            method:     http.MethodPost,
            path:       "/v1/enrollments/" + bodeEnrollmentID.String() + "/attempts",
            wantStatus: 403,
        },
        {
            name:       "random enrollment id is not found",
            tenant:     "harbourline",
            email:      harbourlineEmail,
            method:     http.MethodPost,
            path:       "/v1/enrollments/00000000-0000-7000-8000-000000000000/lessons/" + first90Les1ID + "/progress",
            body:       `{"status":"completed"}`,
            wantStatus: 404,
        },
        {
            name:       "random attempt submit is not found",
            tenant:     "harbourline",
            email:      harbourlineEmail,
            method:     http.MethodPost,
            path:       "/v1/attempts/00000000-0000-7000-8000-000000000000/submit",
            wantStatus: 404,
        },
        {
            name:       "random attempt answer save is not found",
            tenant:     "harbourline",
            email:      harbourlineEmail,
            method:     http.MethodPost,
            path:       "/v1/attempts/00000000-0000-7000-8000-000000000000/answers",
            body:       `{"question_id":"` + q1ID + `","option_id":"` + q1Opt3ID + `"}`,
            wantStatus: 404,
        },
        {
            name:       "no headers on progress endpoint",
            method:     http.MethodPost,
            path:       "/v1/enrollments/" + bodeEnrollmentID.String() + "/lessons/" + first90Les1ID + "/progress",
            body:       `{"status":"completed"}`,
            wantStatus: 401,
        },
        {
            name:       "wrong tenant slug on progress endpoint",
            tenant:     "does-not-exist",
            email:      harbourlineEmail,
            method:     http.MethodPost,
            path:       "/v1/enrollments/" + bodeEnrollmentID.String() + "/lessons/" + first90Les1ID + "/progress",
            body:       `{"status":"completed"}`,
            wantStatus: 401,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            var body io.Reader
            if tc.body != "" {
                body = strings.NewReader(tc.body)
            }
            req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
            if err != nil {
                t.Fatalf("new request: %v", err)
            }
            if tc.tenant != "" {
                req.Header.Set("X-Dev-Tenant", tc.tenant)
            }
            if tc.email != "" {
                req.Header.Set("X-Dev-User-Email", tc.email)
            }
            if tc.body != "" {
                req.Header.Set("Content-Type", "application/json")
            }

            resp, err := http.DefaultClient.Do(req)
            if err != nil {
                t.Fatalf("do: %v", err)
            }
            defer resp.Body.Close()

            if resp.StatusCode != tc.wantStatus {
                b, _ := io.ReadAll(resp.Body)
                t.Fatalf("status = %d, want %d. body: %s", resp.StatusCode, tc.wantStatus, b)
            }
        })
    }
}

// -----------------------------------------------------------------------------
// Docket isolation: Volta manager sees only Volta reports
// -----------------------------------------------------------------------------

type docketResponse struct {
    ManagerEmail string `json:"manager_email"`
    Reports      []struct {
        Email    string `json:"email"`
        FullName string `json:"full_name"`
    } `json:"reports"`
}

func TestTeamDocketIsolation(t *testing.T) {
    pool := newTestPool(t)
    seedVolta(t, pool)

    ts := httptest.NewServer(server.New(pool, server.Options{DevAuth: true}))
    defer ts.Close()

    // 1. Volta manager's docket sees exactly one report, the Volta report.
    t.Run("volta manager sees only volta reports", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/team/docket", nil)
        req.Header.Set("X-Dev-Tenant", "volta")
        req.Header.Set("X-Dev-User-Email", voltaMgrEmail)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != 200 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, want 200. body: %s", resp.StatusCode, b)
        }

        var d docketResponse
        if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
            t.Fatalf("decode: %v", err)
        }

        if d.ManagerEmail != voltaMgrEmail {
            t.Errorf("manager_email = %q, want %q", d.ManagerEmail, voltaMgrEmail)
        }
        if len(d.Reports) != 1 {
            t.Fatalf("reports count = %d, want 1; got %+v", len(d.Reports), d.Reports)
        }
        if d.Reports[0].Email != voltaRepEmail {
            t.Errorf("report email = %q, want %q", d.Reports[0].Email, voltaRepEmail)
        }
        // No harbourline user may leak into a Volta response.
        for _, r := range d.Reports {
            if strings.Contains(strings.ToLower(r.Email), "harbourline") {
                t.Errorf("cross-tenant leak: volta docket returned %q", r.Email)
            }
        }
    })

    // 2. A Volta learner (no team.view) gets 403.
    t.Run("volta learner cannot access docket", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/team/docket", nil)
        req.Header.Set("X-Dev-Tenant", "volta")
        req.Header.Set("X-Dev-User-Email", voltaEmail)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != 403 {
            t.Errorf("status = %d, want 403", resp.StatusCode)
        }
    })

    // 3. Harbourline manager's docket contains only harbourline reports —
    // proving RLS hides Volta users from a Harbourline manager.
    t.Run("harbourline manager sees no volta reports", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/team/docket", nil)
        req.Header.Set("X-Dev-Tenant", "harbourline")
        req.Header.Set("X-Dev-User-Email", tundeEmail)

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            t.Fatalf("do: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != 200 {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("status = %d, want 200. body: %s", resp.StatusCode, b)
        }
        var d docketResponse
        if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
            t.Fatalf("decode: %v", err)
        }
        for _, r := range d.Reports {
            if strings.Contains(strings.ToLower(r.Email), "volta") {
                t.Errorf("cross-tenant leak: harbourline docket returned %q", r.Email)
            }
        }
    })

    // 4. No headers → 401.
    t.Run("no headers", func(t *testing.T) {
        req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/team/docket", nil)
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