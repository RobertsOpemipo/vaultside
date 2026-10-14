-- +goose Up

CREATE TABLE departments (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name        text NOT NULL,
  parent_id   uuid REFERENCES departments(id),
  external_id text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);
CREATE INDEX departments_tenant_idx ON departments (tenant_id);

CREATE TABLE teams (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name          text NOT NULL,
  department_id uuid REFERENCES departments(id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);
CREATE INDEX teams_tenant_idx ON teams (tenant_id);

CREATE TABLE team_members (
  team_id    uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       text NOT NULL DEFAULT 'member' CHECK (role IN ('member','lead')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (team_id, user_id)
);

ALTER TABLE users ADD CONSTRAINT users_department_fk
  FOREIGN KEY (department_id) REFERENCES departments(id);

GRANT SELECT, INSERT, UPDATE, DELETE ON departments, teams, team_members TO app_rw;

ALTER TABLE departments  ENABLE ROW LEVEL SECURITY;
ALTER TABLE departments  FORCE  ROW LEVEL SECURITY;
ALTER TABLE teams        ENABLE ROW LEVEL SECURITY;
ALTER TABLE teams        FORCE  ROW LEVEL SECURITY;
ALTER TABLE team_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_members FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_departments ON departments
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_teams ON teams
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- team_members has no tenant_id column; policy joins to its parent team.
CREATE POLICY tenant_isolation_team_members ON team_members
  USING (EXISTS (
    SELECT 1 FROM teams t
    WHERE t.id = team_members.team_id
      AND t.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
  ))
  WITH CHECK (EXISTS (
    SELECT 1 FROM teams t
    WHERE t.id = team_members.team_id
      AND t.tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
  ));

-- +goose Down
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_department_fk;
DROP TABLE team_members, teams, departments CASCADE;