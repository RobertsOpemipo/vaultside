-- +goose Up

CREATE TABLE learning_paths (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key         text NOT NULL,
  title       text NOT NULL,
  description text NOT NULL DEFAULT '',
  kind        text NOT NULL DEFAULT 'onboarding' CHECK (kind IN ('onboarding','compliance','skills')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE INDEX learning_paths_tenant_idx ON learning_paths (tenant_id);

CREATE TABLE path_items (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  path_id      uuid NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
  position     int  NOT NULL,
  course_id    uuid NOT NULL REFERENCES courses(id),
  is_required  boolean NOT NULL DEFAULT true,
  UNIQUE (path_id, position)
);
CREATE INDEX path_items_tenant_idx ON path_items (tenant_id);

CREATE TABLE assignment_rules (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  name        text NOT NULL,
  path_id     uuid NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
  priority    int  NOT NULL DEFAULT 100,
  is_active   boolean NOT NULL DEFAULT true,
  -- conditions is a simple AND list:
  --   {"and": [{"field":"department_id","op":"eq","value":"uuid"}, ...]}
  -- Supported fields: department_id, manager_id, hired_within_days, status, role_key
  -- Supported ops:    eq, neq, lt, lte, gt, gte
  conditions  jsonb NOT NULL DEFAULT '{"and":[]}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assignment_rules_tenant_active_idx ON assignment_rules (tenant_id) WHERE is_active = true;

CREATE TABLE onboarding_templates (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  key         text NOT NULL,
  title       text NOT NULL,
  description text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE INDEX onboarding_templates_tenant_idx ON onboarding_templates (tenant_id);

CREATE TABLE onboarding_items (
  id                        uuid PRIMARY KEY,
  tenant_id                 uuid NOT NULL,
  template_id               uuid NOT NULL REFERENCES onboarding_templates(id) ON DELETE CASCADE,
  position                  int  NOT NULL,
  title                     text NOT NULL,
  description               text NOT NULL DEFAULT '',
  requires_manager_approval boolean NOT NULL DEFAULT false,
  UNIQUE (template_id, position)
);
CREATE INDEX onboarding_items_tenant_idx ON onboarding_items (tenant_id);

CREATE TABLE onboarding_instances (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  template_id  uuid NOT NULL REFERENCES onboarding_templates(id),
  started_at   timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  UNIQUE (user_id, template_id)
);
CREATE INDEX onboarding_instances_tenant_idx ON onboarding_instances (tenant_id, user_id);

CREATE TABLE onboarding_item_progress (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  instance_id       uuid NOT NULL REFERENCES onboarding_instances(id) ON DELETE CASCADE,
  template_item_id  uuid NOT NULL REFERENCES onboarding_items(id),
  status            text NOT NULL DEFAULT 'not_started'
                    CHECK (status IN ('not_started','in_progress','awaiting_approval','completed')),
  completed_at      timestamptz,
  approved_by       uuid REFERENCES users(id),
  approved_at       timestamptz,
  UNIQUE (instance_id, template_item_id)
);
CREATE INDEX onboarding_item_progress_tenant_idx ON onboarding_item_progress (tenant_id, instance_id);

-- ---------- Grants ----------
GRANT SELECT, INSERT, UPDATE, DELETE ON
  learning_paths, path_items, assignment_rules,
  onboarding_templates, onboarding_items,
  onboarding_instances, onboarding_item_progress
  TO app_rw;

-- ---------- RLS ----------
ALTER TABLE learning_paths            ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning_paths            FORCE  ROW LEVEL SECURITY;
ALTER TABLE path_items                ENABLE ROW LEVEL SECURITY;
ALTER TABLE path_items                FORCE  ROW LEVEL SECURITY;
ALTER TABLE assignment_rules          ENABLE ROW LEVEL SECURITY;
ALTER TABLE assignment_rules          FORCE  ROW LEVEL SECURITY;
ALTER TABLE onboarding_templates      ENABLE ROW LEVEL SECURITY;
ALTER TABLE onboarding_templates      FORCE  ROW LEVEL SECURITY;
ALTER TABLE onboarding_items          ENABLE ROW LEVEL SECURITY;
ALTER TABLE onboarding_items          FORCE  ROW LEVEL SECURITY;
ALTER TABLE onboarding_instances      ENABLE ROW LEVEL SECURITY;
ALTER TABLE onboarding_instances      FORCE  ROW LEVEL SECURITY;
ALTER TABLE onboarding_item_progress  ENABLE ROW LEVEL SECURITY;
ALTER TABLE onboarding_item_progress  FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_learning_paths ON learning_paths
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_path_items ON path_items
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_assignment_rules ON assignment_rules
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_onboarding_templates ON onboarding_templates
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_onboarding_items ON onboarding_items
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_onboarding_instances ON onboarding_instances
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_onboarding_item_progress ON onboarding_item_progress
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE onboarding_item_progress, onboarding_instances, onboarding_items, onboarding_templates,
           assignment_rules, path_items, learning_paths CASCADE;