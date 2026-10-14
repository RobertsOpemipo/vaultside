-- +goose Up

-- =============================================================
-- Regulations — the abstract policies an org must follow.
-- =============================================================
CREATE TABLE regulations (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key         text NOT NULL,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  authority   text NOT NULL DEFAULT '',
  version     int  NOT NULL DEFAULT 1,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE INDEX regulations_tenant_idx ON regulations (tenant_id);

CREATE TABLE course_regulations (
  tenant_id     uuid NOT NULL,
  course_id     uuid NOT NULL REFERENCES courses(id)     ON DELETE CASCADE,
  regulation_id uuid NOT NULL REFERENCES regulations(id) ON DELETE CASCADE,
  PRIMARY KEY (course_id, regulation_id)
);
CREATE INDEX course_regulations_tenant_idx     ON course_regulations (tenant_id);
CREATE INDEX course_regulations_regulation_idx ON course_regulations (regulation_id);

-- =============================================================
-- Attestations — append-only
-- =============================================================
CREATE TABLE attestations (
  id                 uuid PRIMARY KEY,
  tenant_id          uuid NOT NULL,
  user_id            uuid NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
  regulation_id      uuid NOT NULL REFERENCES regulations(id) ON DELETE CASCADE,
  regulation_version int  NOT NULL,
  text_hash          text NOT NULL,
  signed_at          timestamptz NOT NULL DEFAULT now(),
  ip                 inet,
  user_agent         text
);
CREATE INDEX attestations_tenant_idx     ON attestations (tenant_id);
CREATE INDEX attestations_user_idx       ON attestations (tenant_id, user_id);
CREATE INDEX attestations_regulation_idx ON attestations (tenant_id, regulation_id);

-- =============================================================
-- Certificates — created here; delivery schema in 00008 didn't
-- include this table.
-- =============================================================
CREATE TABLE certificates (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  user_id           uuid NOT NULL REFERENCES users(id)             ON DELETE CASCADE,
  course_id         uuid NOT NULL REFERENCES courses(id)           ON DELETE CASCADE,
  course_version_id uuid NOT NULL REFERENCES course_versions(id),
  issued_at         timestamptz NOT NULL DEFAULT now(),
  expires_at        timestamptz,
  verify_code       text NOT NULL UNIQUE,
  revoked_at        timestamptz,
  revoked_reason    text,
  renewed_from_id   uuid REFERENCES certificates(id),
  pdf_storage_key   text,
  issued_by         uuid REFERENCES users(id)
);
CREATE INDEX certificates_tenant_idx     ON certificates (tenant_id);
CREATE INDEX certificates_user_idx       ON certificates (tenant_id, user_id);
CREATE INDEX certificates_course_idx     ON certificates (tenant_id, course_id);
CREATE INDEX certificates_expiry_idx     ON certificates (tenant_id, expires_at) WHERE revoked_at IS NULL;

-- =============================================================
-- certificate_events — append-only audit trail
-- =============================================================
CREATE TABLE certificate_events (
  id             bigserial PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  certificate_id uuid NOT NULL REFERENCES certificates(id) ON DELETE CASCADE,
  event_type     text NOT NULL CHECK (event_type IN ('issued','renewed','revoked','expired')),
  detail         jsonb NOT NULL DEFAULT '{}',
  actor_id       uuid,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX certificate_events_tenant_idx ON certificate_events (tenant_id, created_at DESC);
CREATE INDEX certificate_events_cert_idx   ON certificate_events (tenant_id, certificate_id);

-- =============================================================
-- Grants
-- =============================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON regulations, course_regulations TO app_rw;
GRANT SELECT, INSERT                 ON attestations        TO app_rw;
GRANT SELECT, INSERT, UPDATE         ON certificates        TO app_rw;
GRANT SELECT, INSERT                 ON certificate_events  TO app_rw;
GRANT USAGE, SELECT ON SEQUENCE certificate_events_id_seq TO app_rw;

-- =============================================================
-- RLS
-- =============================================================
ALTER TABLE regulations        ENABLE ROW LEVEL SECURITY;
ALTER TABLE regulations        FORCE  ROW LEVEL SECURITY;
ALTER TABLE course_regulations ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_regulations FORCE  ROW LEVEL SECURITY;
ALTER TABLE attestations       ENABLE ROW LEVEL SECURITY;
ALTER TABLE attestations       FORCE  ROW LEVEL SECURITY;
ALTER TABLE certificates       ENABLE ROW LEVEL SECURITY;
ALTER TABLE certificates       FORCE  ROW LEVEL SECURITY;
ALTER TABLE certificate_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE certificate_events FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_regulations ON regulations
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_course_regulations ON course_regulations
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_attestations ON attestations
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_certificates ON certificates
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE POLICY tenant_isolation_certificate_events ON certificate_events
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

REVOKE UPDATE, DELETE ON attestations, certificate_events FROM app_rw;

-- +goose Down
DROP TABLE certificate_events, certificates, attestations, course_regulations, regulations CASCADE;