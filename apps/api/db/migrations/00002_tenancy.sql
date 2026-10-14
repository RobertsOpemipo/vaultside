-- +goose Up

-- Application role. No BYPASSRLS, does not own tables.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_rw') THEN
    CREATE ROLE app_rw LOGIN PASSWORD 'app_rw_dev_only';
  END IF;
END$$;
-- +goose StatementEnd

CREATE TABLE tenants (
  id          uuid PRIMARY KEY,
  slug        text NOT NULL UNIQUE,
  name        text NOT NULL,
  theme       text NOT NULL DEFAULT 'expedition-atlas',
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  email          citext NOT NULL,
  full_name      text NOT NULL,
  external_id    text,
  department_id  uuid,
  manager_id     uuid REFERENCES users(id),
  hired_on       date,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deprovisioned')),
  last_login_at  timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  UNIQUE (tenant_id, email)
);
CREATE INDEX users_tenant_idx        ON users (tenant_id);
CREATE INDEX users_tenant_email_idx  ON users (tenant_id, email);
CREATE INDEX users_external_idx      ON users (tenant_id, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX users_manager_idx       ON users (tenant_id, manager_id)  WHERE manager_id IS NOT NULL;

CREATE TABLE sessions (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  token_hash  text NOT NULL UNIQUE,
  ip          inet,
  user_agent  text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz
);
CREATE INDEX sessions_tenant_idx ON sessions (tenant_id);
CREATE INDEX sessions_user_idx   ON sessions (user_id);

CREATE TABLE invitations (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  email        citext NOT NULL,
  role_key     text NOT NULL,
  token_hash   text NOT NULL UNIQUE,
  invited_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz NOT NULL,
  accepted_at  timestamptz
);
CREATE INDEX invitations_tenant_idx ON invitations (tenant_id);
CREATE UNIQUE INDEX invitations_pending_idx ON invitations (tenant_id, email) WHERE accepted_at IS NULL;

CREATE TABLE audit_log (
  id           bigserial PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  actor_id     uuid,
  actor_type   text NOT NULL DEFAULT 'user' CHECK (actor_type IN ('user','system','api_token','ai')),
  action       text NOT NULL,
  entity_type  text NOT NULL,
  entity_id    uuid,
  detail       jsonb NOT NULL DEFAULT '{}',
  ip           inet,
  user_agent   text,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_tenant_created_idx ON audit_log (tenant_id, created_at DESC);
CREATE INDEX audit_log_entity_idx         ON audit_log (tenant_id, entity_type, entity_id);

GRANT USAGE ON SCHEMA public TO app_rw;
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants, users, sessions, invitations TO app_rw;
GRANT SELECT, INSERT                 ON audit_log TO app_rw;
GRANT USAGE, SELECT                  ON SEQUENCE audit_log_id_seq TO app_rw;

ALTER TABLE users       ENABLE ROW LEVEL SECURITY;
ALTER TABLE users       FORCE  ROW LEVEL SECURITY;
ALTER TABLE sessions    ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions    FORCE  ROW LEVEL SECURITY;
ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations FORCE  ROW LEVEL SECURITY;
ALTER TABLE audit_log   ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log   FORCE  ROW LEVEL SECURITY;

ALTER TABLE tenants     ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_users ON users
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_sessions ON sessions
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_invitations ON invitations
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_audit_log ON audit_log
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_self ON tenants
  USING      (id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

REVOKE UPDATE, DELETE ON audit_log FROM app_rw;

-- +goose Down
DROP TABLE audit_log, invitations, sessions, users, tenants CASCADE;
DROP ROLE IF EXISTS app_rw;