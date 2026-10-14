-- +goose Up

CREATE TABLE courses (
  id                 uuid PRIMARY KEY,
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  title              text NOT NULL,
  summary            text NOT NULL DEFAULT '',
  kind               text NOT NULL CHECK (kind IN ('onboarding','compliance','skills')),
  current_version_id uuid,
  created_by         uuid REFERENCES users(id),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz
);
CREATE INDEX courses_tenant_idx ON courses (tenant_id) WHERE deleted_at IS NULL;

CREATE TABLE course_versions (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  course_id    uuid NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
  version      int  NOT NULL,
  status       text NOT NULL CHECK (status IN ('draft','in_review','published','retired')),
  authored_by  uuid REFERENCES users(id),
  reviewed_by  uuid REFERENCES users(id),
  published_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (course_id, version),
  -- Immutability of published versions: once published, status can only go
  -- to 'retired'. Enforced in the app; a trigger would be nice but the
  -- application is the only writer.
  CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX course_versions_tenant_idx ON course_versions (tenant_id);
CREATE INDEX course_versions_course_idx ON course_versions (course_id);

ALTER TABLE courses
  ADD CONSTRAINT courses_current_version_fk
  FOREIGN KEY (current_version_id) REFERENCES course_versions(id);

CREATE TABLE content_assets (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  kind          text NOT NULL CHECK (kind IN ('video','pdf','image','audio','other')),
  storage_key   text NOT NULL,
  mime_type     text NOT NULL,
  size_bytes    bigint NOT NULL DEFAULT 0,
  duration_s    int,
  width         int,
  height        int,
  checksum      text,
  uploaded_by   uuid REFERENCES users(id),
  uploaded_at   timestamptz NOT NULL DEFAULT now(),
  scan_status   text NOT NULL DEFAULT 'pending' CHECK (scan_status IN ('pending','clean','infected','failed')),
  transcode_status text NOT NULL DEFAULT 'n/a' CHECK (transcode_status IN ('n/a','pending','ready','failed')),
  UNIQUE (tenant_id, storage_key)
);
CREATE INDEX content_assets_tenant_idx ON content_assets (tenant_id);

CREATE TABLE scorm_packages (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  asset_id      uuid NOT NULL REFERENCES content_assets(id),
  scorm_version text NOT NULL CHECK (scorm_version IN ('1.2','2004-3rd','2004-4th')),
  entry_point   text NOT NULL,
  manifest      jsonb NOT NULL DEFAULT '{}'
);
CREATE INDEX scorm_packages_tenant_idx ON scorm_packages (tenant_id);

CREATE TABLE lessons (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  version_id   uuid NOT NULL REFERENCES course_versions(id) ON DELETE CASCADE,
  position     int  NOT NULL,
  title        text NOT NULL,
  kind         text NOT NULL CHECK (kind IN ('video','pdf','rich_text','scorm','quiz')),
  content_md   text,
  asset_id     uuid REFERENCES content_assets(id),
  scorm_id     uuid REFERENCES scorm_packages(id),
  duration_s   int NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (version_id, position)
);
CREATE INDEX lessons_tenant_idx  ON lessons (tenant_id);
CREATE INDEX lessons_version_idx ON lessons (version_id);

-- ---------- Grants ----------
GRANT SELECT, INSERT, UPDATE, DELETE
  ON courses, course_versions, content_assets, scorm_packages, lessons
  TO app_rw;

-- ---------- RLS ----------
ALTER TABLE courses         ENABLE ROW LEVEL SECURITY;
ALTER TABLE courses         FORCE  ROW LEVEL SECURITY;
ALTER TABLE course_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_versions FORCE  ROW LEVEL SECURITY;
ALTER TABLE content_assets  ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_assets  FORCE  ROW LEVEL SECURITY;
ALTER TABLE scorm_packages  ENABLE ROW LEVEL SECURITY;
ALTER TABLE scorm_packages  FORCE  ROW LEVEL SECURITY;
ALTER TABLE lessons         ENABLE ROW LEVEL SECURITY;
ALTER TABLE lessons         FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_courses ON courses
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_course_versions ON course_versions
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_content_assets ON content_assets
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_scorm_packages ON scorm_packages
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_lessons ON lessons
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
ALTER TABLE courses DROP CONSTRAINT IF EXISTS courses_current_version_fk;
DROP TABLE lessons, scorm_packages, content_assets, course_versions, courses CASCADE;