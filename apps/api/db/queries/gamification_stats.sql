-- name: CountCompletedLessonsForUser :one
SELECT count(*)::bigint AS n
FROM lesson_progress
WHERE enrollment_id IN (SELECT id FROM enrollments WHERE user_id = $1)
  AND status = 'completed';

-- name: CountCompletedCoursesForUser :one
SELECT count(*)::bigint AS n
FROM enrollments
WHERE user_id = $1 AND status = 'completed';