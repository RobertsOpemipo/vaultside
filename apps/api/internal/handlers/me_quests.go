package handlers

import (
    "log"
    "net/http"
    "time"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type questItem struct {
    ID          string `json:"id"`
    Key         string `json:"key"`
    Title       string `json:"title"`
    Description string `json:"description,omitempty"`
    Cadence     string `json:"cadence"`
    Kind        string `json:"kind"`
    TargetValue int32  `json:"target_value"`
    Progress    int32  `json:"progress"`
    XPReward    int32  `json:"xp_reward"`
    ExpiresAt   string `json:"expires_at,omitempty"`
    CompletedAt string `json:"completed_at,omitempty"`
}

func MyQuests(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    active, err := q.ListActiveQuestsForUser(r.Context(), claims.UserID)
    if err != nil {
        log.Printf("me_quests: active: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "quests_failed", "")
        return
    }
    completed, err := q.ListCompletedQuestsForUser(r.Context(), db.ListCompletedQuestsForUserParams{
        UserID: claims.UserID,
        Limit:  20,
    })
    if err != nil {
        log.Printf("me_quests: completed: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "quests_failed", "")
        return
    }

    activeOut := make([]questItem, 0, len(active))
    for _, row := range active {
        item := questItem{
            ID:          row.ID.String(),
            Key:         row.Key,
            Title:       row.Title,
            Description: row.Description,
            Cadence:     row.Cadence,
            Kind:        row.Kind,
            TargetValue: row.TargetValue,
            Progress:    row.Progress,
            XPReward:    row.XpReward,
        }
        if row.ExpiresAt.Valid {
            item.ExpiresAt = row.ExpiresAt.Time.UTC().Format(time.RFC3339)
        }
        activeOut = append(activeOut, item)
    }

    completedOut := make([]questItem, 0, len(completed))
    for _, row := range completed {
        item := questItem{
            ID:       row.ID.String(),
            Key:      row.Key,
            Title:    row.Title,
            Cadence:  row.Cadence,
            Progress: row.Progress,
            XPReward: row.XpReward,
        }
        if row.CompletedAt.Valid {
            item.CompletedAt = row.CompletedAt.Time.UTC().Format(time.RFC3339)
        }
        completedOut = append(completedOut, item)
    }

    httpx.JSON(w, http.StatusOK, map[string]any{
        "active":    activeOut,
        "completed": completedOut,
    })
}