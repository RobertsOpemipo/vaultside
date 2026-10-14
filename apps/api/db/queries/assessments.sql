-- name: GetAssessment :one
SELECT id, tenant_id, version_id, title, pass_mark, max_attempts, time_limit_s, shuffle, pool_size
FROM assessments
WHERE id = $1;

-- name: GetAssessmentForVersion :one
SELECT id, tenant_id, version_id, title, pass_mark, max_attempts, time_limit_s, shuffle, pool_size
FROM assessments
WHERE version_id = $1
ORDER BY created_at
LIMIT 1;

-- name: CreateAssessment :one
INSERT INTO assessments (id, tenant_id, version_id, title, pass_mark, max_attempts, time_limit_s, shuffle, pool_size)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, title, pass_mark, max_attempts;

-- name: ListQuestions :many
SELECT id, assessment_id, position, kind, prompt_md, explanation_md, points
FROM questions
WHERE assessment_id = $1
ORDER BY position;

-- name: GetQuestion :one
SELECT id, tenant_id, assessment_id, position, kind, prompt_md, explanation_md, points
FROM questions
WHERE id = $1;

-- name: CreateQuestion :one
INSERT INTO questions (id, tenant_id, assessment_id, position, kind, prompt_md, explanation_md, points)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, position;

-- name: ListOptions :many
SELECT id, question_id, position, text_md, is_correct, feedback_md
FROM question_options
WHERE question_id = $1
ORDER BY position;

-- name: CreateOption :one
INSERT INTO question_options (id, tenant_id, question_id, position, text_md, is_correct, feedback_md)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, position;