package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5/middleware"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/platform/antivirus"
    "github.com/vaultside/vaultside/apps/api/internal/platform/config"
    "github.com/vaultside/vaultside/apps/api/internal/platform/crypto"
    "github.com/vaultside/vaultside/apps/api/internal/platform/db"
    "github.com/vaultside/vaultside/apps/api/internal/platform/storage"
    "github.com/vaultside/vaultside/apps/api/internal/server"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config: %v", err)
    }

    ssoKey, err := crypto.ParseKey(cfg.SSOSecretKeyB64)
    if err != nil {
        log.Fatalf("sso secret key: %v", err)
    }

    samlKeypair, err := auth.ParseSAMLKeypair(cfg.SAMLCertB64, cfg.SAMLKeyB64)
    if err != nil {
        log.Fatalf("saml keypair: %v", err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    pool, err := db.New(ctx, cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("db: %v", err)
    }
    defer pool.Close()
    log.Printf("db connected (env=%s)", cfg.Env)

    store, err := storage.New(ctx, storage.Config{
        Endpoint:  cfg.S3Endpoint,
        Region:    cfg.S3Region,
        Bucket:    cfg.S3Bucket,
        AccessKey: cfg.S3AccessKey,
        SecretKey: cfg.S3SecretKey,
    })
    if err != nil {
        log.Fatalf("storage: %v", err)
    }
    log.Printf("storage ready (bucket=%s endpoint=%s)", cfg.S3Bucket, cfg.S3Endpoint)

    handler := server.New(pool.Pool, server.Options{
        DevAuth:       cfg.DevAuth,
        Storage:       store,
        Scanner:       antivirus.NoopScanner{},
        SSOSecretKey:  ssoKey,
        SessionSecret: []byte(cfg.SessionSecret),
        SAMLKeypair:   samlKeypair,
        AppBaseURL:    cfg.AppBaseURL,
        APIBaseURL:    cfg.APIBaseURL,
        SecureCookies: cfg.Env == "production",
    })
    r := middleware.Logger(handler)

    addr := ":" + cfg.Port
    srv := &http.Server{
        Addr:              addr,
        Handler:           r,
        ReadHeaderTimeout: 5 * time.Second,
    }

    go func() {
        log.Printf("api listening on %s", addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %v", err)
        }
    }()

    <-ctx.Done()
    log.Println("api shutting down")
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = srv.Shutdown(shutdownCtx)
}