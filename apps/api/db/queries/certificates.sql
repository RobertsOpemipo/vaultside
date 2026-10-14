-- name: ListCertificatesForUser :many
SELECT id, course_id, course_version_id, issued_at, expires_at, verify_code, revoked_at
FROM certificates
WHERE user_id = $1
ORDER BY issued_at DESC;

-- name: GetCertificateByVerifyCode :one
SELECT c.id, c.user_id, c.course_id, c.issued_at, c.expires_at, c.verify_code, c.revoked_at,
       u.full_name AS user_full_name,
       co.title AS course_title,
       co.kind  AS course_kind
FROM certificates c
JOIN users   u  ON u.id  = c.user_id
JOIN courses co ON co.id = c.course_id
WHERE c.verify_code = $1;

-- name: GetCertificate :one
SELECT id, tenant_id, user_id, course_id, course_version_id, issued_at, expires_at, verify_code, revoked_at
FROM certificates
WHERE id = $1;

-- name: CountActiveCertificatesForUserVersion :one
SELECT count(*) FROM certificates
WHERE user_id = $1 AND course_version_id = $2 AND revoked_at IS NULL;

-- name: CreateCertificate :one
INSERT INTO certificates (id, tenant_id, user_id, course_id, course_version_id, expires_at, verify_code, issued_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, verify_code, issued_at, expires_at;

-- name: CreateCertificateEvent :one
INSERT INTO certificate_events (tenant_id, certificate_id, event_type, detail, actor_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at;

-- name: RevokeCertificate :exec
UPDATE certificates
SET revoked_at = now(), revoked_reason = $2
WHERE id = $1 AND revoked_at IS NULL;

-- name: ListExpiringCertificates :many
SELECT id, tenant_id, user_id, course_id, verify_code, expires_at
FROM certificates
WHERE expires_at IS NOT NULL
  AND expires_at BETWEEN now() AND now() + interval '60 days'
  AND revoked_at IS NULL
ORDER BY expires_at;