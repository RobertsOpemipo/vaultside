package handlers

import (
    "log"
    "net/http"
    "time"

    "github.com/google/uuid"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/gamification"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type xpSummary struct {
    Level          int   `json:"level"`
    TotalXP        int64 `json:"total_xp"`
    XPIntoLevel    int64 `json:"xp_into_level"`
    XPForNextLevel int64 `json:"xp_for_next_level"`
    ProgressPct    int   `json:"progress_pct"`
}

func MyXP(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    total, err := q.SumXPForUser(r.Context(), claims.UserID)
    if err != nil {
        log.Printf("me_xp: sum: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "xp_failed", "")
        return
    }

    p := gamification.ProgressFor(total)
    httpx.JSON(w, http.StatusOK, xpSummary{
        Level:          p.Level,
        TotalXP:        p.TotalXP,
        XPIntoLevel:    p.XPIntoLevel,
        XPForNextLevel: p.XPForNextLevel,
        ProgressPct:    p.ProgressPct,
    })
}

type xpLedgerItem struct {
    ID        int64  `json:"id"`
    Delta     int32  `json:"delta"`
    Reason    string `json:"reason"`
    RefType   string `json:"ref_type,omitempty"`
    RefID     string `json:"ref_id,omitempty"`
    CreatedAt string `json:"created_at"`
}

func MyXPLedger(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    rows, err := q.ListXPLedgerForUser(r.Context(), db.ListXPLedgerForUserParams{
        UserID: claims.UserID,
        Limit:  20,
        Offset: 0,
    })
    if err != nil {
        log.Printf("me_xp: ledger: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "ledger_failed", "")
        return
    }

    out := make([]xpLedgerItem, 0, len(rows))
    for _, row := range rows {
        item := xpLedgerItem{
            ID:        row.ID,
            Delta:     row.Delta,
            Reason:    row.Reason,
            CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
        }
        if row.RefType != nil {
            item.RefType = *row.RefType
        }
        if row.RefID.Valid {
            item.RefID = uuid.UUID(row.RefID.Bytes).String()
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type streakResponse struct {
    CurrentDays int32  `json:"current_days"`
    LongestDays int32  `json:"longest_days"`
    LastDay     string `json:"last_activity_on,omitempty"`
}

func MyStreak(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    s, err := q.GetStreak(r.Context(), db.GetStreakParams{
        TenantID: claims.TenantID,
        UserID:   claims.UserID,
    })
    if err != nil {
        httpx.JSON(w, http.StatusOK, streakResponse{})
        return
    }
    resp := streakResponse{
        CurrentDays: s.CurrentDays,
        LongestDays: s.LongestDays,
    }
    if s.LastActivityOn.Valid {
        resp.LastDay = s.LastActivityOn.Time.Format("2006-01-02")
    }
    httpx.JSON(w, http.StatusOK, resp)
}