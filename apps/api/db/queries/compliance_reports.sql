-- name: ListDepartments :many
SELECT id, name FROM departments ORDER BY name;

-- name: CountDepartmentHeadcount :one
SELECT count(*) FROM users
WHERE department_id = $1 AND deleted_at IS NULL AND status = 'active';

-- name: CountCompliantInDepartmentForRegulation :one
SELECT count(DISTINCT u.id)
FROM users u
JOIN certificates cert ON cert.user_id = u.id
JOIN course_regulations cr ON cr.course_id = cert.course_id
WHERE u.department_id = $1
  AND u.deleted_at IS NULL
  AND u.status = 'active'
  AND cr.regulation_id = $2
  AND cert.revoked_at IS NULL
  AND (cert.expires_at IS NULL OR cert.expires_at > now());

-- name: ListExpiringCertificatesDetailed :many
SELECT c.id, c.user_id, c.verify_code, c.expires_at,
       u.full_name AS user_full_name,
       u.email AS user_email,
       co.title AS course_title,
       d.name AS department_name
FROM certificates c
JOIN users u ON u.id = c.user_id
JOIN courses co ON co.id = c.course_id
LEFT JOIN departments d ON d.id = u.department_id
WHERE c.expires_at IS NOT NULL
  AND c.expires_at BETWEEN now() AND now() + interval '60 days'
  AND c.revoked_at IS NULL
ORDER BY c.expires_at;

-- name: ListUserComplianceSnapshot :many
SELECT u.id AS user_id,
       u.email,
       u.full_name,
       COALESCE(d.name, '') AS department,
       u.hired_on,
       u.status,
       (SELECT count(DISTINCT cert.id)
        FROM certificates cert
        WHERE cert.user_id = u.id
          AND cert.revoked_at IS NULL
          AND (cert.expires_at IS NULL OR cert.expires_at > now())) AS active_certificates,
       (SELECT count(*)
        FROM attestations a
        WHERE a.user_id = u.id) AS total_attestations
FROM users u
LEFT JOIN departments d ON d.id = u.department_id
WHERE u.deleted_at IS NULL
ORDER BY u.full_name;

-- name: ListAttestationsInRange :many
SELECT a.id, a.user_id, u.email AS user_email, u.full_name AS user_full_name,
       a.regulation_id, r.key AS regulation_key, r.name AS regulation_name,
       a.regulation_version, a.text_hash, a.signed_at
FROM attestations a
JOIN users u ON u.id = a.user_id
JOIN regulations r ON r.id = a.regulation_id
WHERE a.signed_at >= $1 AND a.signed_at < $2
ORDER BY a.signed_at;

-- name: ListCertificatesInRange :many
SELECT c.id, c.user_id, u.email AS user_email, u.full_name AS user_full_name,
       c.course_id, co.title AS course_title, co.kind AS course_kind,
       c.verify_code, c.issued_at, c.expires_at, c.revoked_at
FROM certificates c
JOIN users u ON u.id = c.user_id
JOIN courses co ON co.id = c.course_id
WHERE c.issued_at >= $1 AND c.issued_at < $2
ORDER BY c.issued_at;

-- name: ListAttemptsInRange :many
SELECT a.id, a.enrollment_id, a.assessment_id, a.attempt_number, a.status,
       a.score, a.passed, a.started_at, a.submitted_at,
       e.user_id, u.email AS user_email, u.full_name AS user_full_name,
       ass.title AS assessment_title
FROM attempts a
JOIN enrollments e ON e.id = a.enrollment_id
JOIN users u ON u.id = e.user_id
JOIN assessments ass ON ass.id = a.assessment_id
WHERE a.started_at >= $1 AND a.started_at < $2
ORDER BY a.started_at;