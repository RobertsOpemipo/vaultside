-- name: CreateEnrollmentIfMissing :execrows
INSERT INTO enrollments (id, tenant_id, user_id, course_id, course_version_id, source)
VALUES ($1, $2, $3, $4, $5, 'rule')
ON CONFLICT (user_id, course_version_id) DO NOTHING;

-- name: CountUserEnrollmentsForCourse :one
SELECT count(*) FROM enrollments
WHERE user_id = $1 AND course_id = $2;

-- name: GetOnboardingInstanceByUserTemplate :one
SELECT id, tenant_id, user_id, template_id, started_at, completed_at
FROM onboarding_instances
WHERE user_id = $1 AND template_id = $2;