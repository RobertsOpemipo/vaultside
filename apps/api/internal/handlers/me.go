package handlers

import (
    "net/http"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type MeResponse struct {
    UserID      string   `json:"user_id"`
    Email       string   `json:"email"`
    FullName    string   `json:"full_name"`
    TenantID    string   `json:"tenant_id"`
    TenantSlug  string   `json:"tenant_slug"`
    Status      string   `json:"status"`
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
}

func Me(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    u, err := q.GetUser(r.Context(), claims.UserID)
    if err != nil {
        httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
        return
    }
    roles, err := q.ListUserRoles(r.Context(), u.ID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "roles_failed", "")
        return
    }
    perms, err := q.ListUserPermissions(r.Context(), u.ID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "permissions_failed", "")
        return
    }
    if roles == nil {
        roles = []string{}
    }
    if perms == nil {
        perms = []string{}
    }

    httpx.JSON(w, http.StatusOK, MeResponse{
        UserID:      u.ID.String(),
        Email:       u.Email,
        FullName:    u.FullName,
        TenantID:    claims.TenantID.String(),
        TenantSlug:  claims.TenantSlug,
        Status:      u.Status,
        Roles:       roles,
        Permissions: perms,
    })
}

func MePermissions(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())
    perms, err := q.ListUserPermissions(r.Context(), claims.UserID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "permissions_failed", "")
        return
    }
    if perms == nil {
        perms = []string{}
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"permissions": perms})
}