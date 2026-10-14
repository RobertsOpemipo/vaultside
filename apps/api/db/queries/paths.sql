-- name: ListLearningPaths :many
SELECT id, tenant_id, key, title, description, kind, created_at
FROM learning_paths
ORDER BY title;

-- name: GetLearningPath :one
SELECT id, tenant_id, key, title, description, kind, created_at
FROM learning_paths
WHERE id = $1;

-- name: ListPathItems :many
SELECT pi.id, pi.path_id, pi.position, pi.course_id, pi.is_required,
       c.title AS course_title, c.kind AS course_kind, c.current_version_id
FROM path_items pi
JOIN courses c ON c.id = pi.course_id
WHERE pi.path_id = $1
ORDER BY pi.position;