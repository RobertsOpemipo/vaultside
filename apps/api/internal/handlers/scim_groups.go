package handlers

import (
    "encoding/json"
    "errors"
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

const scimGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"

type scimGroupMember struct {
    Value   string `json:"value"`
    Display string `json:"display,omitempty"`
    Ref     string `json:"$ref,omitempty"`
    Type    string `json:"type,omitempty"`
}

type scimGroupMeta struct {
    ResourceType string `json:"resourceType"`
    Created      string `json:"created,omitempty"`
    LastModified string `json:"lastModified,omitempty"`
    Location     string `json:"location,omitempty"`
}

type scimGroupResource struct {
    Schemas     []string          `json:"schemas"`
    ID          string            `json:"id"`
    ExternalID  string            `json:"externalId,omitempty"`
    DisplayName string            `json:"displayName"`
    Members     []scimGroupMember `json:"members"`
    Meta        scimGroupMeta     `json:"meta"`
}

func groupToSCIM(teamID uuid.UUID, name string, externalID *string, createdAt time.Time, members []scimGroupMember, baseURL string) scimGroupResource {
    res := scimGroupResource{
        Schemas:     []string{scimGroupSchema},
        ID:          teamID.String(),
        DisplayName: name,
        Members:     members,
        Meta: scimGroupMeta{
            ResourceType: "Group",
            Created:      createdAt.UTC().Format(time.RFC3339),
            LastModified: createdAt.UTC().Format(time.RFC3339),
            Location:     baseURL + "/Groups/" + teamID.String(),
        },
    }
    if externalID != nil {
        res.ExternalID = *externalID
    }
    return res
}

func groupMembersToSCIM(rows []db.ListTeamMembersRow, baseURL string) []scimGroupMember {
    out := make([]scimGroupMember, 0, len(rows))
    for _, m := range rows {
        out = append(out, scimGroupMember{
            Value:   m.UserID.String(),
            Display: m.FullName,
            Ref:     baseURL + "/Users/" + m.UserID.String(),
            Type:    "User",
        })
    }
    return out
}

// ---------------- List ----------------

func SCIMListGroups(w http.ResponseWriter, r *http.Request) {
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

    filterName, _ := parseSimpleGroupFilter(r.URL.Query().Get("filter"))

    rows, err := q.ListTeams(r.Context())
    if err != nil {
        log.Printf("scim_groups: list: %v", err)
        scimError(w, http.StatusInternalServerError, "list failed")
        return
    }

    resources := make([]scimGroupResource, 0, len(rows))
    for i, t := range rows {
        if i < startIndex-1 || len(resources) >= count {
            continue
        }
        if filterName != "" && !strings.EqualFold(t.Name, filterName) {
            continue
        }
        memberRows, _ := q.ListTeamMembers(r.Context(), t.ID)
        resources = append(resources, groupToSCIM(t.ID, t.Name, t.ExternalID, t.CreatedAt,
            groupMembersToSCIM(memberRows, base), base))
    }

    scimJSON(w, http.StatusOK, scimListResponse{
        Schemas:      []string{scimListSchema},
        TotalResults: len(rows),
        StartIndex:   startIndex,
        ItemsPerPage: len(resources),
        Resources:    resources,
    })
}

// ---------------- Get ----------------

func SCIMGetGroup(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        scimError(w, http.StatusBadRequest, "invalid group id")
        return
    }
    team, err := q.GetTeam(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            scimError(w, http.StatusNotFound, "group not found")
            return
        }
        log.Printf("scim_groups: get: %v", err)
        scimError(w, http.StatusInternalServerError, "get failed")
        return
    }
    memberRows, _ := q.ListTeamMembers(r.Context(), team.ID)
    scimJSON(w, http.StatusOK, groupToSCIM(team.ID, team.Name, team.ExternalID, team.CreatedAt,
        groupMembersToSCIM(memberRows, base), base))
}

// ---------------- Create ----------------

type scimCreateGroupRequest struct {
    Schemas     []string          `json:"schemas"`
    ExternalID  string            `json:"externalId,omitempty"`
    DisplayName string            `json:"displayName"`
    Members     []scimGroupMember `json:"members,omitempty"`
}

func SCIMCreateGroup(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)

    var req scimCreateGroupRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        scimError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }
    name := strings.TrimSpace(req.DisplayName)
    if name == "" {
        scimError(w, http.StatusBadRequest, "displayName required")
        return
    }

    // Idempotency: if externalId already maps to a team, return it.
    if req.ExternalID != "" {
        if existing, err := q.GetTeamByExternalID(r.Context(), &req.ExternalID); err == nil {
            memberRows, _ := q.ListTeamMembers(r.Context(), existing.ID)
            scimJSON(w, http.StatusOK, groupToSCIM(existing.ID, existing.Name, existing.ExternalID,
                existing.CreatedAt, groupMembersToSCIM(memberRows, base), base))
            return
        }
    }

    var extID *string
    if req.ExternalID != "" {
        extID = &req.ExternalID
    }
    teamID := uuid.Must(uuid.NewV7())
    _, err := q.CreateTeam(r.Context(), db.CreateTeamParams{
        ID:           teamID,
        TenantID:     claims.TenantID,
        Name:         name,
        DepartmentID: pgtype.UUID{},
        ExternalID:   extID,
    })
    if err != nil {
        log.Printf("scim_groups: create: %v", err)
        scimError(w, http.StatusInternalServerError, "create failed")
        return
    }

    // Add members
    for _, m := range req.Members {
        uid, err := uuid.Parse(m.Value)
        if err != nil {
            continue
        }
        _ = q.AddTeamMember(r.Context(), db.AddTeamMemberParams{
            TeamID: teamID,
            UserID: uid,
            Role:   "member",
        })
    }

    team, _ := q.GetTeam(r.Context(), teamID)
    memberRows, _ := q.ListTeamMembers(r.Context(), teamID)
    scimJSON(w, http.StatusCreated, groupToSCIM(teamID, team.Name, team.ExternalID, team.CreatedAt,
        groupMembersToSCIM(memberRows, base), base))
}

// ---------------- Patch ----------------

func SCIMPatchGroup(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    base := scimBaseURL(r)

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        scimError(w, http.StatusBadRequest, "invalid group id")
        return
    }
    team, err := q.GetTeam(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            scimError(w, http.StatusNotFound, "group not found")
            return
        }
        scimError(w, http.StatusInternalServerError, "get failed")
        return
    }

    var req scimPatchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        scimError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }

    newName := team.Name
    newExt := team.ExternalID

    for _, op := range req.Operations {
        opName := strings.ToLower(op.Op)
        path := strings.ToLower(op.Path)

        switch {
        case path == "displayname":
            if s, ok := op.Value.(string); ok && s != "" {
                newName = s
            }
        case path == "externalid":
            if s, ok := op.Value.(string); ok {
                newExt = &s
            }
        case path == "members" || strings.HasPrefix(path, "members"):
            // Support replace/add/remove on the full members array.
            switch opName {
            case "replace":
                _ = q.ClearTeamMembers(r.Context(), id)
                addMembers(r, q, id, op.Value)
            case "add":
                addMembers(r, q, id, op.Value)
            case "remove":
                removeMembers(r, q, id, op.Value)
            }
        case path == "" && opName == "replace":
            if m, ok := op.Value.(map[string]interface{}); ok {
                if v, ok := m["displayName"].(string); ok && v != "" {
                    newName = v
                }
                if v, ok := m["externalId"].(string); ok {
                    newExt = &v
                }
                if mv, ok := m["members"]; ok {
                    _ = q.ClearTeamMembers(r.Context(), id)
                    addMembers(r, q, id, mv)
                }
            }
        default:
            scimError(w, http.StatusBadRequest, "unsupported path: "+op.Path)
            return
        }
    }

    if err := q.UpdateTeamName(r.Context(), db.UpdateTeamNameParams{
        ID:         id,
        Name:       newName,
        ExternalID: newExt,
    }); err != nil {
        log.Printf("scim_groups: patch: %v", err)
        scimError(w, http.StatusInternalServerError, "patch failed")
        return
    }

    updated, _ := q.GetTeam(r.Context(), id)
    memberRows, _ := q.ListTeamMembers(r.Context(), id)
    scimJSON(w, http.StatusOK, groupToSCIM(updated.ID, updated.Name, updated.ExternalID, updated.CreatedAt,
        groupMembersToSCIM(memberRows, base), base))
}

func addMembers(r *http.Request, q *db.Queries, teamID uuid.UUID, val interface{}) {
    members := scimMembersFromValue(val)
    for _, m := range members {
        uid, err := uuid.Parse(m.Value)
        if err != nil {
            continue
        }
        _ = q.AddTeamMember(r.Context(), db.AddTeamMemberParams{
            TeamID: teamID,
            UserID: uid,
            Role:   "member",
        })
    }
}

func removeMembers(r *http.Request, q *db.Queries, teamID uuid.UUID, val interface{}) {
    members := scimMembersFromValue(val)
    for _, m := range members {
        uid, err := uuid.Parse(m.Value)
        if err != nil {
            continue
        }
        _ = q.RemoveTeamMember(r.Context(), db.RemoveTeamMemberParams{
            TeamID: teamID,
            UserID: uid,
        })
    }
}

// scimMembersFromValue normalises an SCIM value into a slice of members.
// Handles both []interface{} of maps (from JSON) and a single map.
func scimMembersFromValue(val interface{}) []scimGroupMember {
    out := []scimGroupMember{}
    switch v := val.(type) {
    case []interface{}:
        for _, item := range v {
            if m, ok := item.(map[string]interface{}); ok {
                if s, ok := m["value"].(string); ok {
                    display, _ := m["display"].(string)
                    out = append(out, scimGroupMember{Value: s, Display: display})
                }
            }
        }
    case []map[string]interface{}:
        for _, m := range v {
            if s, ok := m["value"].(string); ok {
                display, _ := m["display"].(string)
                out = append(out, scimGroupMember{Value: s, Display: display})
            }
        }
    case map[string]interface{}:
        if s, ok := v["value"].(string); ok {
            display, _ := v["display"].(string)
            out = append(out, scimGroupMember{Value: s, Display: display})
        }
    }
    return out
}

// ---------------- Delete ----------------

func SCIMDeleteGroup(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        scimError(w, http.StatusBadRequest, "invalid group id")
        return
    }
    _ = q.ClearTeamMembers(r.Context(), id)
    if err := q.DeleteTeam(r.Context(), id); err != nil {
        log.Printf("scim_groups: delete: %v", err)
        scimError(w, http.StatusInternalServerError, "delete failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// ---------------- filter helper ----------------

func parseSimpleGroupFilter(s string) (name string, _ *string) {
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
    if field == "displayname" || field == "externalid" {
        return val, nil
    }
    return "", nil
}