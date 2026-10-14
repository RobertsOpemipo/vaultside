-- name: CreateSession :one
INSERT INTO sessions (id, tenant_id, user_id, token_hash, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, expires_at;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeSessionsForUser :exec
UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now() - interval '7 days';