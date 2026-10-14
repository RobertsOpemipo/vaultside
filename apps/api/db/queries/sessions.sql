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

-- name: ListActiveSessionsForUser :many
SELECT id, ip, user_agent, created_at, last_seen_at, expires_at
FROM sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > now()
ORDER BY last_seen_at DESC;

-- name: RevokeOtherSessions :exec
UPDATE sessions
SET revoked_at = now()
WHERE user_id = sqlc.arg('user_id')
  AND id <> sqlc.arg('keep_id')
  AND revoked_at IS NULL;

-- name: RevokeSessionByIDForUser :exec
UPDATE sessions
SET revoked_at = now()
WHERE id = sqlc.arg('session_id')
  AND user_id = sqlc.arg('user_id')
  AND revoked_at IS NULL;