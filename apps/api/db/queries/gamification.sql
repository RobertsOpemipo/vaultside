-- name: AwardXP :one
INSERT INTO xp_ledger (tenant_id, user_id, delta, reason, ref_type, ref_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING xp_ledger.id, xp_ledger.created_at;

-- name: SumXPForUser :one
SELECT COALESCE(SUM(delta), 0)::bigint AS total
FROM xp_ledger WHERE user_id = $1;

-- name: ListXPLedgerForUser :many
SELECT id, delta, reason, ref_type, ref_id, created_at
FROM xp_ledger
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListBadges :many
SELECT id, key, name, description, category, points_value, theme_meta
FROM badges
ORDER BY category, name;

-- name: ListUserBadges :many
SELECT ub.id, ub.awarded_at, b.key, b.name, b.description, b.category, b.points_value, b.theme_meta
FROM user_badges ub
JOIN badges b ON b.id = ub.badge_id
WHERE ub.user_id = $1
ORDER BY ub.awarded_at DESC;

-- name: AwardBadge :one
INSERT INTO user_badges (id, tenant_id, user_id, badge_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, badge_id) DO NOTHING
RETURNING user_badges.id, user_badges.awarded_at;

-- name: ListBadgesForUserKeys :many
SELECT b.key
FROM user_badges ub
JOIN badges b ON b.id = ub.badge_id
WHERE ub.user_id = $1;

-- name: GetStreak :one
SELECT current_days, longest_days, last_activity_on, freezes_used_this_month, freeze_month
FROM streaks
WHERE tenant_id = $1 AND user_id = $2;

-- name: UpsertStreak :one
INSERT INTO streaks (tenant_id, user_id, current_days, longest_days, last_activity_on, freezes_used_this_month, freeze_month, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (tenant_id, user_id) DO UPDATE
  SET current_days = EXCLUDED.current_days,
      longest_days = GREATEST(streaks.longest_days, EXCLUDED.longest_days),
      last_activity_on = EXCLUDED.last_activity_on,
      freezes_used_this_month = EXCLUDED.freezes_used_this_month,
      freeze_month = EXCLUDED.freeze_month,
      updated_at = now()
RETURNING streaks.current_days, streaks.longest_days, streaks.last_activity_on, streaks.freezes_used_this_month, streaks.freeze_month;

-- name: ListActiveQuestsForUser :many
SELECT uq.id, uq.progress, uq.completed_at, uq.expires_at,
       q.key, q.title, q.description, q.cadence, q.kind, q.target_value, q.xp_reward, q.theme_meta
FROM user_quests uq
JOIN quests q ON q.id = uq.quest_id
WHERE uq.user_id = $1
  AND uq.completed_at IS NULL
  AND (uq.expires_at IS NULL OR uq.expires_at > now())
ORDER BY q.cadence, uq.expires_at NULLS LAST;

-- name: ListCompletedQuestsForUser :many
SELECT uq.id, uq.progress, uq.completed_at, q.key, q.title, q.cadence, q.xp_reward
FROM user_quests uq
JOIN quests q ON q.id = uq.quest_id
WHERE uq.user_id = $1 AND uq.completed_at IS NOT NULL
ORDER BY uq.completed_at DESC
LIMIT $2;

-- name: IncrementUserQuestProgress :one
UPDATE user_quests
SET progress = user_quests.progress + $2,
    completed_at = CASE
      WHEN user_quests.progress + $2 >= (
        SELECT quests.target_value FROM quests WHERE quests.id = user_quests.quest_id
      )
      THEN now()
      ELSE user_quests.completed_at
    END
WHERE user_quests.id = $1
RETURNING user_quests.id, user_quests.progress, user_quests.completed_at;

-- name: InsertUserQuest :one
INSERT INTO user_quests (id, tenant_id, user_id, quest_id, progress, expires_at)
VALUES ($1, $2, $3, $4, 0, $5)
ON CONFLICT (user_id, quest_id, expires_at) DO UPDATE SET progress = user_quests.progress
RETURNING user_quests.id, user_quests.expires_at;

-- name: ListTeamMissionsForUser :many
SELECT tm.id, tm.title, tm.description, tm.target_value, tm.current_value, tm.starts_on, tm.ends_on
FROM team_missions tm
JOIN team_members tmem ON tmem.team_id = tm.team_id
WHERE tmem.user_id = $1
ORDER BY tm.starts_on DESC;

-- name: GetTeamMembershipForUser :many
SELECT team_id FROM team_members WHERE user_id = $1;

-- name: SumXPForUsersInTeam :many
SELECT xl.user_id, COALESCE(SUM(xl.delta), 0)::bigint AS total
FROM xp_ledger xl
JOIN team_members tm ON tm.user_id = xl.user_id
WHERE tm.team_id = $1
GROUP BY xl.user_id
ORDER BY total DESC;

-- name: ListTeamLeaderboard :many
SELECT u.id AS user_id, u.full_name, COALESCE(SUM(xl.delta), 0)::bigint AS total_xp
FROM team_members tm
JOIN users u ON u.id = tm.user_id
LEFT JOIN xp_ledger xl ON xl.user_id = u.id
WHERE tm.team_id = $1
GROUP BY u.id, u.full_name
ORDER BY total_xp DESC, u.full_name;