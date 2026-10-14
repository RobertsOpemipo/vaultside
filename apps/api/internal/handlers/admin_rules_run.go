package handlers

import (
    "errors"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
    "github.com/vaultside/vaultside/apps/api/internal/rules"
)

type runSummary struct {
    RulesEvaluated             int `json:"rules_evaluated"`
    UsersMatched               int `json:"users_matched"`
    EnrollmentsCreated         int `json:"enrollments_created"`
    OnboardingInstancesCreated int `json:"onboarding_instances_created"`
    OnboardingItemsCreated     int `json:"onboarding_items_created"`
}

type runDetail struct {
    RuleID                     string `json:"rule_id"`
    RuleName                   string `json:"rule_name"`
    UserID                     string `json:"user_id"`
    UserEmail                  string `json:"user_email"`
    EnrollmentsCreated         int    `json:"enrollments_created"`
    OnboardingInstanceCreated  bool   `json:"onboarding_instance_created"`
    OnboardingItemsCreated     int    `json:"onboarding_items_created"`
}

type runResponse struct {
    Summary runSummary  `json:"summary"`
    Details []runDetail `json:"details"`
}

// RunRules evaluates every active assignment rule and, for each matching user,
// creates enrollments for the path's courses and starts the onboarding
// checklist if the rule is linked to a template. Idempotent: safe to run
// repeatedly, and does not create duplicates.
func RunRules(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    activeRules, err := q.ListActiveAssignmentRules(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "rules_failed", "")
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

    now := time.Now()
    summary := runSummary{RulesEvaluated: len(activeRules)}
    details := make([]runDetail, 0)

    for _, rule := range activeRules {
        pathItems, err := q.ListPathItems(r.Context(), rule.PathID)
        if err != nil {
            httpx.Error(w, http.StatusInternalServerError, "path_items_failed", "")
            return
        }

        var templateID uuid.UUID
        var templateItems []db.ListOnboardingItemsRow
        if rule.OnboardingTemplateID.Valid {
            templateID = uuid.UUID(rule.OnboardingTemplateID.Bytes)
            templateItems, err = q.ListOnboardingItems(r.Context(), templateID)
            if err != nil {
                httpx.Error(w, http.StatusInternalServerError, "template_items_failed", "")
                return
            }
        }

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
            summary.UsersMatched++

            detail := runDetail{
                RuleID:    rule.ID.String(),
                RuleName:  rule.Name,
                UserID:    u.ID.String(),
                UserEmail: u.Email,
            }

            // Enroll in every path item's current version.
            for _, item := range pathItems {
                if !item.CurrentVersionID.Valid {
                    continue
                }
                n, err := q.CreateEnrollmentIfMissing(r.Context(), db.CreateEnrollmentIfMissingParams{
                    ID:              uuid.Must(uuid.NewV7()),
                    TenantID:        claims.TenantID,
                    UserID:          u.ID,
                    CourseID:        item.CourseID,
                    CourseVersionID: uuid.UUID(item.CurrentVersionID.Bytes),
                })
                if err != nil {
                    httpx.Error(w, http.StatusInternalServerError, "enroll_failed", "")
                    return
                }
                detail.EnrollmentsCreated += int(n)
            }
            summary.EnrollmentsCreated += detail.EnrollmentsCreated

            // Start the onboarding checklist if the rule links one.
            if len(templateItems) > 0 {
                _, err := q.GetOnboardingInstanceByUserTemplate(r.Context(),
                    db.GetOnboardingInstanceByUserTemplateParams{
                        UserID:     u.ID,
                        TemplateID: templateID,
                    })
                isNew := errors.Is(err, pgx.ErrNoRows)

                inst, err := q.GetOrCreateOnboardingInstance(r.Context(),
                    db.GetOrCreateOnboardingInstanceParams{
                        ID:         uuid.Must(uuid.NewV7()),
                        TenantID:   claims.TenantID,
                        UserID:     u.ID,
                        TemplateID: templateID,
                    })
                if err != nil {
                    httpx.Error(w, http.StatusInternalServerError, "onboarding_instance_failed", "")
                    return
                }
                if isNew {
                    detail.OnboardingInstanceCreated = true
                    summary.OnboardingInstancesCreated++
                }

                // Pre-create the item rows so the manager docket has something to read.
                for _, ti := range templateItems {
                    _, err := q.UpsertOnboardingItemProgress(r.Context(),
                        db.UpsertOnboardingItemProgressParams{
                            ID:             uuid.Must(uuid.NewV7()),
                            TenantID:       claims.TenantID,
                            InstanceID:     inst.ID,
                            TemplateItemID: ti.ID,
                            Status:         "not_started",
                        })
                    if err != nil {
                        httpx.Error(w, http.StatusInternalServerError, "item_progress_failed", "")
                        return
                    }
                    if isNew {
                        detail.OnboardingItemsCreated++
                    }
                }
                summary.OnboardingItemsCreated += detail.OnboardingItemsCreated
            }

            details = append(details, detail)
        }
    }

    httpx.JSON(w, http.StatusOK, runResponse{
        Summary: summary,
        Details: details,
    })
}