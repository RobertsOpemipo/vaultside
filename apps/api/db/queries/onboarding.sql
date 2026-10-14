-- name: ListOnboardingTemplates :many
SELECT id, tenant_id, key, title, description, created_at
FROM onboarding_templates
ORDER BY title;

-- name: GetOnboardingTemplate :one
SELECT id, tenant_id, key, title, description, created_at
FROM onboarding_templates
WHERE id = $1;

-- name: ListOnboardingItems :many
SELECT id, template_id, position, title, description, requires_manager_approval
FROM onboarding_items
WHERE template_id = $1
ORDER BY position;

-- name: GetOrCreateOnboardingInstance :one
INSERT INTO onboarding_instances (id, tenant_id, user_id, template_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, template_id) DO UPDATE
  SET user_id = EXCLUDED.user_id
RETURNING id, user_id, template_id, started_at, completed_at;

-- name: GetOnboardingInstance :one
SELECT id, tenant_id, user_id, template_id, started_at, completed_at
FROM onboarding_instances
WHERE id = $1;

-- name: GetOnboardingInstanceForUser :one
SELECT oi.id, oi.user_id, oi.template_id, oi.started_at, oi.completed_at,
       ot.key AS template_key, ot.title AS template_title
FROM onboarding_instances oi
JOIN onboarding_templates ot ON ot.id = oi.template_id
WHERE oi.user_id = $1
LIMIT 1;

-- name: ListOnboardingInstanceProgress :many
SELECT oip.id, oip.template_item_id, oip.status, oip.completed_at,
       oip.approved_by, oip.approved_at,
       oi.position, oi.title, oi.description, oi.requires_manager_approval
FROM onboarding_item_progress oip
JOIN onboarding_items oi ON oi.id = oip.template_item_id
WHERE oip.instance_id = $1
ORDER BY oi.position;

-- name: UpsertOnboardingItemProgress :one
INSERT INTO onboarding_item_progress (id, tenant_id, instance_id, template_item_id, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (instance_id, template_item_id) DO UPDATE
  SET status = EXCLUDED.status,
      completed_at = CASE
        WHEN EXCLUDED.status = 'completed' THEN now()
        ELSE onboarding_item_progress.completed_at
      END
RETURNING id, status, completed_at;

-- name: ApproveOnboardingItem :one
UPDATE onboarding_item_progress
SET status = 'completed',
    approved_by = $2,
    approved_at = now(),
    completed_at = now()
WHERE id = $1 AND status = 'awaiting_approval'
RETURNING id, status, completed_at, approved_at;

-- name: MarkOnboardingInstanceComplete :exec
UPDATE onboarding_instances
SET completed_at = now()
WHERE id = $1 AND completed_at IS NULL;