-- name: PatchUser :one
UPDATE users
SET email       = $2,
    full_name   = $3,
    external_id = $4,
    status      = $5,
    updated_at  = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id;