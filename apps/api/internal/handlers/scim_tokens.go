package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type scimTokenItem struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    TokenPrefix string `json:"token_prefix"`
    LastUsedAt  string `json:"last_used_at,omitempty"`
    CreatedAt   string `json:"created_at"`
}

func ListSCIMTokens(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListSCIMTokens(r.Context())
    if err != nil {
        log.Printf("scim_tokens: list: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]scimTokenItem, 0, len(rows))
    for _, row := range rows {
        item := scimTokenItem{
            ID:          row.ID.String(),
            Name:        row.Name,
            TokenPrefix: row.TokenPrefix,
            CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
        }
        if row.LastUsedAt.Valid {
            item.LastUsedAt = row.LastUsedAt.Time.UTC().Format(time.RFC3339)
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"tokens": out})
}

type createSCIMTokenRequest struct {
    Name string `json:"name"`
}

type createSCIMTokenResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    TokenPrefix string `json:"token_prefix"`
    Token       string `json:"token"`
    CreatedAt   string `json:"created_at"`
}

func CreateSCIMToken(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    var req createSCIMTokenRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    req.Name = strings.TrimSpace(req.Name)
    if req.Name == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_name", "name is required")
        return
    }

    raw, hash, prefix, err := auth.GenerateSCIMToken()
    if err != nil {
        log.Printf("scim_tokens: generate: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "token_failed", "")
        return
    }

    row, err := q.CreateSCIMToken(r.Context(), db.CreateSCIMTokenParams{
        ID:          uuid.Must(uuid.NewV7()),
        TenantID:    claims.TenantID,
        Name:        req.Name,
        TokenHash:   hash,
        TokenPrefix: prefix,
        CreatedBy:   pgtype.UUID{Bytes: claims.UserID, Valid: true},
    })
    if err != nil {
        log.Printf("scim_tokens: create: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "create_failed", "")
        return
    }

    httpx.JSON(w, http.StatusCreated, createSCIMTokenResponse{
        ID:          row.ID.String(),
        Name:        row.Name,
        TokenPrefix: row.TokenPrefix,
        Token:       raw,
        CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
    })
}

func RevokeSCIMToken(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid token id")
        return
    }

    if err := q.RevokeSCIMToken(r.Context(), id); err != nil {
        log.Printf("scim_tokens: revoke: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "revoke_failed", "")
        return
    }

    httpx.JSON(w, http.StatusOK, map[string]any{"revoked": true})
}