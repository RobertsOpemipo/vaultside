package handlers

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
    "github.com/vaultside/vaultside/apps/api/internal/platform/crypto"
)

type SSOHandler struct {
    secretKey []byte
    http      *http.Client
}

func NewSSOHandler(secretKey []byte) *SSOHandler {
    return &SSOHandler{
        secretKey: secretKey,
        http:      &http.Client{Timeout: 10 * time.Second},
    }
}

// ---------------- List ----------------

type ssoListItem struct {
    ID              string `json:"id"`
    DisplayName     string `json:"display_name"`
    Kind            string `json:"kind"`
    Enabled         bool   `json:"enabled"`
    IsDefault       bool   `json:"is_default"`
    OidcIssuer      string `json:"oidc_issuer,omitempty"`
    OidcClientID    string `json:"oidc_client_id,omitempty"`
    OidcScopes      string `json:"oidc_scopes,omitempty"`
    SamlSpEntityID  string `json:"saml_sp_entity_id,omitempty"`
    JitProvisioning bool   `json:"jit_provisioning"`
    DefaultRoleKey  string `json:"default_role_key,omitempty"`
    CreatedAt       string `json:"created_at"`
    UpdatedAt       string `json:"updated_at"`
}

func (h *SSOHandler) List(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListSSOConnections(r.Context())
    if err != nil {
        log.Printf("sso: list: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]ssoListItem, 0, len(rows))
    for _, row := range rows {
        item := ssoListItem{
            ID:              row.ID.String(),
            DisplayName:     row.DisplayName,
            Kind:            row.Kind,
            Enabled:         row.Enabled,
            IsDefault:       row.IsDefault,
            OidcScopes:      row.OidcScopes,
            JitProvisioning: row.JitProvisioning,
            CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
            UpdatedAt:       row.UpdatedAt.UTC().Format(time.RFC3339),
        }
        if row.OidcIssuer != nil {
            item.OidcIssuer = *row.OidcIssuer
        }
        if row.OidcClientID != nil {
            item.OidcClientID = *row.OidcClientID
        }
        if row.SamlSpEntityID != nil {
            item.SamlSpEntityID = *row.SamlSpEntityID
        }
        if row.DefaultRoleKey != nil {
            item.DefaultRoleKey = *row.DefaultRoleKey
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"connections": out})
}

// ---------------- Get detail ----------------

type ssoDetail struct {
    ID              string `json:"id"`
    DisplayName     string `json:"display_name"`
    Kind            string `json:"kind"`
    Enabled         bool   `json:"enabled"`
    IsDefault       bool   `json:"is_default"`
    OidcIssuer      string `json:"oidc_issuer,omitempty"`
    OidcClientID    string `json:"oidc_client_id,omitempty"`
    OidcScopes      string `json:"oidc_scopes,omitempty"`
    SamlSpEntityID  string `json:"saml_sp_entity_id,omitempty"`
    JitProvisioning bool   `json:"jit_provisioning"`
    DefaultRoleKey  string `json:"default_role_key,omitempty"`
    HasSecret       bool   `json:"has_secret"`
    HasSamlMetadata bool   `json:"has_saml_metadata"`
    CreatedAt       string `json:"created_at"`
    UpdatedAt       string `json:"updated_at"`
}

func (h *SSOHandler) Get(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid connection id")
        return
    }

    row, err := q.GetSSOConnection(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "connection not found")
            return
        }
        log.Printf("sso: get: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    out := ssoDetail{
        ID:              row.ID.String(),
        DisplayName:     row.DisplayName,
        Kind:            row.Kind,
        Enabled:         row.Enabled,
        IsDefault:       row.IsDefault,
        OidcScopes:      row.OidcScopes,
        JitProvisioning: row.JitProvisioning,
        HasSecret:       len(row.OidcClientSecretEnc) > 0,
        HasSamlMetadata: row.SamlIdpMetadataXml != nil && *row.SamlIdpMetadataXml != "",
        CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
        UpdatedAt:       row.UpdatedAt.UTC().Format(time.RFC3339),
    }
    if row.OidcIssuer != nil {
        out.OidcIssuer = *row.OidcIssuer
    }
    if row.OidcClientID != nil {
        out.OidcClientID = *row.OidcClientID
    }
    if row.SamlSpEntityID != nil {
        out.SamlSpEntityID = *row.SamlSpEntityID
    }
    if row.DefaultRoleKey != nil {
        out.DefaultRoleKey = *row.DefaultRoleKey
    }
    httpx.JSON(w, http.StatusOK, out)
}

// ---------------- Create ----------------

type createSSORequest struct {
    DisplayName        string `json:"display_name"`
    Kind               string `json:"kind"`
    Enabled            bool   `json:"enabled"`
    IsDefault          bool   `json:"is_default"`
    OidcIssuer         string `json:"oidc_issuer"`
    OidcClientID       string `json:"oidc_client_id"`
    OidcClientSecret   string `json:"oidc_client_secret"`
    OidcScopes         string `json:"oidc_scopes"`
    SamlIdpMetadataXml string `json:"saml_idp_metadata_xml"`
    SamlSpEntityID     string `json:"saml_sp_entity_id"`
    JitProvisioning    bool   `json:"jit_provisioning"`
    DefaultRoleKey     string `json:"default_role_key"`
}

func (h *SSOHandler) Create(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    var req createSSORequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }

    req.DisplayName = strings.TrimSpace(req.DisplayName)
    if req.DisplayName == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_display_name", "")
        return
    }
    if req.Kind != "oidc" && req.Kind != "saml" {
        httpx.Error(w, http.StatusBadRequest, "bad_kind", "kind must be oidc or saml")
        return
    }

    scopes := req.OidcScopes
    if scopes == "" {
        scopes = "openid email profile"
    }

    var oidcIssuer, oidcClientID, samlMeta, samlEntity, defaultRole *string
    var secretEnc []byte

    switch req.Kind {
    case "oidc":
        if req.OidcIssuer == "" || req.OidcClientID == "" {
            httpx.Error(w, http.StatusBadRequest, "missing_oidc_fields",
                "oidc_issuer and oidc_client_id are required")
            return
        }
        if req.OidcClientSecret == "" {
            httpx.Error(w, http.StatusBadRequest, "missing_secret",
                "oidc_client_secret is required")
            return
        }
        enc, err := crypto.Encrypt([]byte(req.OidcClientSecret), h.secretKey)
        if err != nil {
            log.Printf("sso: encrypt secret: %v", err)
            httpx.Error(w, http.StatusInternalServerError, "encrypt_failed", "")
            return
        }
        secretEnc = enc
        oidcIssuer = &req.OidcIssuer
        oidcClientID = &req.OidcClientID
    case "saml":
        if req.SamlIdpMetadataXml == "" {
            httpx.Error(w, http.StatusBadRequest, "missing_saml_metadata",
                "saml_idp_metadata_xml is required")
            return
        }
        samlMeta = &req.SamlIdpMetadataXml
        if req.SamlSpEntityID != "" {
            samlEntity = &req.SamlSpEntityID
        }
    }

    if req.DefaultRoleKey != "" {
        defaultRole = &req.DefaultRoleKey
    }

    // If this is being marked as default, clear any prior default first.
    if req.IsDefault {
        if err := q.ClearDefaultSSOForTenant(r.Context(), claims.TenantID); err != nil {
            log.Printf("sso: clear default: %v", err)
            httpx.Error(w, http.StatusInternalServerError, "clear_default_failed", "")
            return
        }
    }

    newID := uuid.Must(uuid.NewV7())
    row, err := q.CreateSSOConnection(r.Context(), db.CreateSSOConnectionParams{
        ID:                  newID,
        TenantID:            claims.TenantID,
        DisplayName:         req.DisplayName,
        Kind:                req.Kind,
        Enabled:             req.Enabled,
        IsDefault:           req.IsDefault,
        OidcIssuer:          oidcIssuer,
        OidcClientID:        oidcClientID,
        OidcClientSecretEnc: secretEnc,
        OidcScopes:          scopes,
        SamlIdpMetadataXml:  samlMeta,
        SamlSpEntityID:      samlEntity,
        JitProvisioning:     req.JitProvisioning,
        DefaultRoleKey:      defaultRole,
        CreatedBy:           pgtype.UUID{Bytes: claims.UserID, Valid: true},
    })
    if err != nil {
        log.Printf("sso: create: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "create_failed", "")
        return
    }

    httpx.JSON(w, http.StatusCreated, map[string]any{
        "id":           row.ID.String(),
        "display_name": row.DisplayName,
        "kind":         row.Kind,
        "enabled":      row.Enabled,
        "is_default":   row.IsDefault,
    })
}

// ---------------- Update ----------------

type updateSSORequest struct {
    Enabled   *bool `json:"enabled,omitempty"`
    IsDefault *bool `json:"is_default,omitempty"`
}

func (h *SSOHandler) Update(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid connection id")
        return
    }

    existing, err := q.GetSSOConnection(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "connection not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    var req updateSSORequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }

    enabled := existing.Enabled
    if req.Enabled != nil {
        enabled = *req.Enabled
    }
    isDefault := existing.IsDefault
    if req.IsDefault != nil {
        isDefault = *req.IsDefault
    }

    // If turning on default, clear siblings first.
    if isDefault && !existing.IsDefault {
        if err := q.ClearDefaultSSOForTenant(r.Context(), claims.TenantID); err != nil {
            log.Printf("sso: clear default: %v", err)
            httpx.Error(w, http.StatusInternalServerError, "clear_default_failed", "")
            return
        }
    }

    if err := q.UpdateSSOConnectionEnabled(r.Context(), db.UpdateSSOConnectionEnabledParams{
        ID:        id,
        Enabled:   enabled,
        IsDefault: isDefault,
    }); err != nil {
        log.Printf("sso: update: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "update_failed", "")
        return
    }

    httpx.JSON(w, http.StatusOK, map[string]any{
        "id":         id.String(),
        "enabled":    enabled,
        "is_default": isDefault,
    })
}

// ---------------- Delete ----------------

func (h *SSOHandler) Delete(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid connection id")
        return
    }

    if err := q.DeleteSSOConnection(r.Context(), id); err != nil {
        log.Printf("sso: delete: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "delete_failed", "")
        return
    }

    httpx.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---------------- Test connection ----------------

type testResponse struct {
    OK      bool              `json:"ok"`
    Kind    string            `json:"kind"`
    Details map[string]string `json:"details,omitempty"`
    Error   string            `json:"error,omitempty"`
}

type oidcDiscovery struct {
    Issuer                string `json:"issuer"`
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint         string `json:"token_endpoint"`
    JWKSURI               string `json:"jwks_uri"`
}

func (h *SSOHandler) Test(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid connection id")
        return
    }

    conn, err := q.GetSSOConnection(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "connection not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    switch conn.Kind {
    case "oidc":
        h.testOIDC(ctx, w, conn)
    case "saml":
        h.testSAML(ctx, w, conn)
    default:
        httpx.Error(w, http.StatusBadRequest, "bad_kind", "unknown connection kind")
    }
}

func (h *SSOHandler) testOIDC(ctx context.Context, w http.ResponseWriter, conn db.SsoConnection) {
    if conn.OidcIssuer == nil || *conn.OidcIssuer == "" {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "oidc", Error: "no issuer configured",
        })
        return
    }
    issuer := strings.TrimRight(*conn.OidcIssuer, "/")
    url := issuer + "/.well-known/openid-configuration"

    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := h.http.Do(req)
    if err != nil {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "oidc", Error: fmt.Sprintf("fetch discovery: %v", err),
        })
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "oidc",
            Error: fmt.Sprintf("discovery returned HTTP %d", resp.StatusCode),
        })
        return
    }

    body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
    if err != nil {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "oidc", Error: "read discovery response",
        })
        return
    }

    var disc oidcDiscovery
    if err := json.Unmarshal(body, &disc); err != nil {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "oidc", Error: "discovery document is not valid JSON",
        })
        return
    }
    if disc.AuthorizationEndpoint == "" || disc.TokenEndpoint == "" {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "oidc", Error: "discovery missing authorization_endpoint or token_endpoint",
        })
        return
    }

    httpx.JSON(w, http.StatusOK, testResponse{
        OK:   true,
        Kind: "oidc",
        Details: map[string]string{
            "issuer":                 disc.Issuer,
            "authorization_endpoint": disc.AuthorizationEndpoint,
            "token_endpoint":         disc.TokenEndpoint,
            "jwks_uri":               disc.JWKSURI,
        },
    })
}

func (h *SSOHandler) testSAML(_ context.Context, w http.ResponseWriter, conn db.SsoConnection) {
    if conn.SamlIdpMetadataXml == nil || *conn.SamlIdpMetadataXml == "" {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "saml", Error: "no IdP metadata configured",
        })
        return
    }
    // Minimal structural check: does the XML contain an EntityDescriptor?
    // Full SAML metadata parsing lands in Part D.
    if !strings.Contains(*conn.SamlIdpMetadataXml, "EntityDescriptor") {
        httpx.JSON(w, http.StatusOK, testResponse{
            OK: false, Kind: "saml", Error: "metadata does not contain EntityDescriptor",
        })
        return
    }
    httpx.JSON(w, http.StatusOK, testResponse{
        OK:   true,
        Kind: "saml",
        Details: map[string]string{
            "metadata_bytes": fmt.Sprintf("%d", len(*conn.SamlIdpMetadataXml)),
        },
    })
}