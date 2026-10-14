package server

import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/handlers"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
    "github.com/vaultside/vaultside/apps/api/internal/platform/antivirus"
    "github.com/vaultside/vaultside/apps/api/internal/platform/storage"
)

type Options struct {
    DevAuth       bool
    Storage       *storage.Client
    Scanner       antivirus.Scanner
    SSOSecretKey  []byte
    SessionSecret []byte
    SAMLKeypair   *auth.SAMLKeypair
    AppBaseURL    string
    APIBaseURL    string
    SecureCookies bool
    IdleTimeout   time.Duration
}

func New(pool *pgxpool.Pool, opts Options) http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)

    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{opts.AppBaseURL},
        AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization", "X-Dev-Tenant", "X-Dev-User-Email"},
        ExposedHeaders:   []string{"Content-Disposition"},
        AllowCredentials: true,
        MaxAge:           300,
    }))

    r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
    r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
        if err := pool.Ping(req.Context()); err != nil {
            w.WriteHeader(http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(200)
    })

    uploads := handlers.NewUploadHandler(opts.Storage, opts.Scanner)
    sso := handlers.NewSSOHandler(opts.SSOSecretKey)
    oidc := &handlers.OIDCHandler{
        Pool:          pool,
        SSOSecretKey:  opts.SSOSecretKey,
        SessionKey:    opts.SessionSecret,
        AppBaseURL:    opts.AppBaseURL,
        SecureCookies: opts.SecureCookies,
    }
    saml := &handlers.SAMLHandler{
        Pool:          pool,
        Keypair:       opts.SAMLKeypair,
        APIBaseURL:    opts.APIBaseURL,
        AppBaseURL:    opts.AppBaseURL,
        SessionKey:    opts.SessionSecret,
        SecureCookies: opts.SecureCookies,
    }

    r.Route("/v1", func(r chi.Router) {
        r.Get("/verify/{tenantSlug}/{code}", func(w http.ResponseWriter, req *http.Request) {
            handlers.PublicVerifyCertificate(w, req, pool)
        })

        r.Get("/auth/oidc/{slug}/start", oidc.Start)
        r.Get("/auth/oidc/callback", oidc.Callback)
        r.Get("/auth/saml/{slug}/metadata", saml.Metadata)
        r.Get("/auth/saml/{slug}/start", saml.Start)
        r.Post("/auth/saml/{slug}/acs", saml.ACS)

        r.Group(func(r chi.Router) {
            r.Use(auth.SessionMiddleware(pool, opts.IdleTimeout))
            r.Post("/auth/logout", handlers.Logout(pool))
        })

        r.Group(func(r chi.Router) {
            r.Use(auth.SessionMiddleware(pool, opts.IdleTimeout))
            if opts.DevAuth {
                r.Use(auth.DevAuthMiddleware(pool))
            }
            r.Use(httpx.WithTenantTx(pool))

            r.Get("/me", handlers.Me)
            r.Get("/me/permissions", handlers.MePermissions)
            r.Get("/me/enrollments", handlers.ListMyEnrollments)
            r.Get("/me/attestations", handlers.ListMyAttestations)
            r.Get("/me/certificates", handlers.ListMyCertificates)
            r.Get("/me/sessions", handlers.ListMySessions)
            r.Post("/me/sessions/revoke-all", handlers.RevokeOtherSessions)
            r.Delete("/me/sessions/{id}", handlers.RevokeSessionByID)
            r.Get("/courses", handlers.ListCourses)
            r.Get("/courses/{id}", handlers.GetCourseDetail)
            r.Get("/lessons/{id}", handlers.GetLessonDetail)
            r.Get("/regulations", handlers.ListRegulations)
            r.Get("/assets/{id}/url", uploads.AssetURL)

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("course.view"))
                r.Post("/courses/{id}/enroll", handlers.EnrollSelf)
                r.Post("/enrollments/{id}/lessons/{lessonId}/progress", handlers.RecordLessonProgress)
                r.Post("/enrollments/{id}/attempts", handlers.StartAttempt)
                r.Post("/attempts/{id}/answers", handlers.SaveAnswer)
                r.Post("/attempts/{id}/submit", handlers.SubmitAttempt)
                r.Post("/regulations/{id}/attest", handlers.SignAttestation)
            })

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("course.create"))
                r.Post("/uploads/presign", uploads.Presign)
                r.Post("/uploads/{id}/complete", uploads.Complete)
            })

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("rule.manage"))
                r.Get("/admin/rules", handlers.ListRules)
                r.Post("/admin/rules/run", handlers.RunRules)
                r.Post("/admin/rules/{id}/dry-run", handlers.DryRunRule)
            })

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("team.view"))
                r.Get("/team/docket", handlers.TeamDocket)
            })

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("report.export"))
                r.Get("/admin/compliance/heatmap", handlers.ComplianceHeatmap)
                r.Get("/admin/compliance/expiring", handlers.ExpiringCertificates)
                r.Post("/admin/reports/audit-pack", handlers.GenerateAuditPack)
            })

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("sso.manage"))
                r.Get("/admin/sso/connections", sso.List)
                r.Post("/admin/sso/connections", sso.Create)
                r.Get("/admin/sso/connections/{id}", sso.Get)
                r.Patch("/admin/sso/connections/{id}", sso.Update)
                r.Delete("/admin/sso/connections/{id}", sso.Delete)
                r.Post("/admin/sso/connections/{id}/test", sso.Test)
            })

            r.Group(func(r chi.Router) {
                r.Use(httpx.RequirePermission("scim.manage"))
                r.Get("/admin/scim/tokens", handlers.ListSCIMTokens)
                r.Post("/admin/scim/tokens", handlers.CreateSCIMToken)
                r.Delete("/admin/scim/tokens/{id}", handlers.RevokeSCIMToken)
            })
        })
    })

    r.Route("/scim/v2", func(r chi.Router) {
        r.Use(auth.SCIMAuthMiddleware(pool))
        r.Use(httpx.WithTenantTx(pool))

        r.Get("/ServiceProviderConfig", handlers.SCIMServiceProviderConfig)
        r.Get("/Schemas", handlers.SCIMListSchemas)
        r.Get("/Schemas/{id}", handlers.SCIMGetSchema)
        r.Get("/ResourceTypes", handlers.SCIMListResourceTypes)

        r.Get("/Users", handlers.SCIMListUsers)
        r.Post("/Users", handlers.SCIMCreateUser)
        r.Get("/Users/{id}", handlers.SCIMGetUser)
        r.Patch("/Users/{id}", handlers.SCIMPatchUser)
        r.Delete("/Users/{id}", handlers.SCIMDeleteUser)

        r.Get("/Groups", handlers.SCIMListGroups)
        r.Post("/Groups", handlers.SCIMCreateGroup)
        r.Get("/Groups/{id}", handlers.SCIMGetGroup)
        r.Patch("/Groups/{id}", handlers.SCIMPatchGroup)
        r.Delete("/Groups/{id}", handlers.SCIMDeleteGroup)
    })

    return r
}