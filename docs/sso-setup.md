# SSO and SCIM setup - Entra ID and Okta

Gate 5 of Phase 5 requires passing provisioning and login checks against real
Microsoft Entra ID and Okta test tenants. That check is on the pre-pilot
checklist (see docs/pre-pilot-checklist.md). This document is the exact
configuration for each provider.

The Vaultside code is IdP-agnostic: the OIDC handler uses `oidc.NewProvider(issuer)`
which reads `/.well-known/openid-configuration`, and the SAML handler parses any
spec-compliant IdP metadata XML. Switching providers requires editing one row in
`sso_connections`. No code changes.

---

## Microsoft Entra ID

### 1. Register the application

Entra admin portal -> App registrations -> New registration.

    Name                Vaultside LMS
    Account types       Accounts in this organizational directory only
    Redirect URI        http://localhost:8080/v1/auth/oidc/callback
                        (and https://<your-domain>/v1/auth/oidc/callback in production)
    Platform            Web

After creating, note the Application (client) ID and Directory (tenant) ID from
the Overview page.

### 2. Create a client secret

Certificates & secrets -> New client secret. Copy the Value immediately - it is
shown once. This is the OIDC client secret.

### 3. Configure token claims (optional)

Token configuration -> Add optional claim -> ID -> select `email`, `given_name`,
`family_name`. Without this, `email` may not be present in the ID token;
Vaultside falls back to `preferred_username`.

### 4. Values for Vaultside

    kind                = "oidc"
    oidc_issuer         = "https://login.microsoftonline.com/<tenant-id>/v2.0"
    oidc_client_id      = "<application-client-id>"
    oidc_client_secret  = "<client-secret-value>"
    oidc_scopes         = "openid email profile"

### 5. Verify login

Point a browser at:

    http://localhost:8080/v1/auth/oidc/<tenant-slug>/start

Sign in as a user who exists in the Entra tenant and (for non-JIT) in Vaultside,
and confirm the callback sets a session.

---

## Okta

### 1. Create an OIDC web app

Okta admin -> Applications -> Create App Integration -> OIDC - OpenID Connect
-> Web Application.

    App integration name   Vaultside LMS
    Grant type             Authorization Code
    Sign-in redirect URI   http://localhost:8080/v1/auth/oidc/callback
                           (and the production URL)
    Sign-out redirect URI  http://localhost:3000/
    Controlled access      Assign to specific groups or "All users"

After creating, note the Client ID and Client secret. The Okta domain
(your-org.okta.com) is on the dashboard.

### 2. Values for Vaultside

    kind                = "oidc"
    oidc_issuer         = "https://<your-org>.okta.com/oauth2/default"
    oidc_client_id      = "<client-id>"
    oidc_client_secret  = "<client-secret>"
    oidc_scopes         = "openid email profile"

### 3. Verify login

Same as Entra - point the browser at /v1/auth/oidc/<tenant-slug>/start.

---

## SAML 2.0 with either provider

Vaultside's SAML SP metadata is served at:

    http://localhost:8080/v1/auth/saml/<tenant-slug>/metadata

### Entra ID - SAML app

1. Enterprise applications -> New application -> Create your own application
   -> Integrate any other application you don't find in the gallery (Non-gallery).
2. Single sign-on -> SAML.
3. Basic SAML Configuration:
   - Identifier (Entity ID): http://localhost:8080/v1/auth/saml/<tenant-slug>/metadata
   - Reply URL (ACS URL):     http://localhost:8080/v1/auth/saml/<tenant-slug>/acs
4. SAML Certificates -> download Federation Metadata XML.
5. Attributes & Claims -> ensure a claim that carries the user's email is emitted.
   Vaultside reads in this order:
   - NameID if it looks like an email
   - attribute named `email` or `mail`
   - attribute with FriendlyName = email
   - any attribute value containing `@`

### Okta - SAML app

1. Applications -> Create App Integration -> SAML 2.0.
2. General Settings:
   - Single sign-on URL: http://localhost:8080/v1/auth/saml/<tenant-slug>/acs
   - Audience URI:       http://localhost:8080/v1/auth/saml/<tenant-slug>/metadata
3. Attribute Statements: add a statement with Name `email` and Value `user.email`.
4. Finish the wizard and download the Identity Provider metadata from the Sign On tab.

### Values for Vaultside

    kind                   = "saml"
    saml_idp_metadata_xml  = "<contents of the IdP metadata XML>"
    saml_sp_entity_id      = "http://localhost:8080/v1/auth/saml/<tenant-slug>/metadata"

---

## SCIM 2.0 provisioning

SCIM is HTTP + JSON with a Bearer token. No IdP-specific configuration is
required on Vaultside's side beyond issuing a token.

1. Sign in as an admin and get a SCIM token:

       $h = @{
         "X-Dev-Tenant"     = "harbourline"
         "X-Dev-User-Email" = "ada.okafor@harbourline.test"
         "Content-Type"     = "application/json"
       }
       Invoke-RestMethod -Method Post `
         -Uri http://localhost:8080/v1/admin/scim/tokens `
         -Headers $h -Body '{"name":"entra-provisioning"}'

   The `token` field in the response is shown once. Store it in the IdP's
   provisioning configuration.

2. Provider-side config:
   - Entra ID: Enterprise applications -> your app -> Provisioning -> Automatic
     -> Tenant URL = https://<your-domain>/scim/v2, Secret Token = the token from
     step 1 -> Test Connection.
   - Okta: Applications -> your app -> Provisioning -> Configure API Integration
     -> Base URL = https://<your-domain>/scim/v2, API Token = the token.

3. Vaultside supports SCIM Users and Groups. Groups map to teams. Deprovisioning
   (active: false or DELETE) revokes the user's active sessions immediately.

### Verify provisioning

Run the connection test in the IdP; then confirm in Vaultside:

    cd $HOME\Desktop\Projects\vaultside
    docker compose exec postgres psql -U vaultside -d vaultside -c "SELECT email, status, updated_at FROM users ORDER BY updated_at DESC LIMIT 5;"

New users from the IdP should appear with status = 'active'. Removing a user in
the IdP should flip status to 'deprovisioned' (or set deleted_at).

---

## Adding a second IdP for the same tenant

Vaultside supports multiple SSO connections per tenant. Currently, /auth/oidc/{slug}/start
and /auth/saml/{slug}/start use the default connection for that kind. To add a
second OIDC provider (say, both Entra and Okta):

1. Create the second connection with is_default: false.
2. To test it, PATCH the two connections to flip which one is default:

       $h = @{ "X-Dev-Tenant"="harbourline"; "X-Dev-User-Email"="ada.okafor@harbourline.test"; "Content-Type"="application/json" }
       Invoke-RestMethod -Method Patch `
         -Uri http://localhost:8080/v1/admin/sso/connections/<current-default-id> `
         -Headers $h -Body '{"is_default":false}'
       Invoke-RestMethod -Method Patch `
         -Uri http://localhost:8080/v1/admin/sso/connections/<other-id> `
         -Headers $h -Body '{"is_default":true}'

3. Add /auth/oidc/{slug}/start?connection=<id> (or a picker page) in a future
   phase to avoid needing to flip defaults manually.