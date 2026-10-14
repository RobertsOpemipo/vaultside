-- name: ResolveTenantBySlug :one
SELECT id, slug, name, theme, status
FROM tenants
WHERE slug = $1 AND status = 'active';

-- name: GetTenant :one
SELECT id, slug, name, theme, status, created_at, updated_at
FROM tenants
WHERE id = $1;

-- name: ListTenants :many
SELECT id, slug, name, theme, status, created_at
FROM tenants
ORDER BY name;