-- name: ListDirectReports :many
SELECT id, tenant_id, email, full_name, department_id, hired_on, status
FROM users
WHERE manager_id = $1
  AND deleted_at IS NULL
ORDER BY full_name;

-- name: CountEnrollmentsByStatusForUser :many
SELECT status, count(*) AS n
FROM enrollments
WHERE user_id = $1
GROUP BY status;

-- name: CountOverdueEnrollmentsForUser :one
SELECT count(*) FROM enrollments
WHERE user_id = $1
  AND status IN ('assigned','in_progress')
  AND due_on IS NOT NULL
  AND due_on < now()::date;

-- name: GetOnboardingSummaryForUser :one
SELECT i.id AS instance_id,
       t.title AS template_title,
       i.started_at,
       i.completed_at,
       (SELECT count(*) FROM onboarding_item_progress oip WHERE oip.instance_id = i.id) AS items_total,
       (SELECT count(*) FROM onboarding_item_progress oip WHERE oip.instance_id = i.id AND oip.status = 'completed') AS items_completed,
       (SELECT count(*) FROM onboarding_item_progress oip WHERE oip.instance_id = i.id AND oip.status = 'awaiting_approval') AS items_awaiting,
       (SELECT count(*) FROM onboarding_item_progress oip
          JOIN onboarding_items oi ON oi.id = oip.template_item_id
          WHERE oip.instance_id = i.id AND oi.requires_manager_approval = true AND oip.status <> 'completed') AS items_pending_manager
FROM onboarding_instances i
JOIN onboarding_templates t ON t.id = i.template_id
WHERE i.user_id = $1
LIMIT 1;