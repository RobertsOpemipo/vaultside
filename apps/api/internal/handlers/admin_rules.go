package handlers

import (
    "errors"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"

    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
    "github.com/vaultside/vaultside/apps/api/internal/rules"
)

// ---------------- List rules ----------------

type ruleSummary struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    PathID        string `json:"path_id"`
    Priority      int32  `json:"priority"`
    IsActive      bool   `json:"is_active"`
    HasOnboarding bool   `json:"has_onboarding"`
}

func ListRules(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListActiveAssignmentRules(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]ruleSummary, 0, len(rows))
    for _, row := range rows {
        out = append(out, ruleSummary{
            ID:            row.ID.String(),
            Name:          row.Name,
            PathID:        row.PathID.String(),
            Priority:      row.Priority,
            IsActive:      row.IsActive,
            HasOnboarding: row.OnboardingTemplateID.Valid,
        })
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"rules": out})
}

// ---------------- Dry-run ----------------

type dryRunUser struct {
    UserID    string `json:"user_id"`
    Email     string `json:"email"`
    FullName  string `json:"full_name"`
    AlreadyIn int64  `json:"already_enrolled_count"`
}

type dryRunResponse struct {
    RuleID       string       `json:"rule_id"`
    RuleName     string       `json:"rule_name"`
    PathID       string       `json:"path_id"`
    TotalMatched int          `json:"total_matched"`
    MatchedUsers []dryRunUser `json:"matched_users"`
}

func DryRunRule(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    ruleID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid rule id")
        return
    }

    rule, err := q.GetAssignmentRule(r.Context(), ruleID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "rule not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    users, err := q.ListUsersForMatching(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "users_failed", "")
        return
    }

    roleRows, err := q.ListAllUserRoles(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "roles_failed", "")
        return
    }
    rolesByUser := make(map[uuid.UUID][]string, len(users))
    for _, rr := range roleRows {
        rolesByUser[rr.UserID] = append(rolesByUser[rr.UserID], rr.RoleKey)
    }

    pathItems, err := q.ListPathItems(r.Context(), rule.PathID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "path_items_failed", "")
        return
    }

    now := time.Now()
    matched := make([]dryRunUser, 0)
    for _, u := range users {
        facts := dbUserToFacts(u, rolesByUser[u.ID])
        ok, err := rules.Match(rule.Conditions, facts, now)
        if err != nil {
            httpx.Error(w, http.StatusInternalServerError, "match_failed", err.Error())
            return
        }
        if !ok {
            continue
        }
        var already int64
        for _, item := range pathItems {
            n, err := q.CountUserEnrollmentsForCourse(r.Context(), db.CountUserEnrollmentsForCourseParams{
                UserID:   u.ID,
                CourseID: item.CourseID,
            })
            if err != nil {
                httpx.Error(w, http.StatusInternalServerError, "count_failed", "")
                return
            }
            already += n
        }
        matched = append(matched, dryRunUser{
            UserID:    u.ID.String(),
            Email:     u.Email,
            FullName:  u.FullName,
            AlreadyIn: already,
        })
    }

    httpx.JSON(w, http.StatusOK, dryRunResponse{
        RuleID:       rule.ID.String(),
        RuleName:     rule.Name,
        PathID:       rule.PathID.String(),
        TotalMatched: len(matched),
        MatchedUsers: matched,
    })
}

// dbUserToFacts adapts a sqlc row to the engine's UserFacts.
func dbUserToFacts(u db.ListUsersForMatchingRow, roles []string) rules.UserFacts {
    f := rules.UserFacts{
        UserID:   u.ID,
        Email:    u.Email,
        FullName: u.FullName,
        Status:   u.Status,
        RoleKeys: roles,
    }
    if u.DepartmentID.Valid {
        f.DepartmentID = uuid.UUID(u.DepartmentID.Bytes)
        f.HasDept = true
    }
    if u.ManagerID.Valid {
        f.ManagerID = uuid.UUID(u.ManagerID.Bytes)
        f.HasManager = true
    }
    if u.HiredOn.Valid {
        f.HiredOn = u.HiredOn.Time
        f.HasHiredOn = true
    }
    return f
}