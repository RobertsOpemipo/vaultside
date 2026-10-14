-- +goose Up

-- XP ledger: append-only. Every award writes a row.
CREATE TABLE xp_ledger (
  id          bigserial PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  delta       int  NOT NULL,
  reason      text NOT NULL,
  ref_type    text,
  ref_id      uuid,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX xp_ledger_tenant_user_idx    ON xp_ledger (tenant_id, user_id, created_at DESC);
CREATE INDEX xp_ledger_tenant_created_idx ON xp_ledger (tenant_id, created_at DESC);

CREATE TABLE streaks (
  tenant_id               uuid NOT NULL,
  user_id                 uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  current_days            int  NOT NULL DEFAULT 0,
  longest_days            int  NOT NULL DEFAULT 0,
  last_activity_on        date,
  freezes_used_this_month int  NOT NULL DEFAULT 0,
  freeze_month            text,
  updated_at              timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE badges (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  key          text NOT NULL,
  name         text NOT NULL,
  description  text NOT NULL DEFAULT '',
  category     text NOT NULL CHECK (category IN ('milestone','skill','behavior','team')),
  points_value int  NOT NULL DEFAULT 0,
  theme_meta   jsonb NOT NULL DEFAULT '{}',
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE INDEX badges_tenant_idx ON badges (tenant_id);

CREATE TABLE user_badges (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  badge_id    uuid NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
  awarded_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, badge_id)
);
CREATE INDEX user_badges_tenant_user_idx ON user_badges (tenant_id, user_id);

CREATE TABLE quests (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  key           text NOT NULL,
  title         text NOT NULL,
  description   text NOT NULL DEFAULT '',
  cadence       text NOT NULL CHECK (cadence IN ('daily','weekly','onboarding','mission')),
  kind          text NOT NULL CHECK (kind IN ('lesson','quiz_pass','path_step','custom')),
  target_value  int  NOT NULL DEFAULT 1,
  xp_reward     int  NOT NULL DEFAULT 10,
  theme_meta    jsonb NOT NULL DEFAULT '{}',
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key)
);
CREATE INDEX quests_tenant_idx ON quests (tenant_id);

CREATE TABLE user_quests (
  id           uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  quest_id     uuid NOT NULL REFERENCES quests(id) ON DELETE CASCADE,
  progress     int  NOT NULL DEFAULT 0,
  completed_at timestamptz,
  expires_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, quest_id, expires_at)
);
CREATE INDEX user_quests_tenant_user_idx ON user_quests (tenant_id, user_id, completed_at) WHERE completed_at IS NULL;

CREATE TABLE team_missions (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  team_id       uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
  title         text NOT NULL,
  description   text NOT NULL DEFAULT '',
  target_value  int  NOT NULL,
  current_value int  NOT NULL DEFAULT 0,
  starts_on     date NOT NULL DEFAULT current_date,
  ends_on       date,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX team_missions_tenant_team_idx ON team_missions (tenant_id, team_id);

GRANT SELECT, INSERT ON xp_ledger TO app_rw;
GRANT SELECT, INSERT, UPDATE ON streaks, user_badges, user_quests, team_missions TO app_rw;
GRANT SELECT, INSERT, UPDATE, DELETE ON badges, quests TO app_rw;
GRANT USAGE, SELECT ON SEQUENCE xp_ledger_id_seq TO app_rw;

ALTER TABLE xp_ledger     ENABLE ROW LEVEL SECURITY;
ALTER TABLE xp_ledger     FORCE  ROW LEVEL SECURITY;
ALTER TABLE streaks       ENABLE ROW LEVEL SECURITY;
ALTER TABLE streaks       FORCE  ROW LEVEL SECURITY;
ALTER TABLE badges        ENABLE ROW LEVEL SECURITY;
ALTER TABLE badges        FORCE  ROW LEVEL SECURITY;
ALTER TABLE user_badges   ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_badges   FORCE  ROW LEVEL SECURITY;
ALTER TABLE quests        ENABLE ROW LEVEL SECURITY;
ALTER TABLE quests        FORCE  ROW LEVEL SECURITY;
ALTER TABLE user_quests   ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_quests   FORCE  ROW LEVEL SECURITY;
ALTER TABLE team_missions ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_missions FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_xp_ledger ON xp_ledger
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation_streaks ON streaks
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation_badges ON badges
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation_user_badges ON user_badges
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation_quests ON quests
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation_user_quests ON user_quests
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation_team_missions ON team_missions
  USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

REVOKE UPDATE, DELETE ON xp_ledger FROM app_rw;

-- +goose Down
DROP TABLE team_missions, user_quests, quests, user_badges, badges, streaks, xp_ledger CASCADE;