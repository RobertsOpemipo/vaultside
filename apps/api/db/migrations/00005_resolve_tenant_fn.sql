-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resolve_tenant_slug(p_slug text)
RETURNS TABLE (
  id     uuid,
  slug   text,
  name   text,
  theme  text,
  status text
)
LANGUAGE sql
SECURITY DEFINER
STABLE
SET search_path = public
AS $$
  SELECT t.id, t.slug, t.name, t.theme, t.status
  FROM tenants t
  WHERE t.slug = p_slug
    AND t.status = 'active'
  LIMIT 1
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION resolve_tenant_slug(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION resolve_tenant_slug(text) TO app_rw;

-- +goose Down
DROP FUNCTION IF EXISTS resolve_tenant_slug(text);