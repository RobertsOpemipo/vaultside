-- name: ListActiveAssignmentRules :many
SELECT id, tenant_id, name, path_id, priority, is_active, conditions,
       onboarding_template_id, created_at
FROM assignment_rules
WHERE is_active = true
ORDER BY priority, name;

-- name: GetAssignmentRule :one
SELECT id, tenant_id, name, path_id, priority, is_active, conditions,
       onboarding_template_id, created_at
FROM assignment_rules
WHERE id = $1;

-- name: ListUsersForMatching :many
SELECT id, tenant_id, email, full_name, department_id, manager_id, hired_on, status
FROM users
WHERE deleted_at IS NULL AND status = 'active';

-- name: ListAllUserRoles :many
SELECT ur.user_id, r.key AS role_key
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id;