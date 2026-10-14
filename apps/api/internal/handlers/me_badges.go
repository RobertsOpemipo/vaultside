package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type badgeItem struct {
    Key         string          `json:"key"`
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Category    string          `json:"category"`
    PointsValue int32           `json:"points_value"`
    ThemeMeta   json.RawMessage `json:"theme_meta,omitempty"`
    AwardedAt   string          `json:"awarded_at,omitempty"`
    Earned      bool            `json:"earned"`
}

func MyBadges(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    all, err := q.ListBadges(r.Context())
    if err != nil {
        log.Printf("me_badges: list all: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    owned, err := q.ListUserBadges(r.Context(), claims.UserID)
    if err != nil {
        log.Printf("me_badges: list owned: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    awardedByKey := make(map[string]string, len(owned))
    for _, ub := range owned {
        awardedByKey[ub.Key] = ub.AwardedAt.UTC().Format(time.RFC3339)
    }

    out := make([]badgeItem, 0, len(all))
    for _, b := range all {
        item := badgeItem{
            Key:         b.Key,
            Name:        b.Name,
            Description: b.Description,
            Category:    b.Category,
            PointsValue: b.PointsValue,
        }
        if len(b.ThemeMeta) > 0 {
            item.ThemeMeta = json.RawMessage(b.ThemeMeta)
        }
        if at, ok := awardedByKey[b.Key]; ok {
            item.Earned = true
            item.AwardedAt = at
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"badges": out})
}