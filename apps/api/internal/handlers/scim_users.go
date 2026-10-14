package handlers

import (
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

const (
    scimContentType = "application/scim+json"
    scimUserSchema  = "urn:ietf:params:scim:schemas:core:2.0:User"
    scimListSchema  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
    scimErrorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
    scimSPSchema    = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
)

func scimJSON(w http.ResponseWriter, status int, body any) {
    w.Header().Set("Content-Type", scimContentType)
    w.WriteHeader(status)
    if body != nil {
        _ = json.NewEncoder(w).Encode(body)
    }
}

func scimError(w http.ResponseWriter, status int, detail string) {
    scimJSON(w, status, map[string]any{
        "schemas": []string{scimErrorSchema},
        "detail":  detail,
        "status":  fmt.Sprintf("%d", status),
    })
}

func SCIMServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
    scheme := "http"
    if r.TLS != nil {
        scheme = "https"
    }
    base := scheme + "://" + r.Host + "/scim/v2"

    scimJSON(w, http.StatusOK, map[string]any{
        "schemas":          []string{scimSPSchema},
        "documentationUri": "https://vaultside.example/docs/scim",
        "patch":            map[string]any{"supported": true},
        "bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
        "filter":           map[string]any{"supported": true, "maxResults": 200},
        "changePassword":   map[string]any{"supported": false},
        "sort":             map[string]any{"supported": false},
        "etag":             map[string]any{"supported": false},
        "authenticationSchemes": []map[string]any{
            {
                "type":        "oauthbearertoken",
                "name":        "OAuth Bearer Token",
                "description": "Authentication scheme using the OAuth Bearer Token Standard",
                "specUri":     "http://www.rfc-editor.org/info/rfc6750",
                "primary":     true,
            },
        },
        "meta": map[string]any{
            "resourceType": "ServiceProviderConfig",
            "location":     base + "/ServiceProviderConfig",
        },
    })
}

type scimUserName struct {
    Formatted  string `json:"formatted,omitempty"`
    GivenName  string `json:"givenName,omitempty"`
    FamilyName string `json:"familyName,omitempty"`
}

type scimUserEmail struct {
    Value   string `json:"value"`
    Primary bool   `json:"primary"`
    Type    string `json:"type,omitempty"`
}

type scimUserMeta struct {
    ResourceType string `json:"resourceType"`
    Created      string `json:"created,omitempty"`
    LastModified string `json:"lastModified,omitempty"`
    Location     string `json:"location,omitempty"`
}

type scimUserResource struct {
    Schemas    []string        `json:"schemas"`
    ID         string          `json:"id"`
    ExternalID string          `json:"externalId,omitempty"`
    UserName   string          `json:"userName"`
    Name       scimUserName    `json:"name"`
    Emails     []scimUserEmail `json:"emails"`
    Active     bool            `json:"active"`
    Meta       scimUserMeta    `json:"meta"`
}

func splitName(full string) (given, family string) {
    full = strings.TrimSpace(full)
    if full == "" {
        return "", ""
    }
    parts := strings.Fields(full)
    if len(parts) == 1 {
        return parts[0], ""
    }
    return parts[0], strings.Join(parts[1:], " ")
}

func userToSCIMFromListRow(u db.ListUsersRow, baseURL string) scimUserResource {
    given, family := splitName(u.FullName)
    res := scimUserResource{
        Schemas:  []string{scimUserSchema},
        ID:       u.ID.String(),
        UserName: u.Email,
        Active:   u.Status == "active",
        Name:     scimUserName{Formatted: u.FullName, GivenName: given, FamilyName: family},
        Emails:   []scimUserEmail{{Value: u.Email, Primary: true, Type: "work"}},
        Meta: scimUserMeta{
            ResourceType: "User",
            Created:      u.CreatedAt.UTC().Format(time.RFC3339),
            LastModified: u.UpdatedAt.UTC().Format(time.RFC3339),
            Location:     baseURL + "/Users/" + u.ID.String(),
        },
    }
    if u.ExternalID != nil {
        res.ExternalID = *u.ExternalID
    }
    return res
}

func userToSCIMFromGetRow(u db.GetUserRow, baseURL string) scimUserResource {
    given, family := splitName(u.FullName)
    res := scimUserResource{
        Schemas:  []string{scimUserSchema},
        ID:       u.ID.String(),
        UserName: u.Email,
        Active:   u.Status == "active",
        Name:     scimUserName{Formatted: u.FullName, GivenName: given, FamilyName: family},
        Emails:   []scimUserEmail{{Value: u.Email, Primary: true, Type: "work"}},
        Meta: scimUserMeta{
            ResourceType: "User",
            Created:      u.CreatedAt.UTC().Format(time.RFC3339),
            LastModified: u.UpdatedAt.UTC().Format(time.RFC3339),
            Location:     baseURL + "/Users/" + u.ID.String(),
        },
    }
    if u.ExternalID != nil {
        res.ExternalID = *u.ExternalID
    }
    return res
}

type scimListResponse struct {
    Schemas      []string `json:"schemas"`
    TotalResults int      `json:"totalResults"`
    StartIndex   int      `json:"startIndex"`
    ItemsPerPage int      `json:"itemsPerPage"`
    Resources    any      `json:"Resources"`
}

func SCIMListUsers(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)

    startIndex := 1
    count := 100
    if v := r.URL.Query().Get("startIndex"); v != "" {
        if n, err := parseIntSafe(v); err == nil && n > 0 {
            startIndex = n
        }
    }
    if v := r.URL.Query().Get("count"); v != "" {
        if n, err := parseIntSafe(v); err == nil && n > 0 && n <= 500 {
            count = n
        }
    }

    filterUser, filterStatus := parseSimpleUserFilter(r.URL.Query().Get("filter"))

    rows, err := q.ListUsers(r.Context(), db.ListUsersParams{
        Limit:        int32(count),
        Offset:       int32(startIndex - 1),
        DepartmentID: pgtype.UUID{},
        Status:       filterStatus,
    })
    if err != nil {
        log.Printf("scim_users: list: %v", err)
        scimError(w, http.StatusInternalServerError, "list failed")
        return
    }

    total, _ := q.CountUsers(r.Context())

    resources := make([]scimUserResource, 0, len(rows))
    for _, u := range rows {
        if filterUser != "" && !strings.EqualFold(u.Email, filterUser) {
            continue
        }
        resources = append(resources, userToSCIMFromListRow(u, base))
    }

    scimJSON(w, http.StatusOK, scimListResponse{
        Schemas:      []string{scimListSchema},
        TotalResults: int(total),
        StartIndex:   startIndex,
        ItemsPerPage: len(resources),
        Resources:    resources,
    })
}

func SCIMGetUser(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        scimError(w, http.StatusBadRequest, "invalid user id")
        return
    }
    u, err := q.GetUser(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            scimError(w, http.StatusNotFound, "user not found")
            return
        }
        log.Printf("scim_users: get: %v", err)
        scimError(w, http.StatusInternalServerError, "get failed")
        return
    }
    scimJSON(w, http.StatusOK, userToSCIMFromGetRow(u, base))
}

type scimCreateUserRequest struct {
    Schemas    []string        `json:"schemas"`
    ExternalID string          `json:"externalId,omitempty"`
    UserName   string          `json:"userName"`
    Name       scimUserName    `json:"name"`
    Emails     []scimUserEmail `json:"emails,omitempty"`
    Active     *bool           `json:"active,omitempty"`
}

func SCIMCreateUser(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)

    var req scimCreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        scimError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }
    email := strings.TrimSpace(req.UserName)
    if email == "" {
        email = primaryEmail(req.Emails)
    }
    if email == "" {
        scimError(w, http.StatusBadRequest, "userName or primary email required")
        return
    }
    fullName := req.Name.Formatted
    if fullName == "" {
        fullName = strings.TrimSpace(req.Name.GivenName + " " + req.Name.FamilyName)
    }
    if fullName == "" {
        fullName = email
    }
    active := true
    if req.Active != nil {
        active = *req.Active
    }
    status := "active"
    if !active {
        status = "deprovisioned"
    }

    var externalID *string
    if req.ExternalID != "" {
        externalID = &req.ExternalID
    }

    created, err := q.CreateUser(r.Context(), db.CreateUserParams{
        ID:           uuid.Must(uuid.NewV7()),
        TenantID:     claims.TenantID,
        Email:        email,
        FullName:     fullName,
        ExternalID:   externalID,
        DepartmentID: pgtype.UUID{},
        ManagerID:    pgtype.UUID{},
        HiredOn:      pgtype.Date{},
        Status:       status,
    })
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            scimError(w, http.StatusConflict, "user with this userName already exists")
            return
        }
        log.Printf("scim_users: create: %v", err)
        scimError(w, http.StatusInternalServerError, "create failed")
        return
    }

    got, err := q.GetUser(r.Context(), created.ID)
    if err != nil {
        scimError(w, http.StatusInternalServerError, "reload failed")
        return
    }
    scimJSON(w, http.StatusCreated, userToSCIMFromGetRow(got, base))
}

type scimPatchOp struct {
    Op    string      `json:"op"`
    Path  string      `json:"path,omitempty"`
    Value interface{} `json:"value,omitempty"`
}

type scimPatchRequest struct {
    Schemas    []string      `json:"schemas"`
    Operations []scimPatchOp `json:"Operations"`
}

func SCIMPatchUser(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        scimError(w, http.StatusBadRequest, "invalid user id")
        return
    }

    var req scimPatchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        scimError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }
    if len(req.Operations) == 0 {
        scimError(w, http.StatusBadRequest, "no Operations provided")
        return
    }

    current, err := q.GetUser(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            scimError(w, http.StatusNotFound, "user not found")
            return
        }
        scimError(w, http.StatusInternalServerError, "get failed")
        return
    }

    newEmail := current.Email
    newName := current.FullName
    newStatus := current.Status
    newExternal := current.ExternalID

    for _, op := range req.Operations {
        opName := strings.ToLower(op.Op)
        if opName != "add" && opName != "replace" && opName != "remove" {
            scimError(w, http.StatusBadRequest, "unsupported op: "+op.Op)
            return
        }
        path := strings.ToLower(op.Path)
        switch {
        case path == "" && opName == "replace":
            if m, ok := op.Value.(map[string]interface{}); ok {
                if v, ok := m["userName"].(string); ok && v != "" {
                    newEmail = v
                }
                if v, ok := m["externalId"].(string); ok {
                    newExternal = &v
                }
                if v, ok := m["active"].(bool); ok {
                    if v {
                        newStatus = "active"
                    } else {
                        newStatus = "deprovisioned"
                    }
                }
                if nm, ok := m["name"].(map[string]interface{}); ok {
                    formatted, _ := nm["formatted"].(string)
                    if formatted == "" {
                        g, _ := nm["givenName"].(string)
                        f, _ := nm["familyName"].(string)
                        formatted = strings.TrimSpace(g + " " + f)
                    }
                    if formatted != "" {
                        newName = formatted
                    }
                }
            }
        case path == "username":
            if s, ok := op.Value.(string); ok && s != "" {
                newEmail = s
            }
        case path == "name.formatted":
            if s, ok := op.Value.(string); ok && s != "" {
                newName = s
            }
        case path == "name.givenname":
            if s, ok := op.Value.(string); ok {
                _, f := splitName(current.FullName)
                newName = strings.TrimSpace(s + " " + f)
            }
        case path == "name.familyname":
            if s, ok := op.Value.(string); ok {
                g, _ := splitName(current.FullName)
                newName = strings.TrimSpace(g + " " + s)
            }
        case path == "active":
            if b, ok := op.Value.(bool); ok {
                if b {
                    newStatus = "active"
                } else {
                    newStatus = "deprovisioned"
                }
            }
        case path == "externalid":
            if s, ok := op.Value.(string); ok {
                newExternal = &s
            }
        default:
            scimError(w, http.StatusBadRequest, "unsupported path: "+op.Path)
            return
        }
    }

    _, err = q.PatchUser(r.Context(), db.PatchUserParams{
        ID:         id,
        Email:      newEmail,
        FullName:   newName,
        ExternalID: newExternal,
        Status:     newStatus,
    })
    if err != nil {
        log.Printf("scim_users: patch: %v", err)
        scimError(w, http.StatusInternalServerError, "patch failed")
        return
    }

    if newStatus != "active" {
        _ = q.RevokeSessionsForUser(r.Context(), id)
    }

    got, err := q.GetUser(r.Context(), id)
    if err != nil {
        scimError(w, http.StatusInternalServerError, "reload failed")
        return
    }
    scimJSON(w, http.StatusOK, userToSCIMFromGetRow(got, base))
}

func SCIMDeleteUser(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        scimError(w, http.StatusBadRequest, "invalid user id")
        return
    }
    if err := q.SoftDeleteUser(r.Context(), id); err != nil {
        log.Printf("scim_users: delete: %v", err)
        scimError(w, http.StatusInternalServerError, "delete failed")
        return
    }
    _ = q.RevokeSessionsForUser(r.Context(), id)
    w.WriteHeader(http.StatusNoContent)
}

func scimBaseURL(r *http.Request) string {
    scheme := "http"
    if r.TLS != nil {
        scheme = "https"
    }
    host := r.Host
    if host == "" {
        host = "localhost"
    }
    return scheme + "://" + host + "/scim/v2"
}

func primaryEmail(emails []scimUserEmail) string {
    for _, e := range emails {
        if e.Primary && e.Value != "" {
            return e.Value
        }
    }
    if len(emails) > 0 {
        return emails[0].Value
    }
    return ""
}

func parseIntSafe(s string) (int, error) {
    var n int
    _, err := fmt.Sscanf(s, "%d", &n)
    return n, err
}

func parseSimpleUserFilter(s string) (userName string, status *string) {
    s = strings.TrimSpace(s)
    if s == "" {
        return "", nil
    }
    parts := strings.Fields(s)
    if len(parts) != 3 {
        return "", nil
    }
    field := strings.ToLower(parts[0])
    op := strings.ToLower(parts[1])
    val := strings.Trim(parts[2], `"`)
    if op != "eq" {
        return "", nil
    }
    switch field {
    case "username", "emails.value":
        return val, nil
    case "active":
        if val == "true" {
            v := "active"
            return "", &v
        }
        if val == "false" {
            v := "deprovisioned"
            return "", &v
        }
    }
    return "", nil
}