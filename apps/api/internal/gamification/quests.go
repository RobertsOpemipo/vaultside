package gamification

import (
    "context"
    "log"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/db"
)

// DailyExpiry returns the timestamp at which today's daily quests expire.
// We roll over at 00:00 UTC.
func DailyExpiry(now time.Time) time.Time {
    t := now.UTC()
    return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
}

// WeeklyExpiry returns the timestamp at which this week's weekly quests expire.
// Weeks reset Monday 00:00 UTC.
func WeeklyExpiry(now time.Time) time.Time {
    t := now.UTC()
    // Weekday() : Sunday = 0, Monday = 1, ...
    daysUntilMonday := (8 - int(t.Weekday())) % 7
    if daysUntilMonday == 0 {
        daysUntilMonday = 7
    }
    next := t.AddDate(0, 0, daysUntilMonday)
    return time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
}

// EnsureQuests makes sure the user has rows in user_quests for every active
// quest template whose cadence needs generation. Returns the number of new
// rows created.
//
// Generation rules:
//   - daily: one row per user per calendar day (expires at next UTC midnight)
//   - weekly: one row per user per week (expires next Monday UTC midnight)
//   - onboarding: one row per user, no expiry (assigned once, ever)
func EnsureQuests(
    ctx context.Context,
    q *db.Queries,
    tenantID, userID uuid.UUID,
    now time.Time,
) (int, error) {
    templates, err := q.ListQuestTemplates(ctx)
    if err != nil {
        log.Printf("gamification: ListQuestTemplates: %v", err)
        return 0, err
    }

    // Existing rows we might collide with.
    open, err := q.ListUserQuestsForKind(ctx, userID)
    if err != nil {
        log.Printf("gamification: ListUserQuestsForKind: %v", err)
        return 0, err
    }

    // Any user_quests row that is currently open for a given (cadence, kind)
    // means we don't need to create a new one.
    openKey := make(map[string]bool)
    for _, r := range open {
        // We don't have cadence on the row here; key on kind + completed-nil.
        // That's coarse but avoids duplicates.
        openKey[r.Kind] = true
    }

    created := 0
    for _, tpl := range templates {
        var expires pgtype.Timestamptz
        switch tpl.Cadence {
        case "daily":
            expires = pgtype.Timestamptz{Time: DailyExpiry(now), Valid: true}
        case "weekly":
            expires = pgtype.Timestamptz{Time: WeeklyExpiry(now), Valid: true}
        case "onboarding":
            // Onboarding quests never expire in a meaningful sense. Use a
            // far-future sentinel rather than NULL so the
            // (user_id, quest_id, expires_at) unique constraint can dedupe
            // repeat inserts; Postgres treats NULL as distinct, which would
            // create a new row on every call.
            expires = pgtype.Timestamptz{
                Time:  time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
                Valid: true,
            }
        default:
            continue
        }

        // Skip if a matching open quest exists. onboarding rows have no expiry,
        // so an existing row means we're done forever.
        skip := false
        for _, r := range open {
            if r.Kind == tpl.Kind {
                // For daily/weekly we still want to allow a new row once the
                // previous one completed or expired; but we can't tell from
                // here without cadence. Conservative: skip.
                skip = true
                break
            }
        }
        if skip {
            continue
        }

        _, err := q.InsertUserQuest(ctx, db.InsertUserQuestParams{
            ID:        uuid.Must(uuid.NewV7()),
            TenantID:  tenantID,
            UserID:    userID,
            QuestID:   tpl.ID,
            ExpiresAt: expires,
        })
        if err != nil {
            log.Printf("gamification: InsertUserQuest %s: %v", tpl.Key, err)
            continue
        }
        created++
    }
    return created, nil
}

// ProgressQuestsForKind increments every open quest of the given kind that
// the user hasn't finished. Returns the list of quest IDs that reached their
// target and were marked complete in this call, plus any XP reward summed.
func ProgressQuestsForKind(
    ctx context.Context,
    q *db.Queries,
    tenantID, userID uuid.UUID,
    kind string,
    delta int32,
) (completedQuestKeys []string, xpRewarded int32, err error) {
    rows, err := q.ListUserQuestsForKind(ctx, userID)
    if err != nil {
        log.Printf("gamification: ListUserQuestsForKind: %v", err)
        return nil, 0, err
    }
    for _, r := range rows {
        if r.Kind != kind {
            continue
        }
        updated, err := q.IncrementUserQuestProgress(ctx, db.IncrementUserQuestProgressParams{
            ID:       r.ID,
            Progress: delta,
        })
        if err != nil {
            log.Printf("gamification: IncrementUserQuestProgress: %v", err)
            continue
        }
        if updated.CompletedAt.Valid {
            // Look up the quest's key and xp_reward so we can award it.
            qKey, reward, kerr := questReward(ctx, q, r.ID)
            if kerr != nil {
                continue
            }
            if reward > 0 {
                _, _ = q.AwardXP(ctx, db.AwardXPParams{
                    TenantID: tenantID,
                    UserID:   userID,
                    Delta:    reward,
                    Reason:   "quest.completed",
                    RefType:  strPtr("quest"),
                    RefID:    pgtype.UUID{Bytes: r.ID, Valid: true},
                })
                xpRewarded += reward
            }
            completedQuestKeys = append(completedQuestKeys, qKey)
        }
    }
    return completedQuestKeys, xpRewarded, nil
}

// questReward fetches (key, xp_reward) for a user_quest by joining quests.
// Small helper so the caller can stay clean.
func questReward(ctx context.Context, q *db.Queries, userQuestID uuid.UUID) (string, int32, error) {
    row, err := q.GetUserQuestReward(ctx, userQuestID)
    if err != nil {
        return "", 0, err
    }
    return row.Key, row.XpReward, nil
}

func strPtr(s string) *string { return &s }