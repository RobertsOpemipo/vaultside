package gamification

import (
    "context"
    "errors"
    "log"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/db"
)

// StreakResult is what callers get back after a qualifying action.
type StreakResult struct {
    CurrentDays int32  `json:"current_days"`
    LongestDays int32  `json:"longest_days"`
    Changed     bool   `json:"changed"`
    LastDay     string `json:"last_activity_on,omitempty"`
}

func isWeekend(d time.Time) bool {
    switch d.Weekday() {
    case time.Saturday, time.Sunday:
        return true
    }
    return false
}

// prevWorkingDay returns the most recent weekday strictly before d.
func prevWorkingDay(d time.Time) time.Time {
    prev := d.AddDate(0, 0, -1)
    for isWeekend(prev) {
        prev = prev.AddDate(0, 0, -1)
    }
    return prev
}

func sameDate(a, b time.Time) bool {
    ya, ma, da := a.UTC().Date()
    yb, mb, db2 := b.UTC().Date()
    return ya == yb && ma == mb && da == db2
}

// TouchStreak should be called once per qualifying learning action. It is
// idempotent within a day and treats weekends as neutral: a user who studies
// on a Saturday keeps their streak but does not gain a day.
func TouchStreak(ctx context.Context, q *db.Queries, tenantID, userID uuid.UUID, now time.Time) (StreakResult, error) {
    today := now.UTC()

    if isWeekend(today) {
        return StreakResult{}, nil
    }

    existing, err := q.GetStreak(ctx, db.GetStreakParams{
        TenantID: tenantID,
        UserID:   userID,
    })
    if err != nil && !errors.Is(err, pgx.ErrNoRows) {
        log.Printf("gamification: GetStreak: %v", err)
        return StreakResult{}, err
    }

    if errors.Is(err, pgx.ErrNoRows) {
        // First-ever action.
        row, uerr := q.UpsertStreak(ctx, db.UpsertStreakParams{
            TenantID:             tenantID,
            UserID:               userID,
            CurrentDays:          1,
            LongestDays:          1,
            LastActivityOn:       pgtype.Date{Time: today, Valid: true},
            FreezesUsedThisMonth: 0,
            FreezeMonth:          nil,
        })
        if uerr != nil {
            log.Printf("gamification: UpsertStreak (new): %v", uerr)
            return StreakResult{}, uerr
        }
        return StreakResult{
            CurrentDays: row.CurrentDays,
            LongestDays: row.LongestDays,
            Changed:     true,
            LastDay:     today.Format("2006-01-02"),
        }, nil
    }

    // Already counted today?
    if existing.LastActivityOn.Valid && sameDate(existing.LastActivityOn.Time, today) {
        return StreakResult{
            CurrentDays: existing.CurrentDays,
            LongestDays: existing.LongestDays,
            Changed:     false,
            LastDay:     existing.LastActivityOn.Time.Format("2006-01-02"),
        }, nil
    }

    newCurrent := existing.CurrentDays
    switch {
    case existing.LastActivityOn.Valid && sameDate(existing.LastActivityOn.Time, prevWorkingDay(today)):
        newCurrent++
    case !existing.LastActivityOn.Valid:
        newCurrent = 1
    default:
        // A gap of more than one working day. Freezes land in a later phase.
        newCurrent = 1
    }

    newLongest := existing.LongestDays
    if newCurrent > newLongest {
        newLongest = newCurrent
    }

    row, err := q.UpsertStreak(ctx, db.UpsertStreakParams{
        TenantID:             tenantID,
        UserID:               userID,
        CurrentDays:          newCurrent,
        LongestDays:          newLongest,
        LastActivityOn:       pgtype.Date{Time: today, Valid: true},
        FreezesUsedThisMonth: existing.FreezesUsedThisMonth,
        FreezeMonth:          existing.FreezeMonth,
    })
    if err != nil {
        log.Printf("gamification: UpsertStreak: %v", err)
        return StreakResult{}, err
    }
    return StreakResult{
        CurrentDays: row.CurrentDays,
        LongestDays: row.LongestDays,
        Changed:     newCurrent != existing.CurrentDays,
        LastDay:     today.Format("2006-01-02"),
    }, nil
}