package handlers

import (
    "context"
    "encoding/base64"
    "encoding/xml"
    "errors"
    "fmt"
    "log"
    "net/http"
    "net/url"
    "strings"
    "time"

    "github.com/crewjam/saml"
    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type SAMLHandler struct {
    Pool          *pgxpool.Pool
    Keypair       *auth.SAMLKeypair
    APIBaseURL    string
    AppBaseURL    string
    SessionKey    []byte
    SecureCookies bool
}

// ---------------- Metadata ----------------

func (h *SAMLHandler) Metadata(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    if slug == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_slug", "")
        return
    }
    var tenantID string
    if err := h.Pool.QueryRow(r.Context(),
        `SELECT id::text FROM resolve_tenant_slug($1)`, slug,
    ).Scan(&tenantID); err != nil {
        httpx.Error(w, http.StatusNotFound, "not_found", "unknown tenant")
        return
    }

    type keyInfo struct {
        X509Data struct {
            X509Certificate string `xml:"X509Certificate"`
        } `xml:"X509Data"`
    }
    type keyDescriptor struct {
        Use     string  `xml:"use,attr"`
        KeyInfo keyInfo `xml:"KeyInfo"`
    }
    type acs struct {
        Binding  string `xml:"Binding,attr"`
        Location string `xml:"Location,attr"`
        Index    int    `xml:"index,attr"`
    }
    type spSSO struct {
        ProtocolSupportEnumeration string        `xml:"protocolSupportEnumeration,attr"`
        AuthnRequestsSigned        bool          `xml:"AuthnRequestsSigned,attr"`
        WantAssertionsSigned       bool          `xml:"WantAssertionsSigned,attr"`
        KeyDescriptor              keyDescriptor `xml:"KeyDescriptor"`
        AssertionConsumerService   acs           `xml:"AssertionConsumerService"`
        NameIDFormat               string        `xml:"NameIDFormat"`
    }
    type entityDescriptor struct {
        XMLName        xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
        EntityID       string   `xml:"entityID,attr"`
        SPSSODescriptor spSSO   `xml:"SPSSODescriptor"`
    }

    certB64 := base64.StdEncoding.EncodeToString(h.Keypair.Certificate.Raw)
    var m entityDescriptor
    m.EntityID = h.spEntityID(slug)
    m.SPSSODescriptor.ProtocolSupportEnumeration = "urn:oasis:names:tc:SAML:2.0:protocol"
    m.SPSSODescriptor.AuthnRequestsSigned = false
    m.SPSSODescriptor.WantAssertionsSigned = true
    m.SPSSODescriptor.KeyDescriptor.Use = "signing"
    m.SPSSODescriptor.KeyDescriptor.KeyInfo.X509Data.X509Certificate = certB64
    m.SPSSODescriptor.AssertionConsumerService.Binding = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
    m.SPSSODescriptor.AssertionConsumerService.Location = h.acsURL(slug)
    m.SPSSODescriptor.AssertionConsumerService.Index = 0
    m.SPSSODescriptor.NameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"

    out, err := xml.MarshalIndent(m, "", "  ")
    if err != nil {
        log.Printf("saml metadata: marshal: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "marshal_failed", "")
        return
    }
    full := append([]byte(xml.Header), out...)
    w.Header().Set("Content-Type", "application/samlmetadata+xml")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "vaultside-sp-"+slug+".xml"))
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write(full)
}

// ---------------- Start ----------------

func (h *SAMLHandler) Start(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    if slug == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_slug", "")
        return
    }
    returnTo := sanitizeReturnTo(r.URL.Query().Get("returnTo"))

    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    var tenantID string
    if err := h.Pool.QueryRow(ctx,
        `SELECT id::text FROM resolve_tenant_slug($1)`, slug,
    ).Scan(&tenantID); err != nil {
        httpx.Error(w, http.StatusNotFound, "not_found", "unknown tenant")
        return
    }
    tenantUUID, err := uuid.Parse(tenantID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "bad_tenant_id", "")
        return
    }

    tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        httpx.Error(w, http.StatusServiceUnavailable, "db_unavailable", "")
        return
    }
    defer tx.Rollback(context.Background())

    if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
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
        httpx.Error(w, http.StatusInternalServerError, "sso_get_failed", "")
        return
    }
    if conn.Kind != "saml" {
        httpx.Error(w, http.StatusBadRequest, "not_saml", "default SSO connection is not SAML")
        return
    }
    if conn.SamlIdpMetadataXml == nil || *conn.SamlIdpMetadataXml == "" {
        httpx.Error(w, http.StatusInternalServerError, "incomplete_connection", "no IdP metadata")
        return
    }

    sp, ssoURL, err := h.buildSP(slug, []byte(*conn.SamlIdpMetadataXml))
    if err != nil {
        log.Printf("saml start: build sp: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "sp_build_failed", err.Error())
        return
    }

    state, err := auth.NewSAMLState(tenantUUID, conn.ID, returnTo)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "state_failed", "")
        return
    }

    req, err := sp.MakeAuthenticationRequest(ssoURL, saml.HTTPRedirectBinding, saml.HTTPPostBinding)
    if err != nil {
        log.Printf("saml start: build authn: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "authn_build_failed", err.Error())
        return
    }
    req.ID = state.RequestID

    redirectURL, err := req.Redirect(state.RelayState, sp)
    if err != nil {
        log.Printf("saml start: redirect: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "redirect_build_failed", err.Error())
        return
    }

    encoded, err := auth.EncodeSAMLState(state, h.SessionKey)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "encode_state_failed", "")
        return
    }
    auth.SetSAMLStateCookie(w, encoded, h.SecureCookies)

    http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// ---------------- ACS ----------------

func (h *SAMLHandler) ACS(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    if slug == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_slug", "")
        return
    }

    ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
    defer cancel()

    if err := r.ParseForm(); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_form", "")
        return
    }
    if r.FormValue("SAMLResponse") == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_saml_response", "")
        return
    }

    rawState, err := auth.ReadSAMLStateCookie(r)
    if err != nil || rawState == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_state", "no SAML state cookie")
        return
    }
    state, err := auth.DecodeSAMLState(rawState, h.SessionKey)
    if err != nil {
        log.Printf("saml acs: decode state: %v", err)
        httpx.Error(w, http.StatusBadRequest, "bad_state", "state cookie invalid or expired")
        return
    }
    auth.ClearSAMLStateCookie(w)

    if r.FormValue("RelayState") != state.RelayState {
        httpx.Error(w, http.StatusBadRequest, "relay_state_mismatch", "")
        return
    }

    tx, err := h.Pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
        httpx.Error(w, http.StatusServiceUnavailable, "db_unavailable", "")
        return
    }
    defer tx.Rollback(context.Background())

    if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, state.TenantID.String()); err != nil {
        httpx.Error(w, http.StatusInternalServerError, "tenant_setup_failed", "")
        return
    }

    q := db.New(tx)
    conn, err := q.GetSSOConnection(ctx, state.ConnectionID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "connection_gone", "")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "sso_get_failed", "")
        return
    }
    if !conn.Enabled {
        httpx.Error(w, http.StatusForbidden, "connection_disabled", "")
        return
    }
    if conn.SamlIdpMetadataXml == nil || *conn.SamlIdpMetadataXml == "" {
        httpx.Error(w, http.StatusInternalServerError, "incomplete_connection", "")
        return
    }

    sp, _, err := h.buildSP(slug, []byte(*conn.SamlIdpMetadataXml))
    if err != nil {
        log.Printf("saml acs: build sp: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "sp_build_failed", err.Error())
        return
    }

    assertion, err := sp.ParseResponse(r, []string{state.RequestID})
    if err != nil {
        log.Printf("saml acs: parse response: %v", err)
        httpx.Error(w, http.StatusUnauthorized, "response_invalid", err.Error())
        return
    }

    email := extractEmail(assertion)
    if email == "" {
        httpx.Error(w, http.StatusUnauthorized, "no_email", "SAML response has no email")
        return
    }
    fullName := extractDisplayName(assertion, email)
    externalID := ""
    if assertion.Subject != nil && assertion.Subject.NameID != nil {
        externalID = assertion.Subject.NameID.Value
    }

    userID, err := resolveOrProvisionSAMLUser(ctx, tx, conn, email, fullName, externalID)
    if err != nil {
        if errors.Is(err, errUserNotFound) {
            httpx.Error(w, http.StatusForbidden, "user_not_provisioned",
                "no account exists for "+email+" and JIT provisioning is disabled")
            return
        }
        log.Printf("saml acs: user resolve: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "user_resolve_failed", err.Error())
        return
    }

    if err := tx.Commit(ctx); err != nil {
        log.Printf("saml acs: commit: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "commit_failed", "")
        return
    }

    sessionID, rawToken, expires, err := createSessionForUser(ctx, h.Pool, state.TenantID, userID, r)
    if err != nil {
        log.Printf("saml acs: session: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "session_failed", err.Error())
        return
    }
    log.Printf("saml acs: session %s created for user %s", sessionID, userID)

    auth.SetSessionCookie(w, rawToken, expires, h.SecureCookies)

    dest := h.AppBaseURL + state.ReturnTo
    if state.ReturnTo == "" {
        dest = h.AppBaseURL + "/learn"
    }
    http.Redirect(w, r, dest, http.StatusFound)
}

// ---------------- helpers ----------------

func (h *SAMLHandler) buildSP(slug string, idpXML []byte) (*saml.ServiceProvider, string, error) {
    var idpMeta saml.EntityDescriptor
    if err := xml.Unmarshal(idpXML, &idpMeta); err != nil {
        return nil, "", fmt.Errorf("parse idp metadata: %w", err)
    }
    ssoURL := pickSSOEndpoint(idpMeta)
    if ssoURL == "" {
        return nil, "", fmt.Errorf("no SSO endpoint in IdP metadata")
    }

    acsURL, err := url.Parse(h.acsURL(slug))
    if err != nil {
        return nil, "", fmt.Errorf("parse acs url: %w", err)
    }
    metaURL, err := url.Parse(h.spEntityID(slug))
    if err != nil {
        return nil, "", fmt.Errorf("parse meta url: %w", err)
    }

    sp := &saml.ServiceProvider{
        EntityID:          h.spEntityID(slug),
        Key:               h.Keypair.PrivateKey,
        Certificate:       h.Keypair.Certificate,
        MetadataURL:       *metaURL,
        AcsURL:            *acsURL,
        IDPMetadata:       &idpMeta,
        AllowIDPInitiated: false,
    }
    return sp, ssoURL, nil
}

func pickSSOEndpoint(meta saml.EntityDescriptor) string {
    for _, idp := range meta.IDPSSODescriptors {
        for _, svc := range idp.SingleSignOnServices {
            if svc.Binding == saml.HTTPRedirectBinding || svc.Binding == saml.HTTPPostBinding {
                return svc.Location
            }
        }
    }
    return ""
}

func extractEmail(a *saml.Assertion) string {
    if a == nil {
        return ""
    }
    // Prefer NameID when it is an email.
    if a.Subject != nil && a.Subject.NameID != nil {
        val := a.Subject.NameID.Value
        if strings.Contains(val, "@") {
            return val
        }
    }
    // Email OIDs used by various IdPs.
    emailOIDs := map[string]bool{
        "urn:oid:0.9.2342.19200300.100.1.3": true, // mail
        "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress": true,
    }
    for _, stmt := range a.AttributeStatements {
        for _, attr := range stmt.Attributes {
            name := strings.ToLower(attr.Name)
            friendly := strings.ToLower(attr.FriendlyName)
            match := emailOIDs[name] ||
                name == "email" || name == "mail" ||
                friendly == "email" || friendly == "mail" ||
                strings.HasSuffix(name, "emailaddress") ||
                strings.HasSuffix(name, "email") ||
                strings.HasSuffix(friendly, "emailaddress") ||
                strings.HasSuffix(friendly, "email")
            if !match {
                continue
            }
            for _, v := range attr.Values {
                if strings.Contains(v.Value, "@") {
                    return v.Value
                }
            }
        }
    }
    // Last resort: scan every attribute for any @ value.
    for _, stmt := range a.AttributeStatements {
        for _, attr := range stmt.Attributes {
            for _, v := range attr.Values {
                if strings.Contains(v.Value, "@") && !strings.Contains(v.Value, " ") {
                    return v.Value
                }
            }
        }
    }
    return ""
}

func extractDisplayName(a *saml.Assertion, fallback string) string {
    if a == nil {
        return fallback
    }
    for _, stmt := range a.AttributeStatements {
        for _, attr := range stmt.Attributes {
            name := strings.ToLower(attr.Name)
            if name == "displayname" || name == "name" ||
                strings.HasSuffix(name, "displayname") || strings.HasSuffix(name, "fullname") {
                if len(attr.Values) > 0 && attr.Values[0].Value != "" {
                    return attr.Values[0].Value
                }
            }
        }
    }
    if a.Subject != nil && a.Subject.NameID != nil {
        return a.Subject.NameID.Value
    }
    return fallback
}

func resolveOrProvisionSAMLUser(
    ctx context.Context, tx pgx.Tx, conn db.SsoConnection,
    email, fullName, externalID string,
) (uuid.UUID, error) {
    var userID uuid.UUID
    err := tx.QueryRow(ctx,
        `SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`, email,
    ).Scan(&userID)
    if err == nil {
        return userID, nil
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        return uuid.Nil, fmt.Errorf("lookup: %w", err)
    }
    if !conn.JitProvisioning {
        return uuid.Nil, errUserNotFound
    }
    if fullName == "" {
        fullName = email
    }
    newID := uuid.Must(uuid.NewV7())
    if err := tx.QueryRow(ctx,
        `INSERT INTO users (id, tenant_id, email, full_name, external_id, status)
         VALUES ($1, $2, $3, $4, $5, 'active') RETURNING id`,
        newID, conn.TenantID, email, fullName, externalID,
    ).Scan(&userID); err != nil {
        return uuid.Nil, fmt.Errorf("create user: %w", err)
    }
    if conn.DefaultRoleKey != nil && *conn.DefaultRoleKey != "" {
        _, _ = tx.Exec(ctx,
            `INSERT INTO user_roles (id, tenant_id, user_id, role_id, scope_type)
             SELECT $1, $2, $3, r.id, 'tenant' FROM roles r
             WHERE r.key = $4 AND r.tenant_id = $2 ON CONFLICT DO NOTHING`,
            uuid.Must(uuid.NewV7()), conn.TenantID, userID, *conn.DefaultRoleKey,
        )
    }
    log.Printf("saml: JIT provisioned user %s (%s)", userID, email)
    return userID, nil
}

func (h *SAMLHandler) spEntityID(slug string) string {
    base := strings.TrimRight(h.APIBaseURL, "/")
    return base + "/v1/auth/saml/" + slug + "/metadata"
}

func (h *SAMLHandler) acsURL(slug string) string {
    base := strings.TrimRight(h.APIBaseURL, "/")
    return base + "/v1/auth/saml/" + slug + "/acs"
}