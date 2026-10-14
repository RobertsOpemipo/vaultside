-- name: ListCourses :many
SELECT c.id, c.title, c.summary, c.kind, c.current_version_id, c.created_at,
       cv.version AS current_version
FROM courses c
LEFT JOIN course_versions cv ON cv.id = c.current_version_id
WHERE c.deleted_at IS NULL
ORDER BY c.title
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: GetCourse :one
SELECT id, tenant_id, title, summary, kind, current_version_id, created_by, created_at, updated_at
FROM courses
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateCourse :one
INSERT INTO courses (id, tenant_id, title, summary, kind, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, title, summary, kind, created_at;

-- name: ListCourseVersions :many
SELECT id, course_id, version, status, authored_by, reviewed_by, published_at, created_at
FROM course_versions
WHERE course_id = $1
ORDER BY version DESC;

-- name: GetCourseVersion :one
SELECT id, tenant_id, course_id, version, status, authored_by, reviewed_by, published_at, created_at
FROM course_versions
WHERE id = $1;

-- name: CreateCourseVersion :one
INSERT INTO course_versions (id, tenant_id, course_id, version, status, authored_by)
VALUES ($1, $2, $3, $4, 'draft', $5)
RETURNING id, version, status, created_at;

-- name: PublishCourseVersion :exec
UPDATE course_versions
SET status = 'published', published_at = now(), reviewed_by = $2
WHERE id = $1 AND status IN ('draft','in_review');

-- name: SetCurrentVersion :exec
UPDATE courses
SET current_version_id = $2, updated_at = now()
WHERE id = $1;

-- name: ListLessons :many
SELECT l.id, l.version_id, l.position, l.title, l.kind, l.content_md, l.asset_id, l.scorm_id, l.duration_s,
       ca.storage_key, ca.mime_type
FROM lessons l
LEFT JOIN content_assets ca ON ca.id = l.asset_id
WHERE l.version_id = $1
ORDER BY l.position;

-- name: GetLesson :one
SELECT id, tenant_id, version_id, position, title, kind, content_md, asset_id, scorm_id, duration_s
FROM lessons
WHERE id = $1;

-- name: CreateLesson :one
INSERT INTO lessons (id, tenant_id, version_id, position, title, kind, content_md, asset_id, duration_s)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, position, title, kind;

-- name: UpdateLessonOrder :exec
UPDATE lessons SET position = $2 WHERE id = $1;

-- name: CreateContentAsset :one
INSERT INTO content_assets (id, tenant_id, kind, storage_key, mime_type, size_bytes, duration_s, uploaded_by, scan_status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
RETURNING id, storage_key, scan_status;

-- name: GetContentAsset :one
SELECT id, tenant_id, kind, storage_key, mime_type, size_bytes, duration_s, scan_status, transcode_status
FROM content_assets
WHERE id = $1;

-- name: MarkAssetClean :exec
UPDATE content_assets SET scan_status = 'clean', transcode_status = $2 WHERE id = $1;