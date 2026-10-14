-- +goose Up

-- SECURITY DEFINER function: resolves a session token hash to its tenant and
-- user before app.tenant_id is known. Same pattern as resolve_tenant_slug.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resolve_session_by_hash(p_hash text)
RETURNS TABLE (
  session_id   uuid,
  tenant_id    uuid,
  user_id      uuid,
  expires_at   timestamptz,
  user_email   text,
  user_full_name text
)
LANGUAGE sql
SECURITY DEFINER
STABLE
SET search_path = public
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

REVOKE ALL ON FUNCTION resolve_session_by_hash(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION resolve_session_by_hash(text) TO app_rw;

-- +goose Down
DROP FUNCTION IF EXISTS resolve_session_by_hash(text);