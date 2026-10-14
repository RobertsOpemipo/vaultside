-- +goose Up

-- SECURITY DEFINER: resolve a SCIM token hash to its tenant before app.tenant_id is known.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resolve_scim_token(p_hash text)
RETURNS TABLE (
  token_id    uuid,
  tenant_id   uuid,
  tenant_slug text
)
LANGUAGE sql
SECURITY DEFINER
STABLE
SET search_path = public
AS $$
  SELECT t.id, t.tenant_id, ten.slug
  FROM scim_tokens t
  JOIN tenants ten ON ten.id = t.tenant_id
  WHERE t.token_hash = p_hash
    AND t.revoked_at IS NULL
    AND ten.status = 'active'
  LIMIT 1
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION resolve_scim_token(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION resolve_scim_token(text) TO app_rw;

-- Best-effort last-used touch; separate because the resolver is STABLE.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION touch_scim_token(p_token_id uuid)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
AS $$
  UPDATE scim_tokens SET last_used_at = now() WHERE id = p_token_id;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION touch_scim_token(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION touch_scim_token(uuid) TO app_rw;

-- +goose Down
DROP FUNCTION IF EXISTS touch_scim_token(uuid);
DROP FUNCTION IF EXISTS resolve_scim_token(text);