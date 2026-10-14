-- name: ListSSOConnections :many
SELECT id, tenant_id, display_name, kind, enabled, is_default,
       oidc_issuer, oidc_client_id, oidc_scopes,
       saml_sp_entity_id, jit_provisioning, default_role_key,
       created_at, updated_at
FROM sso_connections
ORDER BY display_name;

-- name: GetSSOConnection :one
SELECT id, tenant_id, display_name, kind, enabled, is_default,
       oidc_issuer, oidc_client_id, oidc_client_secret_enc, oidc_scopes,
       saml_idp_metadata_xml, saml_sp_entity_id, jit_provisioning, default_role_key,
       created_at, updated_at, created_by
FROM sso_connections
WHERE id = $1;

-- name: GetEnabledDefaultSSOConnection :one
SELECT id, tenant_id, display_name, kind, enabled, is_default,
       oidc_issuer, oidc_client_id, oidc_client_secret_enc, oidc_scopes,
       saml_idp_metadata_xml, saml_sp_entity_id, jit_provisioning, default_role_key,
       created_at, updated_at, created_by
FROM sso_connections
WHERE is_default = true AND enabled = true
LIMIT 1;

-- name: CreateSSOConnection :one
INSERT INTO sso_connections (
  id, tenant_id, display_name, kind, enabled, is_default,
  oidc_issuer, oidc_client_id, oidc_client_secret_enc, oidc_scopes,
  saml_idp_metadata_xml, saml_sp_entity_id,
  jit_provisioning, default_role_key, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, display_name, kind, enabled, is_default, created_at;

-- name: UpdateSSOConnectionEnabled :exec
UPDATE sso_connections
SET enabled = $2, is_default = $3, updated_at = now()
WHERE id = $1;

-- name: ClearDefaultSSOForTenant :exec
UPDATE sso_connections
SET is_default = false, updated_at = now()
WHERE tenant_id = $1 AND is_default = true;

-- name: DeleteSSOConnection :exec
DELETE FROM sso_connections WHERE id = $1;