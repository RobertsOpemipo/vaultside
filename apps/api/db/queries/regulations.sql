-- name: ListRegulations :many
SELECT id, tenant_id, key, name, description, authority, version, created_at
FROM regulations
ORDER BY name;

-- name: GetRegulation :one
SELECT id, tenant_id, key, name, description, authority, version, created_at
FROM regulations
WHERE id = $1;

-- name: ListRegulationsForCourse :many
SELECT r.id, r.key, r.name, r.version
FROM regulations r
JOIN course_regulations cr ON cr.regulation_id = r.id
WHERE cr.course_id = $1
ORDER BY r.name;