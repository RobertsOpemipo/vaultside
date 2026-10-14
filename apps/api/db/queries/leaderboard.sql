-- name: ListManagerLeaderboard :many
-- Direct reports of the given manager plus the manager themselves, ordered
-- by cumulative XP. Opted-out users are excluded from ranking.
SELECT u.id AS user_id, u.full_name, u.leaderboard_optout,
       COALESCE(SUM(xl.delta), 0)::bigint AS total_xp
FROM users u
LEFT JOIN xp_ledger xl ON xl.user_id = u.id
WHERE u.deleted_at IS NULL
  AND u.leaderboard_optout = false
  AND (u.manager_id = $1 OR u.id = $1)
GROUP BY u.id, u.full_name, u.leaderboard_optout
ORDER BY total_xp DESC, u.full_name;

-- name: GetLeaderboardOptout :one
SELECT leaderboard_optout FROM users WHERE id = $1;

-- name: SetLeaderboardOptout :exec
UPDATE users SET leaderboard_optout = $2, updated_at = now() WHERE id = $1;