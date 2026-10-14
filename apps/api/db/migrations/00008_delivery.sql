-- +goose Up

CREATE TABLE enrollments (
  id                 uuid PRIMARY KEY,
  tenant_id          uuid NOT NULL,
  user_id            uuid NOT NULL REFERENCES users(id),
  course_id          uuid NOT NULL REFERENCES courses(id),
  course_version_id  uuid NOT NULL REFERENCES course_versions(id),
  source             text NOT NULL CHECK (source IN ('rule','manual','path','scim')),
  status             text NOT NULL DEFAULT 'assigned'
                     CHECK (status IN ('assigned','in_progress','completed','expired','failed')),
  due_on             date,
  assigned_at        timestamptz NOT NULL DEFAULT now(),
  completed_at       timestamptz,
  UNIQUE (user_id, course_version_id)
);
CREATE INDEX enrollments_tenant_idx      ON enrollments (tenant_id);
CREATE INDEX enrollments_user_idx        ON enrollments (tenant_id, user_id);
CREATE INDEX enrollments_due_idx         ON enrollments (tenant_id, due_on) WHERE status IN ('assigned','in_progress');

CREATE TABLE lesson_progress (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  enrollment_id  uuid NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
  lesson_id      uuid NOT NULL REFERENCES lessons(id),
  status         text NOT NULL DEFAULT 'not_started'
                 CHECK (status IN ('not_started','in_progress','completed')),
  seconds_viewed int NOT NULL DEFAULT 0,
  last_position_s int,
  started_at     timestamptz,
  completed_at   timestamptz,
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (enrollment_id, lesson_id)
);
CREATE INDEX lesson_progress_tenant_idx     ON lesson_progress (tenant_id);
CREATE INDEX lesson_progress_enrollment_idx ON lesson_progress (enrollment_id);

CREATE TABLE attempts (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  enrollment_id  uuid NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
  assessment_id  uuid NOT NULL REFERENCES assessments(id),
  attempt_number int  NOT NULL,
  status         text NOT NULL DEFAULT 'in_progress'
                 CHECK (status IN ('in_progress','submitted','graded','abandoned')),
  score          int,
  passed         boolean,
  started_at     timestamptz NOT NULL DEFAULT now(),
  submitted_at   timestamptz,
  UNIQUE (enrollment_id, assessment_id, attempt_number)
);
CREATE INDEX attempts_tenant_idx     ON attempts (tenant_id);
CREATE INDEX attempts_enrollment_idx ON attempts (enrollment_id);

CREATE TABLE attempt_answers (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  attempt_id     uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  question_id    uuid NOT NULL REFERENCES questions(id),
  option_id      uuid REFERENCES question_options(id),
  is_correct     boolean,
  answered_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (attempt_id, question_id)
);
CREATE INDEX attempt_answers_tenant_idx ON attempt_answers (tenant_id);

CREATE TABLE xapi_statements (
  id           bigserial PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  user_id      uuid REFERENCES users(id),
  statement_id uuid NOT NULL UNIQUE,
  verb         text NOT NULL,
  object_id    text NOT NULL,
  raw          jsonb NOT NULL,
  stored_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX xapi_statements_tenant_idx ON xapi_statements (tenant_id, stored_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE
  ON enrollments, lesson_progress, attempts, attempt_answers
  TO app_rw;
GRANT SELECT, INSERT ON xapi_statements TO app_rw;
GRANT USAGE, SELECT ON SEQUENCE xapi_statements_id_seq TO app_rw;

ALTER TABLE enrollments     ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollments     FORCE  ROW LEVEL SECURITY;
ALTER TABLE lesson_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE lesson_progress FORCE  ROW LEVEL SECURITY;
ALTER TABLE attempts        ENABLE ROW LEVEL SECURITY;
ALTER TABLE attempts        FORCE  ROW LEVEL SECURITY;
ALTER TABLE attempt_answers ENABLE ROW LEVEL SECURITY;
ALTER TABLE attempt_answers FORCE  ROW LEVEL SECURITY;
ALTER TABLE xapi_statements ENABLE ROW LEVEL SECURITY;
ALTER TABLE xapi_statements FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_enrollments ON enrollments
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_lesson_progress ON lesson_progress
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_attempts ON attempts
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_attempt_answers ON attempt_answers
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_xapi_statements ON xapi_statements
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- Append-only: xapi statements are never updated or deleted by the app.
REVOKE UPDATE, DELETE ON xapi_statements FROM app_rw;

-- +goose Down
DROP TABLE xapi_statements, attempt_answers, attempts, lesson_progress, enrollments CASCADE;