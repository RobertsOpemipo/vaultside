-- name: ListQuestTemplates :many
SELECT id, key, title, description, cadence, kind, target_value, xp_reward, theme_meta
FROM quests
ORDER BY cadence, key;

-- name: ListUserQuestsByCadence :many
SELECT uq.id, uq.progress, uq.completed_at, uq.expires_at, q.key, q.cadence, q.kind,
       q.target_value, q.xp_reward
FROM user_quests uq
JOIN quests q ON q.id = uq.quest_id
WHERE uq.user_id = $1 AND q.cadence = $2
ORDER BY uq.expires_at DESC
LIMIT 1;

-- name: ListUserQuestsForKind :many
SELECT uq.id, q.kind, uq.progress, q.target_value, uq.completed_at
FROM user_quests uq
JOIN quests q ON q.id = uq.quest_id
WHERE uq.user_id = $1 AND uq.completed_at IS NULL;
-- name: GetUserQuestReward :one
SELECT q.key, q.xp_reward
FROM user_quests uq
JOIN quests q ON q.id = uq.quest_id
WHERE uq.id = $1;