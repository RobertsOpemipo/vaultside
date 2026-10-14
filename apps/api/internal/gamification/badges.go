package gamification

import (
    "context"
    "log"

    "github.com/google/uuid"

    "github.com/vaultside/vaultside/apps/api/internal/db"
)

// BadgeEvent carries the facts we need to decide which badges to award.
type BadgeEvent struct {
    LessonCount     int64
    CourseCount     int64
    JustHadQuizPass bool
    QuizScore       int32
    CurrentStreak   int32
    TotalXP         int64
}

// BadgeRule is a pure predicate.
type BadgeRule func(BadgeEvent) bool

// badgeRules maps badge key -> predicate. Adding a new badge is one entry
// here plus one row in the seed file.
var badgeRules = map[string]BadgeRule{
    "first_lesson":  func(ev BadgeEvent) bool { return ev.LessonCount >= 1 },
    "three_lessons": func(ev BadgeEvent) bool { return ev.LessonCount >= 3 },
    "first_course":  func(ev BadgeEvent) bool { return ev.CourseCount >= 1 },
    "quiz_ace":      func(ev BadgeEvent) bool { return ev.JustHadQuizPass && ev.QuizScore >= 100 },
    "streak_7":      func(ev BadgeEvent) bool { return ev.CurrentStreak >= 7 },
    "xp_500":        func(ev BadgeEvent) bool { return ev.TotalXP >= 500 },
    "xp_1000":       func(ev BadgeEvent) bool { return ev.TotalXP >= 1000 },
}

// BadgeState gathers the state we can compute without event-specific flags.
func BadgeState(
    ctx context.Context,
    q *db.Queries,
    tenantID, userID uuid.UUID,
) (BadgeEvent, error) {
    var ev BadgeEvent

    lessons, err := q.CountCompletedLessonsForUser(ctx, userID)
    if err == nil {
        ev.LessonCount = lessons
    }
    courses, err := q.CountCompletedCoursesForUser(ctx, userID)
    if err == nil {
        ev.CourseCount = courses
    }
    if s, err := q.GetStreak(ctx, db.GetStreakParams{
        TenantID: tenantID,
        UserID:   userID,
    }); err == nil {
        ev.CurrentStreak = s.CurrentDays
    }
    total, err := q.SumXPForUser(ctx, userID)
    if err == nil {
        ev.TotalXP = total
    }
    return ev, nil
}

// CheckAndAwardBadges evaluates every rule against the event, skips badges the
// user already has, and awards any new ones. Returns the list of badge keys
// awarded in this call.
func CheckAndAwardBadges(
    ctx context.Context,
    q *db.Queries,
    tenantID, userID uuid.UUID,
    ev BadgeEvent,
) ([]string, error) {
    owned, err := q.ListBadgesForUserKeys(ctx, userID)
    if err != nil {
        log.Printf("gamification: ListBadgesForUserKeys: %v", err)
        return nil, err
    }
    ownedSet := make(map[string]bool, len(owned))
    for _, k := range owned {
        ownedSet[k] = true
    }

    all, err := q.ListBadges(ctx)
    if err != nil {
        log.Printf("gamification: ListBadges: %v", err)
        return nil, err
    }

    awarded := []string{}
    for _, b := range all {
        if ownedSet[b.Key] {
            continue
        }
        rule, ok := badgeRules[b.Key]
        if !ok {
            continue
        }
        if !rule(ev) {
            continue
        }
        _, err := q.AwardBadge(ctx, db.AwardBadgeParams{
            ID:       uuid.Must(uuid.NewV7()),
            TenantID: tenantID,
            UserID:   userID,
            BadgeID:  b.ID,
        })
        if err != nil {
            log.Printf("gamification: AwardBadge %s: %v", b.Key, err)
            continue
        }
        awarded = append(awarded, b.Key)
    }
    return awarded, nil
}