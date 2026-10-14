-- name: ListMyEnrollments :many
SELECT e.id, e.course_id, e.course_version_id, e.source, e.status, e.due_on,
       e.assigned_at, e.completed_at, c.title, c.kind, c.summary
FROM enrollments e
JOIN courses c ON c.id = e.course_id
WHERE e.user_id = $1
ORDER BY
  CASE e.status WHEN 'assigned' THEN 0 WHEN 'in_progress' THEN 1 ELSE 2 END,
  e.due_on NULLS LAST,
  e.assigned_at DESC;

-- name: GetEnrollment :one
SELECT id, tenant_id, user_id, course_id, course_version_id, source, status, due_on, assigned_at, completed_at
FROM enrollments
WHERE id = $1;

-- name: CreateEnrollment :one
INSERT INTO enrollments (id, tenant_id, user_id, course_id, course_version_id, source, due_on)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, course_version_id) DO UPDATE SET due_on = EXCLUDED.due_on
RETURNING id, status, due_on, assigned_at;

-- name: MarkEnrollmentCompleted :exec
UPDATE enrollments
SET status = 'completed', completed_at = now()
WHERE id = $1 AND status <> 'completed';

-- name: ListLessonProgress :many
SELECT id, lesson_id, status, seconds_viewed, last_position_s, started_at, completed_at
FROM lesson_progress
WHERE enrollment_id = $1;

-- name: UpsertLessonProgress :one
INSERT INTO lesson_progress (id, tenant_id, enrollment_id, lesson_id, status, seconds_viewed, last_position_s, started_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (enrollment_id, lesson_id) DO UPDATE
  SET status = EXCLUDED.status,
      seconds_viewed = GREATEST(lesson_progress.seconds_viewed, EXCLUDED.seconds_viewed),
      last_position_s = EXCLUDED.last_position_s,
      updated_at = now(),
      completed_at = CASE WHEN EXCLUDED.status = 'completed' AND lesson_progress.completed_at IS NULL
                          THEN now() ELSE lesson_progress.completed_at END
RETURNING id, status, seconds_viewed, completed_at;

-- name: CountCompletedLessons :one
SELECT count(*) FROM lesson_progress
WHERE enrollment_id = $1 AND status = 'completed';

-- name: CreateAttempt :one
INSERT INTO attempts (id, tenant_id, enrollment_id, assessment_id, attempt_number)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, attempt_number, status, started_at;

-- name: GetAttempt :one
SELECT id, tenant_id, enrollment_id, assessment_id, attempt_number, status, score, passed, started_at, submitted_at
FROM attempts
WHERE id = $1;

-- name: NextAttemptNumber :one
SELECT COALESCE(MAX(attempt_number), 0) + 1 AS next_number
FROM attempts
WHERE enrollment_id = $1 AND assessment_id = $2;

-- name: SaveAttemptAnswer :exec
INSERT INTO attempt_answers (id, tenant_id, attempt_id, question_id, option_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (attempt_id, question_id) DO UPDATE SET option_id = EXCLUDED.option_id, answered_at = now();

-- name: GetOptionForQuestion :one
SELECT id, is_correct
FROM question_options
WHERE id = $1 AND question_id = $2;

-- name: SubmitAttempt :one
UPDATE attempts
SET status = 'graded',
    score = $2,
    passed = $3,
    submitted_at = now()
WHERE id = $1
RETURNING id, score, passed, submitted_at;