package handlers

import (
    "errors"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"

    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type courseListItem struct {
    ID             string `json:"id"`
    Title          string `json:"title"`
    Summary        string `json:"summary"`
    Kind           string `json:"kind"`
    CurrentVersion *int32 `json:"current_version,omitempty"`
}

func ListCourses(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListCourses(r.Context(), db.ListCoursesParams{Limit: 50, Offset: 0})
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]courseListItem, 0, len(rows))
    for _, row := range rows {
        item := courseListItem{
            ID:      row.ID.String(),
            Title:   row.Title,
            Summary: row.Summary,
            Kind:    row.Kind,
        }
        if row.CurrentVersion != nil {
            item.CurrentVersion = row.CurrentVersion
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"courses": out})
}

type lessonSummary struct {
    ID        string `json:"id"`
    Position  int32  `json:"position"`
    Title     string `json:"title"`
    Kind      string `json:"kind"`
    DurationS int32  `json:"duration_s"`
}

type courseDetail struct {
    ID         string          `json:"id"`
    Title      string          `json:"title"`
    Summary    string          `json:"summary"`
    Kind       string          `json:"kind"`
    VersionID  string          `json:"version_id,omitempty"`
    VersionNum int32           `json:"version,omitempty"`
    Lessons    []lessonSummary `json:"lessons"`
}

func GetCourseDetail(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid course id")
        return
    }
    q := httpx.MustQueries(r.Context())

    c, err := q.GetCourse(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "course not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    detail := courseDetail{
        ID:      c.ID.String(),
        Title:   c.Title,
        Summary: c.Summary,
        Kind:    c.Kind,
        Lessons: []lessonSummary{},
    }

    // c.CurrentVersionID is pgtype.UUID (nullable uuid column);
    // use .Valid and convert .Bytes to uuid.UUID.
    if c.CurrentVersionID.Valid {
        versionID := uuid.UUID(c.CurrentVersionID.Bytes)
        cv, err := q.GetCourseVersion(r.Context(), versionID)
        if err == nil {
            detail.VersionID = cv.ID.String()
            detail.VersionNum = cv.Version
            lessons, err := q.ListLessons(r.Context(), cv.ID)
            if err == nil {
                for _, l := range lessons {
                    detail.Lessons = append(detail.Lessons, lessonSummary{
                        ID:        l.ID.String(),
                        Position:  l.Position,
                        Title:     l.Title,
                        Kind:      l.Kind,
                        DurationS: l.DurationS,
                    })
                }
            }
        }
    }

    httpx.JSON(w, http.StatusOK, detail)
}

func GetLessonDetail(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid lesson id")
        return
    }
    q := httpx.MustQueries(r.Context())
    l, err := q.GetLesson(r.Context(), id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "lesson not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    type lessonDetail struct {
        ID        string `json:"id"`
        VersionID string `json:"version_id"`
        Position  int32  `json:"position"`
        Title     string `json:"title"`
        Kind      string `json:"kind"`
        ContentMD string `json:"content_md,omitempty"`
        AssetID   string `json:"asset_id,omitempty"`
        DurationS int32  `json:"duration_s"`
    }
    out := lessonDetail{
        ID:        l.ID.String(),
        VersionID: l.VersionID.String(),
        Position:  l.Position,
        Title:     l.Title,
        Kind:      l.Kind,
        DurationS: l.DurationS,
    }
    // l.ContentMd is *string (sqlc pointer-ized text).
    if l.ContentMd != nil {
        out.ContentMD = *l.ContentMd
    }
    // l.AssetID is pgtype.UUID (nullable uuid).
    if l.AssetID.Valid {
        out.AssetID = uuid.UUID(l.AssetID.Bytes).String()
    }
    httpx.JSON(w, http.StatusOK, out)
}