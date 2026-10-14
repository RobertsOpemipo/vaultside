-- +goose Up

CREATE TABLE roles (
  id         uuid PRIMARY KEY,
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key        text NOT NULL,
  name       text NOT NULL,
  is_system  boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE INDEX roles_tenant_idx ON roles (tenant_id);

CREATE TABLE role_permissions (
  role_id    uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission text NOT NULL,
  PRIMARY KEY (role_id, permission)
);

CREATE TABLE user_roles (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  role_id     uuid NOT NULL REFERENCES roles(id)   ON DELETE CASCADE,
  scope_type  text NOT NULL DEFAULT 'tenant' CHECK (scope_type IN ('tenant','department','team')),
  scope_id    uuid,
  granted_at  timestamptz NOT NULL DEFAULT now(),
  granted_by  uuid REFERENCES users(id),
  UNIQUE NULLS NOT DISTINCT (tenant_id, user_id, role_id, scope_type, scope_id)
);
CREATE INDEX user_roles_user_idx ON user_roles (tenant_id, user_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON roles, role_permissions, user_roles TO app_rw;

ALTER TABLE roles            ENABLE ROW LEVEL SECURITY;
ALTER TABLE roles            FORCE  ROW LEVEL SECURITY;
ALTER TABLE role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_permissions FORCE  ROW LEVEL SECURITY;
ALTER TABLE user_roles       ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_roles       FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_roles ON roles
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_user_roles ON user_roles
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- role_permissions: no tenant_id; join via roles
CREATE POLICY tenant_isolation_role_permissions ON role_permissions
  USING (EXISTS (
    SELECT 1 FROM roles r
    WHERE r.id = role_permissions.role_id
      AND r.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
  ))
  WITH CHECK (EXISTS (
    SELECT 1 FROM roles r
    WHERE r.id = role_permissions.role_id
      AND r.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
  ));

-- +goose Down
DROP TABLE user_roles, role_permissions, roles CASCADE;