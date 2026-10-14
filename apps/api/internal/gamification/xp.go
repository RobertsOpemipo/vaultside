package gamification

import (
    "context"
    "log"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/db"
)

// XP award amounts. Kept here so the plan's numbers live in one place.
const (
    XPLesson    = 10
    XPQuizPass  = 25
    XPPathStep  = 40
    XPFinalPass = 100
    XPPeerTip   = 15
)

// Reasons written into xp_ledger. Also used to look up existing awards so we
// can enforce the "XP once per ref" anti-gaming rule.
const (
    ReasonLesson   = "lesson.completed"
    ReasonQuizPass = "quiz.pass"
    ReasonFinal    = "assessment.final"
    ReasonPathStep = "path.step"
    ReasonPeerTip  = "peer.tip"
)

// AwardOnce writes an XP row only if no row with the same
// (user_id, reason, ref_type, ref_id) already exists.
// Returns (awarded, error). Awarded is false on the idempotent path.
func AwardOnce(
    ctx context.Context,
    q *db.Queries,
    tenantID, userID uuid.UUID,
    reason string,
    refType string,
    refID uuid.UUID,
    amount int32,
) (bool, error) {
    if amount <= 0 {
        return false, nil
    }

    has, err := q.HasXPAwardForRef(ctx, db.HasXPAwardForRefParams{
        UserID:  userID,
        Reason:  reason,
        RefType: &refType,
        RefID:   pgtype.UUID{Bytes: refID, Valid: true},
    })
    if err != nil {
        log.Printf("gamification: HasXPAwardForRef: %v", err)
        return false, err
    }
    if has {
        return false, nil
    }

    _, err = q.AwardXP(ctx, db.AwardXPParams{
        TenantID: tenantID,
        UserID:   userID,
        Delta:    amount,
        Reason:   reason,
        RefType:  &refType,
        RefID:    pgtype.UUID{Bytes: refID, Valid: true},
    })
    if err != nil {
        log.Printf("gamification: AwardXP: %v", err)
        return false, err
    }
    return true, nil
}