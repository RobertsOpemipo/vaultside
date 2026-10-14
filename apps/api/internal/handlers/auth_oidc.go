package handlers

import (
    "context"
    "errors"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
    "github.com/vaultside/vaultside/apps/api/internal/platform/crypto"
)

type OIDCHandler struct {
    Pool          *pgxpool.Pool
    SSOSecretKey  []byte
    SessionKey    []byte
    AppBaseURL    string
    SecureCookies bool
    SessionTTL    time.Duration
}

func (h *OIDCHandler) redirectURL() string {
    return h.AppBaseURL + "/api-callback" // see note in Start handler
}

// ---------------- Start ----------------

// Start initiates the OIDC authorization code flow for a tenant slug.
// GET /v1/auth/oidc/{slug}/start?returnTo=/learn
func (h *OIDCHandler) Start(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    if slug == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_slug", "")
        return
    }
    returnTo := sanitizeReturnTo(r.URL.Query().Get("returnTo"))

    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    // Resolve tenant by slug (SECURITY DEFINER function).
    var tenantID uuid.UUID
    if err := h.Pool.QueryRow(ctx,
        `SELECT id FROM resolve_tenant_slug($1)`, slug,
    ).Scan(&tenantID); err != nil {
        httpx.Error(w, http.StatusNotFound, "not_found", "unknown tenant")
        return
    }

    // Open a tenant-scoped tx to fetch the default enabled SSO connection.
    tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        httpx.Error(w, http.StatusServiceUnavailable, "db_unavailable", "")
        return
    }
    defer tx.Rollback(context.Background())

    if _, err := tx.Exec(ctx,
        `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String(),
    ); err != nil {
        log.Printf("oidc start: set_config: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "tenant_setup_failed", "")
        return
    }

    q := db.New(tx)
    conn, err := q.GetEnabledDefaultSSOConnection(ctx)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "no_sso", "tenant has no enabled default SSO connection")
            return
        }
        log.Printf("oidc start: get connection: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "sso_get_failed", "")
        return
    }
    if conn.Kind != "oidc" {
        httpx.Error(w, http.StatusBadRequest, "not_oidc", "default SSO connection is not OIDC")
        return
    }
    if conn.OidcIssuer == nil || conn.OidcClientID == nil || len(conn.OidcClientSecretEnc) == 0 {
        httpx.Error(w, http.StatusInternalServerError, "incomplete_connection", "")
        return
    }

    // Decrypt client secret
    clientSecret, err := crypto.Decrypt(conn.OidcClientSecretEnc, h.SSOSecretKey)
    if err != nil {
        log.Printf("oidc start: decrypt secret: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "secret_decrypt_failed", "")
        return
    }

    // Resolve provider
    provider, err := auth.ResolveOIDCProvider(ctx, *conn.OidcIssuer)
    if err != nil {
        log.Printf("oidc start: discovery: %v", err)
        httpx.Error(w, http.StatusBadGateway, "discovery_failed", err.Error())
        return
    }

    // Build the OAuth2 config with our callback URL
    oauth2Cfg := auth.BuildOAuth2Config(
        provider,
        *conn.OidcClientID,
        string(clientSecret),
        h.callbackURL(),
        conn.OidcScopes,
    )

    // Generate state and nonce
    state, err := auth.NewOIDCState(tenantID, conn.ID, returnTo)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "state_failed", "")
        return
    }

    encoded, err := auth.EncodeOIDCState(state, h.SessionKey)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "encode_state_failed", "")
        return
    }
    auth.SetStateCookie(w, encoded, h.SecureCookies)

    // Redirect to IdP authorize endpoint
    authURL := oauth2Cfg.AuthCodeURL(state.State, oidcNonceOption(state.Nonce))
    http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *OIDCHandler) callbackURL() string {
    // The API is at API_BASE_URL. The callback must be routable from the
    // user's browser, so we build it from the API base, not the app base.
    return apiBaseURLFromRequest(h.AppBaseURL) + "/v1/auth/oidc/callback"
}

// ---------------- Callback ----------------

// Callback completes the authorization code flow.
// GET /v1/auth/oidc/callback?code=...&state=...
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
    defer cancel()

    rawState, err := auth.ReadStateCookie(r)
    if err != nil || rawState == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_state", "no OIDC state cookie")
        return
    }
    state, err := auth.DecodeOIDCState(rawState, h.SessionKey)
    if err != nil {
        log.Printf("oidc callback: decode state: %v", err)
        httpx.Error(w, http.StatusBadRequest, "bad_state", "state cookie invalid or expired")
        return
    }
    auth.ClearStateCookie(w)

    // Verify the state matches what the IdP sent back
    if qState := r.URL.Query().Get("state"); qState != state.State {
        httpx.Error(w, http.StatusBadRequest, "state_mismatch", "")
        return
    }
    if errParam := r.URL.Query().Get("error"); errParam != "" {
        desc := r.URL.Query().Get("error_description")
        log.Printf("oidc callback: IdP returned error %s: %s", errParam, desc)
        httpx.Error(w, http.StatusBadRequest, "idp_error", errParam+": "+desc)
        return
    }
    code := r.URL.Query().Get("code")
    if code == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_code", "")
        return
    }

    // Open tenant-scoped tx to fetch the connection
    tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        httpx.Error(w, http.StatusServiceUnavailable, "db_unavailable", "")
        return
    }
    defer tx.Rollback(context.Background())

    if _, err := tx.Exec(ctx,
        `SELECT set_config('app.tenant_id', $1, true)`, state.TenantID.String(),
    ); err != nil {
        log.Printf("oidc callback: set_config: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "tenant_setup_failed", "")
        return
    }

    q := db.New(tx)
    conn, err := q.GetSSOConnection(ctx, state.ConnectionID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "connection_gone", "SSO connection no longer exists")
            return
        }
        log.Printf("oidc callback: get connection: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "sso_get_failed", "")
        return
    }
    if !conn.Enabled {
        httpx.Error(w, http.StatusForbidden, "connection_disabled", "")
        return
    }

    clientSecret, err := crypto.Decrypt(conn.OidcClientSecretEnc, h.SSOSecretKey)
    if err != nil {
        log.Printf("oidc callback: decrypt secret: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "secret_decrypt_failed", "")
        return
    }

    provider, err := auth.ResolveOIDCProvider(ctx, *conn.OidcIssuer)
    if err != nil {
        log.Printf("oidc callback: discovery: %v", err)
        httpx.Error(w, http.StatusBadGateway, "discovery_failed", err.Error())
        return
    }

    oauth2Cfg := auth.BuildOAuth2Config(
        provider,
        *conn.OidcClientID,
        string(clientSecret),
        h.callbackURL(),
        conn.OidcScopes,
    )

    // Exchange code for tokens
    tok, err := oauth2Cfg.Exchange(ctx, code)
    if err != nil {
        log.Printf("oidc callback: exchange: %v", err)
        httpx.Error(w, http.StatusBadGateway, "exchange_failed", err.Error())
        return
    }

    rawIDToken, ok := tok.Extra("id_token").(string)
    if !ok || rawIDToken == "" {
        httpx.Error(w, http.StatusBadGateway, "missing_id_token", "")
        return
    }

    claims, err := auth.VerifyIDToken(ctx, provider, *conn.OidcClientID, rawIDToken, state.Nonce)
    if err != nil {
        log.Printf("oidc callback: verify: %v", err)
        httpx.Error(w, http.StatusUnauthorized, "id_token_invalid", err.Error())
        return
    }
    if claims.Email == "" {
        httpx.Error(w, http.StatusUnauthorized, "no_email_claim", "")
        return
    }

    // Look up existing user, or JIT-provision
    userID, err := resolveOrProvisionUser(ctx, tx, conn, claims)
    if err != nil {
        if errors.Is(err, errUserNotFound) {
            httpx.Error(w, http.StatusForbidden, "user_not_provisioned",
                "no account exists for "+claims.Email+" and JIT provisioning is disabled")
            return
        }
        log.Printf("oidc callback: user resolve: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "user_resolve_failed", err.Error())
        return
    }

    // Commit any JIT user creation before creating the session
    if err := tx.Commit(ctx); err != nil {
        log.Printf("oidc callback: commit: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "commit_failed", "")
        return
    }

    // Create session in a fresh tx
    sessionID, rawToken, expires, err := createSessionForUser(ctx, h.Pool, state.TenantID, userID, r)
    if err != nil {
        log.Printf("oidc callback: session: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "session_failed", err.Error())
        return
    }
    log.Printf("oidc callback: session %s created for user %s", sessionID, userID)

    auth.SetSessionCookie(w, rawToken, expires, h.SecureCookies)

    // Redirect the browser to the frontend
    dest := h.returnRedirect(state.ReturnTo)
    http.Redirect(w, r, dest, http.StatusFound)
}

// ---------------- helpers ----------------

var errUserNotFound = errors.New("user not found and JIT disabled")

func resolveOrProvisionUser(ctx context.Context, tx pgx.Tx, conn db.SsoConnection, claims *auth.IDTokenClaims) (uuid.UUID, error) {
    // Existing user? RLS scopes to the tenant.
    var userID uuid.UUID
    err := tx.QueryRow(ctx,
        `SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`,
        claims.Email,
    ).Scan(&userID)
    if err == nil {
        return userID, nil
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        return uuid.Nil, fmt.Errorf("lookup user: %w", err)
    }
    if !conn.JitProvisioning {
        return uuid.Nil, errUserNotFound
    }

    // Provision
    fullName := claims.Name
    if fullName == "" {
        fullName = claims.PreferredUsername
    }
    if fullName == "" {
        fullName = claims.Email
    }
    newID := uuid.Must(uuid.NewV7())
    if err := tx.QueryRow(ctx,
        `INSERT INTO users (id, tenant_id, email, full_name, external_id, status)
         VALUES ($1, $2, $3, $4, $5, 'active')
         RETURNING id`,
        newID, conn.TenantID, claims.Email, fullName, claims.Subject,
    ).Scan(&userID); err != nil {
        return uuid.Nil, fmt.Errorf("create user: %w", err)
    }

    // Assign default role if configured
    if conn.DefaultRoleKey != nil && *conn.DefaultRoleKey != "" {
        if _, err := tx.Exec(ctx,
            `INSERT INTO user_roles (id, tenant_id, user_id, role_id, scope_type)
             SELECT $1, $2, $3, r.id, 'tenant'
             FROM roles r
             WHERE r.key = $4 AND r.tenant_id = $2
             ON CONFLICT DO NOTHING`,
            uuid.Must(uuid.NewV7()), conn.TenantID, userID, *conn.DefaultRoleKey,
        ); err != nil {
            return uuid.Nil, fmt.Errorf("assign role: %w", err)
        }
    }

    log.Printf("oidc: JIT provisioned user %s (%s) with role %v",
        userID, claims.Email, conn.DefaultRoleKey)
    return userID, nil
}

func createSessionForUser(ctx context.Context, pool *pgxpool.Pool, tenantID, userID uuid.UUID, r *http.Request) (uuid.UUID, string, time.Time, error) {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return uuid.Nil, "", time.Time{}, err
    }
    defer tx.Rollback(context.Background())

    if _, err := tx.Exec(ctx,
        `SELECT set_config('app.tenant_id', $1, true)`, tenantID.String(),
    ); err != nil {
        return uuid.Nil, "", time.Time{}, err
    }
    q := db.New(tx)
    sid, raw, expires, err := auth.CreateSession(ctx, q, tenantID, userID, nil, userAgentPtr(r), auth.SessionAbsoluteTTL)
    if err != nil {
        return uuid.Nil, "", time.Time{}, err
    }
    if err := tx.Commit(ctx); err != nil {
        return uuid.Nil, "", time.Time{}, err
    }
    return sid, raw, expires, nil
}

func userAgentPtr(r *http.Request) *string {
    ua := r.UserAgent()
    if ua == "" {
        return nil
    }
    return &ua
}

func sanitizeReturnTo(s string) string {
    if s == "" {
        return "/learn"
    }
    if !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") {
        return "/learn"
    }
    return s
}

func (h *OIDCHandler) returnRedirect(returnTo string) string {
    if returnTo == "" {
        returnTo = "/learn"
    }
    return h.AppBaseURL + returnTo
}

func apiBaseURLFromRequest(_ string) string {
    // Prefer an env value if we want. For local dev, the API is at :8080.
    // We can override via API_BASE_URL later. For now hardcode.
    return "http://localhost:8080"
}