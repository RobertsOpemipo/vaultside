package handlers

import (
    "encoding/json"
    "log"
    "net/http"

	"github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type leaderboardEntry struct {
    Rank     int    `json:"rank"`
    UserID   string `json:"user_id"`
    FullName string `json:"full_name"`
    TotalXP  int64  `json:"total_xp"`
    IsMe     bool   `json:"is_me"`
}

type leaderboardResponse struct {
    Entries []leaderboardEntry `json:"entries"`
    OptOut  bool               `json:"my_optout"`
}

// TeamLeaderboard returns the calling manager's direct reports plus
// themselves, ranked by cumulative XP. Users who opted out are excluded.
// Requires team.view (same as the docket).
func TeamLeaderboard(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    rows, err := q.ListManagerLeaderboard(r.Context(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
    if err != nil {
        log.Printf("leaderboard: list: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }

    out := make([]leaderboardEntry, 0, len(rows))
    for i, row := range rows {
        out = append(out, leaderboardEntry{
            Rank:     i + 1,
            UserID:   row.UserID.String(),
            FullName: row.FullName,
            TotalXP:  row.TotalXp,
            IsMe:     row.UserID == claims.UserID,
        })
    }

    myOptOut := false
    if v, err := q.GetLeaderboardOptout(r.Context(), claims.UserID); err == nil {
        myOptOut = v
    }

    httpx.JSON(w, http.StatusOK, leaderboardResponse{
        Entries: out,
        OptOut:  myOptOut,
    })
}

type optoutRequest struct {
    OptOut bool `json:"opt_out"`
}

// SetMyLeaderboardOptout toggles whether the caller appears on team
// leaderboards.
func SetMyLeaderboardOptout(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    var req optoutRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    if err := q.SetLeaderboardOptout(r.Context(), db.SetLeaderboardOptoutParams{
        ID:                claims.UserID,
        LeaderboardOptout: req.OptOut,
    }); err != nil {
        log.Printf("leaderboard: set optout: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "update_failed", "")
        return
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"opt_out": req.OptOut})
}