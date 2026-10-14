-- name: InsertAuditLog :one
INSERT INTO audit_log (tenant_id, actor_id, actor_type, action, entity_type, entity_id, detail, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, created_at;

-- name: ListAuditLog :many
SELECT id, tenant_id, actor_id, actor_type, action, entity_type, entity_id, detail, created_at
FROM audit_log
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;