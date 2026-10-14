package handlers

import (
    "errors"
    "net/http"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type docketEnrollmentCounts struct {
    Assigned   int64 `json:"assigned"`
    InProgress int64 `json:"in_progress"`
    Completed  int64 `json:"completed"`
    Overdue    int64 `json:"overdue"`
    Total      int64 `json:"total"`
}

type docketOnboarding struct {
    InstanceID          string `json:"instance_id"`
    TemplateTitle       string `json:"template_title"`
    ItemsTotal          int64  `json:"items_total"`
    ItemsCompleted      int64  `json:"items_completed"`
    ItemsAwaiting       int64  `json:"items_awaiting"`
    ItemsPendingManager int64  `json:"items_pending_manager"`
}

type docketReport struct {
    UserID      string                 `json:"user_id"`
    Email       string                 `json:"email"`
    FullName    string                 `json:"full_name"`
    Enrollments docketEnrollmentCounts `json:"enrollments"`
    Onboarding  *docketOnboarding      `json:"onboarding,omitempty"`
}

type docketResponse struct {
    ManagerID    string         `json:"manager_id"`
    ManagerEmail string         `json:"manager_email"`
    Reports      []docketReport `json:"reports"`
}

// TeamDocket returns the manager's direct reports with per-report enrollment
// and onboarding status. Requires the team.view permission.
func TeamDocket(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    reports, err := q.ListDirectReports(r.Context(),
        pgtype.UUID{Bytes: claims.UserID, Valid: true})
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "reports_failed", "")
        return
    }

    out := make([]docketReport, 0, len(reports))
    for _, rep := range reports {
        entry := docketReport{
            UserID:   rep.ID.String(),
            Email:    rep.Email,
            FullName: rep.FullName,
        }

        rows, err := q.CountEnrollmentsByStatusForUser(r.Context(), rep.ID)
        if err != nil {
            httpx.Error(w, http.StatusInternalServerError, "enrollments_failed", "")
            return
        }
        for _, row := range rows {
            switch row.Status {
            case "assigned":
                entry.Enrollments.Assigned = row.N
            case "in_progress":
                entry.Enrollments.InProgress = row.N
            case "completed":
                entry.Enrollments.Completed = row.N
            }
            entry.Enrollments.Total += row.N
        }

        overdue, err := q.CountOverdueEnrollmentsForUser(r.Context(), rep.ID)
        if err == nil {
            entry.Enrollments.Overdue = overdue
        }

        summary, err := q.GetOnboardingSummaryForUser(r.Context(), rep.ID)
        if err == nil {
            entry.Onboarding = &docketOnboarding{
                InstanceID:          summary.InstanceID.String(),
                TemplateTitle:       summary.TemplateTitle,
                ItemsTotal:          summary.ItemsTotal,
                ItemsCompleted:      summary.ItemsCompleted,
                ItemsAwaiting:       summary.ItemsAwaiting,
                ItemsPendingManager: summary.ItemsPendingManager,
            }
        } else if !errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusInternalServerError, "onboarding_failed", "")
            return
        }

        out = append(out, entry)
    }

    httpx.JSON(w, http.StatusOK, docketResponse{
        ManagerID:    claims.UserID.String(),
        ManagerEmail: claims.UserEmail,
        Reports:      out,
    })
}