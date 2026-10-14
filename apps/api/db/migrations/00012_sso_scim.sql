-- +goose Up

CREATE TABLE sso_connections (
  id                     uuid PRIMARY KEY,
  tenant_id              uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  display_name           text NOT NULL,
  kind                   text NOT NULL CHECK (kind IN ('oidc','saml')),
  enabled                boolean NOT NULL DEFAULT false,
  is_default             boolean NOT NULL DEFAULT false,

  oidc_issuer            text,
  oidc_client_id         text,
  oidc_client_secret_enc bytea,
  oidc_scopes            text NOT NULL DEFAULT 'openid email profile',

  saml_idp_metadata_xml  text,
  saml_sp_entity_id      text,

  jit_provisioning       boolean NOT NULL DEFAULT false,
  default_role_key       text,

  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  created_by             uuid REFERENCES users(id),

  CHECK (
    (kind = 'oidc' AND oidc_issuer IS NOT NULL AND oidc_client_id IS NOT NULL)
    OR
    (kind = 'saml' AND saml_idp_metadata_xml IS NOT NULL)
  )
);
CREATE INDEX sso_connections_tenant_idx ON sso_connections (tenant_id);
CREATE UNIQUE INDEX sso_connections_default_idx
  ON sso_connections (tenant_id) WHERE is_default = true;

CREATE TABLE scim_tokens (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name          text NOT NULL,
  token_hash    text NOT NULL,
  token_prefix  text NOT NULL,
  last_used_at  timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  created_by    uuid REFERENCES users(id),
  revoked_at    timestamptz
);
CREATE INDEX scim_tokens_tenant_idx ON scim_tokens (tenant_id) WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON sso_connections TO app_rw;
GRANT SELECT, INSERT, UPDATE         ON scim_tokens     TO app_rw;

ALTER TABLE sso_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE sso_connections FORCE  ROW LEVEL SECURITY;
ALTER TABLE scim_tokens     ENABLE ROW LEVEL SECURITY;
ALTER TABLE scim_tokens     FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_sso_connections ON sso_connections
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_scim_tokens ON scim_tokens
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE scim_tokens, sso_connections CASCADE;