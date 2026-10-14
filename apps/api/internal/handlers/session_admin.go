package handlers

import (
    "log"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type sessionListItem struct {
    ID         string `json:"id"`
    IP         string `json:"ip,omitempty"`
    UserAgent  string `json:"user_agent,omitempty"`
    CreatedAt  string `json:"created_at"`
    LastSeenAt string `json:"last_seen_at"`
    ExpiresAt  string `json:"expires_at"`
    Current    bool   `json:"current"`
}

func ListMySessions(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    rows, err := q.ListActiveSessionsForUser(r.Context(), claims.UserID)
    if err != nil {
        log.Printf("session_admin: list: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]sessionListItem, 0, len(rows))
    for _, row := range rows {
        item := sessionListItem{
            ID:         row.ID.String(),
            CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
            LastSeenAt: row.LastSeenAt.UTC().Format(time.RFC3339),
            ExpiresAt:  row.ExpiresAt.UTC().Format(time.RFC3339),
            Current:    row.ID == claims.SessionID,
        }
        if row.Ip != nil {
            item.IP = row.Ip.String()
        }
        if row.UserAgent != nil {
            item.UserAgent = *row.UserAgent
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    if claims.SessionID == uuid.Nil {
        httpx.Error(w, http.StatusBadRequest, "session_required",
            "endpoint requires session-based auth, not dev headers")
        return
    }

    if err := q.RevokeOtherSessions(r.Context(), db.RevokeOtherSessionsParams{
        UserID: claims.UserID,
        KeepID: claims.SessionID,
    }); err != nil {
        log.Printf("session_admin: revoke others: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "revoke_failed", "")
        return
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"revoked": true})
}

func RevokeSessionByID(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid session id")
        return
    }
    if err := q.RevokeSessionByIDForUser(r.Context(), db.RevokeSessionByIDForUserParams{
        SessionID: id,
        UserID:    claims.UserID,
    }); err != nil {
        log.Printf("session_admin: revoke by id: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "revoke_failed", "")
        return
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"revoked": true})
}