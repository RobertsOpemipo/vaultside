package handlers

import (
    "encoding/json"
    "errors"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type progressRequest struct {
    Status        string `json:"status"`
    SecondsViewed int32  `json:"seconds_viewed"`
    LastPositionS *int32 `json:"last_position_s"`
}

type progressResponse struct {
    ID                  string `json:"id"`
    Status              string `json:"status"`
    SecondsViewed       int32  `json:"seconds_viewed"`
    EnrollmentCompleted bool   `json:"enrollment_completed"`
}

func RecordLessonProgress(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    enrollmentID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid enrollment id")
        return
    }
    lessonID, err := uuid.Parse(chi.URLParam(r, "lessonId"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid lesson id")
        return
    }

    en, err := q.GetEnrollment(r.Context(), enrollmentID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "enrollment not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }
    if en.UserID != claims.UserID {
        httpx.Error(w, http.StatusForbidden, "forbidden", "not your enrollment")
        return
    }

    lesson, err := q.GetLesson(r.Context(), lessonID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "lesson not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }
    if lesson.VersionID != en.CourseVersionID {
        httpx.Error(w, http.StatusBadRequest, "wrong_lesson", "lesson not part of this course version")
        return
    }

    var req progressRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    if req.Status != "in_progress" && req.Status != "completed" {
        httpx.Error(w, http.StatusBadRequest, "bad_status", "status must be in_progress or completed")
        return
    }
    if req.SecondsViewed < 0 {
        req.SecondsViewed = 0
    }

    row, err := q.UpsertLessonProgress(r.Context(), db.UpsertLessonProgressParams{
        ID:            uuid.Must(uuid.NewV7()),
        TenantID:      claims.TenantID,
        EnrollmentID:  enrollmentID,
        LessonID:      lessonID,
        Status:        req.Status,
        SecondsViewed: req.SecondsViewed,
        LastPositionS: req.LastPositionS,
    })
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "progress_failed", "")
        return
    }

    completed := en.Status == "completed"
    if !completed {
        completed = maybeCompleteEnrollment(r, q, en)
    }

    httpx.JSON(w, http.StatusOK, progressResponse{
        ID:                  row.ID.String(),
        Status:              row.Status,
        SecondsViewed:       row.SecondsViewed,
        EnrollmentCompleted: completed,
    })
}

// maybeCompleteEnrollment flips enrollment to completed when:
//   1. every lesson in the version is complete, AND
//   2a. there is no assessment on the version, OR
//   2b. the learner has a passing attempt on the version's assessment.
//
// After a successful completion, it calls issueCertificateIfNeeded.
func maybeCompleteEnrollment(r *http.Request, q *db.Queries, en db.Enrollment) bool {
    if en.Status == "completed" {
        return true
    }

    lessons, err := q.ListLessons(r.Context(), en.CourseVersionID)
    if err != nil || len(lessons) == 0 {
        return false
    }
    total := int64(len(lessons))

    done, err := q.CountCompletedLessons(r.Context(), en.ID)
    if err != nil || done < total {
        return false
    }

    ass, err := q.GetAssessmentForVersion(r.Context(), en.CourseVersionID)
    switch {
    case err == nil:
        n, err := q.CountPassingAttempts(r.Context(), db.CountPassingAttemptsParams{
            EnrollmentID: en.ID,
            AssessmentID: ass.ID,
        })
        if err != nil || n == 0 {
            return false
        }
    case errors.Is(err, pgx.ErrNoRows):
        // no assessment — lessons alone are enough
    default:
        return false
    }

    if err := q.MarkEnrollmentCompleted(r.Context(), en.ID); err != nil {
        return false
    }

    // Refetch to get the updated status, then auto-issue a certificate.
    if updated, err := q.GetEnrollment(r.Context(), en.ID); err == nil {
        issueCertificateIfNeeded(r.Context(), q, updated)
    }
    return true
}