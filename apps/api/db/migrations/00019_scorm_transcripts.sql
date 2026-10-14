-- +goose Up

-- One row per (user, scorm_package, attempt_number). The cmi_data column
-- carries the SCORM 1.2 / 2004 run-time data model. The runtime fetches this
-- when the player loads, updates it on commit, and finalises it on completion.
CREATE TABLE scorm_attempts (
  id                 uuid PRIMARY KEY,
  tenant_id          uuid NOT NULL,
  user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  scorm_package_id   uuid NOT NULL REFERENCES scorm_packages(id) ON DELETE CASCADE,
  enrollment_id      uuid NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
  lesson_id          uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
  attempt_number     int  NOT NULL DEFAULT 1,
  status             text NOT NULL DEFAULT 'in_progress'
                     CHECK (status IN ('in_progress','completed','passed','failed','abandoned')),

  -- SCORM CMI data model. Keys follow the spec (e.g. 'cmi.core.lesson_status'
  -- for 1.2, 'cmi.completion_status' / 'cmi.success_status' for 2004).
  -- Values are always strings.
  cmi_data           jsonb NOT NULL DEFAULT '{}'::jsonb,

  score_raw          numeric(7,2),
  score_min          numeric(7,2),
  score_max          numeric(7,2),
  completion_status  text,
  success_status     text,
  suspend_data       text,
  session_time_s     int,

  started_at         timestamptz NOT NULL DEFAULT now(),
  last_commit_at     timestamptz NOT NULL DEFAULT now(),
  completed_at       timestamptz,

  UNIQUE (user_id, scorm_package_id, attempt_number)
);
CREATE INDEX scorm_attempts_tenant_idx   ON scorm_attempts (tenant_id);
CREATE INDEX scorm_attempts_user_idx     ON scorm_attempts (tenant_id, user_id);
CREATE INDEX scorm_attempts_lesson_idx   ON scorm_attempts (lesson_id);

-- One row per content asset that has been transcribed. Populated by the
-- worker after the asset upload pipeline runs.
CREATE TABLE transcripts (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  asset_id     uuid NOT NULL REFERENCES content_assets(id) ON DELETE CASCADE,
  language     text NOT NULL DEFAULT 'en',
  status       text NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending','transcribing','ready','failed')),
  text         text,
  segments     jsonb NOT NULL DEFAULT '[]'::jsonb,  -- [{start, end, text}, ...]
  summary      text,
  key_points   jsonb NOT NULL DEFAULT '[]'::jsonb,
  provider     text,
  error        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, asset_id, language)
);
CREATE INDEX transcripts_tenant_idx ON transcripts (tenant_id);
CREATE INDEX transcripts_asset_idx  ON transcripts (tenant_id, asset_id);

-- ---------- Grants ----------
GRANT SELECT, INSERT, UPDATE, DELETE ON scorm_attempts TO app_rw;
GRANT SELECT, INSERT, UPDATE, DELETE ON transcripts   TO app_rw;

-- ---------- RLS ----------
ALTER TABLE scorm_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE scorm_attempts FORCE  ROW LEVEL SECURITY;
ALTER TABLE transcripts    ENABLE ROW LEVEL SECURITY;
ALTER TABLE transcripts    FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_scorm_attempts ON scorm_attempts
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_transcripts ON transcripts
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE transcripts, scorm_attempts CASCADE;