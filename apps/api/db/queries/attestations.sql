-- name: ListAttestationsByUser :many
SELECT a.id, a.regulation_id, a.regulation_version, a.text_hash, a.signed_at,
       r.key AS regulation_key, r.name AS regulation_name
FROM attestations a
JOIN regulations r ON r.id = a.regulation_id
WHERE a.user_id = $1
ORDER BY a.signed_at DESC;

-- name: CreateAttestation :one
INSERT INTO attestations (id, tenant_id, user_id, regulation_id, regulation_version, text_hash, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, signed_at;

-- name: CountAttestationsForUserRegulation :one
SELECT count(*) FROM attestations
WHERE user_id = $1 AND regulation_id = $2 AND regulation_version = $3;