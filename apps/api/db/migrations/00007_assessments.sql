-- +goose Up

CREATE TABLE assessments (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  version_id     uuid NOT NULL REFERENCES course_versions(id) ON DELETE CASCADE,
  title          text NOT NULL,
  pass_mark      int  NOT NULL DEFAULT 80 CHECK (pass_mark BETWEEN 0 AND 100),
  max_attempts   int  NOT NULL DEFAULT 3,
  time_limit_s   int,
  shuffle        boolean NOT NULL DEFAULT true,
  pool_size      int,           -- NULL = use all questions
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assessments_tenant_idx  ON assessments (tenant_id);
CREATE INDEX assessments_version_idx ON assessments (version_id);

CREATE TABLE questions (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  assessment_id uuid NOT NULL REFERENCES assessments(id) ON DELETE CASCADE,
  position      int  NOT NULL,
  kind          text NOT NULL DEFAULT 'multiple_choice'
                CHECK (kind IN ('multiple_choice','true_false','scenario')),
  prompt_md     text NOT NULL,
  explanation_md text,
  points        int NOT NULL DEFAULT 1,
  UNIQUE (assessment_id, position)
);
CREATE INDEX questions_tenant_idx ON questions (tenant_id);

CREATE TABLE question_options (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  question_id  uuid NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  position     int  NOT NULL,
  text_md      text NOT NULL,
  is_correct   boolean NOT NULL DEFAULT false,
  feedback_md  text,
  UNIQUE (question_id, position)
);
CREATE INDEX question_options_tenant_idx ON question_options (tenant_id);

GRANT SELECT, INSERT, UPDATE, DELETE
  ON assessments, questions, question_options
  TO app_rw;

ALTER TABLE assessments      ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessments      FORCE  ROW LEVEL SECURITY;
ALTER TABLE questions        ENABLE ROW LEVEL SECURITY;
ALTER TABLE questions        FORCE  ROW LEVEL SECURITY;
ALTER TABLE question_options ENABLE ROW LEVEL SECURITY;
ALTER TABLE question_options FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_assessments ON assessments
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_questions ON questions
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_question_options ON question_options
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- +goose Down
DROP TABLE question_options, questions, assessments CASCADE;