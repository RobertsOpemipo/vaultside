-- name: CreateSCIMToken :one
INSERT INTO scim_tokens (id, tenant_id, name, token_hash, token_prefix, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, token_prefix, created_at;

-- name: ListSCIMTokens :many
SELECT id, name, token_prefix, last_used_at, created_at
FROM scim_tokens
WHERE revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeSCIMToken :exec
UPDATE scim_tokens SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;