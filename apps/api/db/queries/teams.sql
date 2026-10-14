-- name: ListTeams :many
SELECT id, tenant_id, name, department_id, external_id, created_at
FROM teams
ORDER BY name;

-- name: GetTeam :one
SELECT id, tenant_id, name, department_id, external_id, created_at
FROM teams
WHERE id = $1;

-- name: GetTeamByExternalID :one
SELECT id, tenant_id, name, department_id, external_id, created_at
FROM teams
WHERE external_id = $1
LIMIT 1;

-- name: CreateTeam :one
INSERT INTO teams (id, tenant_id, name, department_id, external_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, tenant_id, name, external_id, created_at;

-- name: UpdateTeamName :exec
UPDATE teams SET name = $2, external_id = $3 WHERE id = $1;

-- name: DeleteTeam :exec
DELETE FROM teams WHERE id = $1;

-- name: ListTeamMembers :many
SELECT tm.user_id, tm.role, u.email, u.full_name
FROM team_members tm
JOIN users u ON u.id = tm.user_id
WHERE tm.team_id = $1
ORDER BY u.full_name;

-- name: AddTeamMember :exec
INSERT INTO team_members (team_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (team_id, user_id) DO NOTHING;

-- name: RemoveTeamMember :exec
DELETE FROM team_members WHERE team_id = $1 AND user_id = $2;

-- name: ClearTeamMembers :exec
DELETE FROM team_members WHERE team_id = $1;

-- name: FindUserForSCIM :one
SELECT id, email, full_name
FROM users
WHERE deleted_at IS NULL
  AND (email = $1 OR (external_id IS NOT NULL AND external_id = $2))
LIMIT 1;