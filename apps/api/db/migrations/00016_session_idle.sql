-- +goose Up

ALTER TABLE sessions ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX sessions_idle_idx ON sessions (last_seen_at) WHERE revoked_at IS NULL;

-- Signature is changing (adding last_seen_at to the OUT columns), so the old
-- function must be dropped before the new one can be created.
DROP FUNCTION IF EXISTS resolve_session_by_hash(text);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resolve_session_by_hash(p_hash text)
RETURNS TABLE (
  session_id     uuid,
  tenant_id      uuid,
  user_id        uuid,
  expires_at     timestamptz,
  last_seen_at   timestamptz,
  user_email     text,
  user_full_name text
)
LANGUAGE sql
SECURITY DEFINER
STABLE
SET search_path = public
AS $$
  SELECT s.id, s.tenant_id, s.user_id, s.expires_at, s.last_seen_at, u.email, u.full_name
  FROM sessions s
  JOIN users u ON u.id = s.user_id
  WHERE s.token_hash = p_hash
    AND s.revoked_at IS NULL
    AND s.expires_at > now()
    AND u.deleted_at IS NULL
    AND u.status = 'active'
  LIMIT 1
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION resolve_session_by_hash(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION resolve_session_by_hash(text) TO app_rw;

-- Best-effort touch, rate-limited: only updates if the last touch was > 5 min ago.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION touch_session(p_id uuid)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
AS $$
  UPDATE sessions
  SET last_seen_at = now()
  WHERE id = p_id AND last_seen_at < now() - interval '5 minutes';
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION touch_session(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION touch_session(uuid) TO app_rw;

-- +goose Down
DROP FUNCTION IF EXISTS touch_session(uuid);
DROP INDEX IF EXISTS sessions_idle_idx;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_seen_at;

-- Restore the previous signature without last_seen_at.
DROP FUNCTION IF EXISTS resolve_session_by_hash(text);
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resolve_session_by_hash(p_hash text)
RETURNS TABLE (
  session_id     uuid,
  tenant_id      uuid,
  user_id        uuid,
  expires_at     timestamptz,
  user_email     text,
  user_full_name text
)
LANGUAGE sql SECURITY DEFINER STABLE SET search_path = public
AS $$
  SELECT s.id, s.tenant_id, s.user_id, s.expires_at, u.email, u.full_name
  FROM sessions s
  JOIN users u ON u.id = s.user_id
  WHERE s.token_hash = p_hash
    AND s.revoked_at IS NULL
    AND s.expires_at > now()
    AND u.deleted_at IS NULL
    AND u.status = 'active'
  LIMIT 1
$$;
-- +goose StatementEnd
GRANT EXECUTE ON FUNCTION resolve_session_by_hash(text) TO app_rw;