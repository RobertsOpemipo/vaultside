-- name: ListAttemptAnswersDetailed :many
SELECT aa.question_id,
       aa.option_id,
       qo.is_correct,
       q.points
FROM attempt_answers aa
JOIN questions q ON q.id = aa.question_id
LEFT JOIN question_options qo ON qo.id = aa.option_id
WHERE aa.attempt_id = $1;

-- name: CountPassingAttempts :one
SELECT count(*) FROM attempts
WHERE enrollment_id = $1
  AND assessment_id = $2
  AND passed = true;