package handlers

import (
    "errors"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type enrollmentItem struct {
    ID          string `json:"id"`
    CourseID    string `json:"course_id"`
    Title       string `json:"title"`
    Summary     string `json:"summary"`
    Kind        string `json:"kind"`
    Status      string `json:"status"`
    DueOn       string `json:"due_on,omitempty"`
    AssignedAt  string `json:"assigned_at"`
    CompletedAt string `json:"completed_at,omitempty"`
}

func ListMyEnrollments(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListMyEnrollments(r.Context(), claims.UserID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]enrollmentItem, 0, len(rows))
    for _, row := range rows {
        item := enrollmentItem{
            ID:         row.ID.String(),
            CourseID:   row.CourseID.String(),
            Title:      row.Title,
            Summary:    row.Summary,
            Kind:       row.Kind,
            Status:     row.Status,
            AssignedAt: row.AssignedAt.Format("2006-01-02T15:04:05Z07:00"),
        }
        // row.DueOn is pgtype.Date.
        if row.DueOn.Valid {
            item.DueOn = row.DueOn.Time.Format("2006-01-02")
        }
        // row.CompletedAt is pgtype.Timestamptz.
        if row.CompletedAt.Valid {
            item.CompletedAt = row.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"enrollments": out})
}

func EnrollSelf(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    courseID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid course id")
        return
    }

    c, err := q.GetCourse(r.Context(), courseID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "course not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }
    if !c.CurrentVersionID.Valid {
        httpx.Error(w, http.StatusConflict, "no_published_version", "course has no published version")
        return
    }
    versionID := uuid.UUID(c.CurrentVersionID.Bytes)

    newID := uuid.Must(uuid.NewV7())
    en, err := q.CreateEnrollment(r.Context(), db.CreateEnrollmentParams{
        ID:              newID,
        TenantID:        claims.TenantID,
        UserID:          claims.UserID,
        CourseID:        c.ID,
        CourseVersionID: versionID,
        Source:          "manual",
        DueOn:           pgtype.Date{}, // NULL
    })
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "enroll_failed", "")
        return
    }
    httpx.JSON(w, http.StatusOK, map[string]any{
        "enrollment_id": en.ID.String(),
        "status":        en.Status,
    })
}