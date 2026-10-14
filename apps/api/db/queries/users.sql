-- name: ListUsers :many
SELECT id, tenant_id, email, full_name, external_id, department_id, manager_id,
       hired_on, status, last_login_at, created_at, updated_at
FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('department_id')::uuid IS NULL OR department_id = sqlc.narg('department_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY full_name
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountUsers :one
SELECT count(*) FROM users WHERE deleted_at IS NULL;

-- name: GetUser :one
SELECT id, tenant_id, email, full_name, external_id, department_id, manager_id,
       hired_on, status, last_login_at, created_at, updated_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT id, tenant_id, email, full_name, external_id, department_id, manager_id,
       hired_on, status, last_login_at, created_at, updated_at
FROM users
WHERE email = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (id, tenant_id, email, full_name, external_id, department_id, manager_id, hired_on, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, tenant_id, email, full_name, status, created_at;

-- name: SoftDeleteUser :exec
UPDATE users
SET deleted_at = now(), updated_at = now(), status = 'deprovisioned'
WHERE id = $1;

-- name: TouchLastLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: ListUserRoles :many
SELECT r.key AS role_key
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.key;

-- name: ListUserPermissions :many
SELECT DISTINCT rp.permission
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE ur.user_id = $1
ORDER BY rp.permission;

-- name: AssignRole :exec
INSERT INTO user_roles (id, tenant_id, user_id, role_id, scope_type, granted_by)
SELECT $1, $2, $3, r.id, 'tenant', $4
FROM roles r
WHERE r.key = $5
  AND r.tenant_id = $2
ON CONFLICT DO NOTHING;