package handlers

import (
    "encoding/json"
    "errors"
    "math"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/gamification"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type questionOptionView struct {
    ID       string `json:"id"`
    Position int32  `json:"position"`
    TextMD   string `json:"text_md"`
}

type questionView struct {
    ID       string               `json:"id"`
    Position int32                `json:"position"`
    Kind     string               `json:"kind"`
    PromptMD string               `json:"prompt_md"`
    Points   int32                `json:"points"`
    Options  []questionOptionView `json:"options"`
}

type startAttemptResponse struct {
    AttemptID   string         `json:"attempt_id"`
    AttemptNo   int32          `json:"attempt_number"`
    PassMark    int32          `json:"pass_mark"`
    MaxAttempts int32          `json:"max_attempts"`
    Questions   []questionView `json:"questions"`
}

func StartAttempt(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    enrollmentID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid enrollment id")
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

    ass, err := q.GetAssessmentForVersion(r.Context(), en.CourseVersionID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "no_assessment", "course has no assessment")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "assessment_failed", "")
        return
    }

    next, err := q.NextAttemptNumber(r.Context(), db.NextAttemptNumberParams{
        EnrollmentID: en.ID,
        AssessmentID: ass.ID,
    })
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "attempt_number_failed", "")
        return
    }
    if next > ass.MaxAttempts {
        httpx.Error(w, http.StatusForbidden, "max_attempts", "maximum attempts reached")
        return
    }

    row, err := q.CreateAttempt(r.Context(), db.CreateAttemptParams{
        ID:            uuid.Must(uuid.NewV7()),
        TenantID:      claims.TenantID,
        EnrollmentID:  en.ID,
        AssessmentID:  ass.ID,
        AttemptNumber: next,
    })
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "attempt_create_failed", "")
        return
    }

    questionRows, err := q.ListQuestions(r.Context(), ass.ID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "questions_failed", "")
        return
    }

    questions := make([]questionView, 0, len(questionRows))
    for _, qr := range questionRows {
        opts, err := q.ListOptions(r.Context(), qr.ID)
        if err != nil {
            httpx.Error(w, http.StatusInternalServerError, "options_failed", "")
            return
        }
        optViews := make([]questionOptionView, 0, len(opts))
        for _, o := range opts {
            optViews = append(optViews, questionOptionView{
                ID:       o.ID.String(),
                Position: o.Position,
                TextMD:   o.TextMd,
            })
        }
        questions = append(questions, questionView{
            ID:       qr.ID.String(),
            Position: qr.Position,
            Kind:     qr.Kind,
            PromptMD: qr.PromptMd,
            Points:   qr.Points,
            Options:  optViews,
        })
    }

    httpx.JSON(w, http.StatusOK, startAttemptResponse{
        AttemptID:   row.ID.String(),
        AttemptNo:   row.AttemptNumber,
        PassMark:    ass.PassMark,
        MaxAttempts: ass.MaxAttempts,
        Questions:   questions,
    })
}

type saveAnswerRequest struct {
    QuestionID string `json:"question_id"`
    OptionID   string `json:"option_id"`
}

func SaveAnswer(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    attemptID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid attempt id")
        return
    }

    attempt, err := q.GetAttempt(r.Context(), attemptID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "attempt not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }
    en, err := q.GetEnrollment(r.Context(), attempt.EnrollmentID)
    if err != nil || en.UserID != claims.UserID {
        httpx.Error(w, http.StatusForbidden, "forbidden", "not your attempt")
        return
    }
    if attempt.Status != "in_progress" {
        httpx.Error(w, http.StatusConflict, "not_in_progress", "attempt is not open")
        return
    }

    var req saveAnswerRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    questionID, err := uuid.Parse(req.QuestionID)
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid question id")
        return
    }

    var optionUUID pgtype.UUID
    if req.OptionID != "" {
        optID, err := uuid.Parse(req.OptionID)
        if err != nil {
            httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid option id")
            return
        }
        optionUUID = pgtype.UUID{Bytes: optID, Valid: true}
    }

    if err := q.SaveAttemptAnswer(r.Context(), db.SaveAttemptAnswerParams{
        ID:         uuid.Must(uuid.NewV7()),
        TenantID:   claims.TenantID,
        AttemptID:  attemptID,
        QuestionID: questionID,
        OptionID:   optionUUID,
    }); err != nil {
        httpx.Error(w, http.StatusInternalServerError, "save_failed", "")
        return
    }

    httpx.JSON(w, http.StatusOK, map[string]any{"saved": true})
}

type perQuestionResult struct {
    QuestionID     string `json:"question_id"`
    SelectedID     string `json:"selected_option_id,omitempty"`
    IsCorrect      bool   `json:"is_correct"`
    PointsAwarded  int32  `json:"points_awarded"`
    PointsPossible int32  `json:"points_possible"`
    ExplanationMD  string `json:"explanation_md,omitempty"`
}

type submitAttemptResponse struct {
    AttemptID         string              `json:"attempt_id"`
    Score             int32               `json:"score"`
    Passed            bool                `json:"passed"`
    PassMark          int32               `json:"pass_mark"`
    Correct           int                 `json:"correct"`
    Total             int                 `json:"total"`
    Results           []perQuestionResult `json:"results"`
    XPAwarded         int32               `json:"xp_awarded"`
    TotalXP           int64               `json:"total_xp"`
    Level             int                 `json:"level"`
    CurrentStreakDays int32               `json:"current_streak_days"`
}

func SubmitAttempt(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    attemptID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid attempt id")
        return
    }

    attempt, err := q.GetAttempt(r.Context(), attemptID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "attempt not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }
    en, err := q.GetEnrollment(r.Context(), attempt.EnrollmentID)
    if err != nil || en.UserID != claims.UserID {
        httpx.Error(w, http.StatusForbidden, "forbidden", "not your attempt")
        return
    }
    if attempt.Status != "in_progress" {
        httpx.Error(w, http.StatusConflict, "already_submitted", "attempt already submitted")
        return
    }

    ass, err := q.GetAssessment(r.Context(), attempt.AssessmentID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "assessment_failed", "")
        return
    }

    questions, err := q.ListQuestions(r.Context(), ass.ID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "questions_failed", "")
        return
    }

    answerRows, err := q.ListAttemptAnswersDetailed(r.Context(), attemptID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "answers_failed", "")
        return
    }
    answerByQuestion := make(map[uuid.UUID]db.ListAttemptAnswersDetailedRow, len(answerRows))
    for _, ar := range answerRows {
        answerByQuestion[ar.QuestionID] = ar
    }

    var totalPoints, earnedPoints int32
    results := make([]perQuestionResult, 0, len(questions))

    for _, qr := range questions {
        totalPoints += qr.Points
        res := perQuestionResult{
            QuestionID:     qr.ID.String(),
            PointsPossible: qr.Points,
        }
        if qr.ExplanationMd != nil {
            res.ExplanationMD = *qr.ExplanationMd
        }
        if ar, ok := answerByQuestion[qr.ID]; ok && ar.OptionID.Valid {
            res.SelectedID = uuid.UUID(ar.OptionID.Bytes).String()
            if ar.IsCorrect != nil && *ar.IsCorrect {
                res.IsCorrect = true
                res.PointsAwarded = qr.Points
                earnedPoints += qr.Points
            }
        }
        results = append(results, res)
    }

    var score int32
    if totalPoints > 0 {
        score = int32(math.Round(float64(earnedPoints) / float64(totalPoints) * 100))
    }
    passed := score >= ass.PassMark

    if _, err := q.SubmitAttempt(r.Context(), db.SubmitAttemptParams{
        ID:     attemptID,
        Score:  &score,
        Passed: &passed,
    }); err != nil {
        httpx.Error(w, http.StatusInternalServerError, "submit_failed", "")
        return
    }

    if passed {
        tryCompleteAfterPass(r, q, en)
    }

    correct := 0
    for _, res := range results {
        if res.IsCorrect {
            correct++
        }
    }

    // ---------------- gamification ----------------
    var xpAwarded int32
    streakDays := int32(0)

    if passed {
        awarded, _ := gamification.AwardOnce(
            r.Context(), q, claims.TenantID, claims.UserID,
            gamification.ReasonQuizPass, "assessment", ass.ID,
            gamification.XPQuizPass,
        )
        if awarded {
            xpAwarded = gamification.XPQuizPass
        }
        sr, err := gamification.TouchStreak(r.Context(), q, claims.TenantID, claims.UserID, time.Now())
        if err == nil {
            streakDays = sr.CurrentDays
        }
    }

    total, _ := q.SumXPForUser(r.Context(), claims.UserID)
    level := gamification.LevelForXP(total)

    httpx.JSON(w, http.StatusOK, submitAttemptResponse{
        AttemptID:         attemptID.String(),
        Score:             score,
        Passed:            passed,
        PassMark:          ass.PassMark,
        Correct:           correct,
        Total:             len(questions),
        Results:           results,
        XPAwarded:         xpAwarded,
        TotalXP:           total,
        Level:             level,
        CurrentStreakDays: streakDays,
    })
}

func tryCompleteAfterPass(r *http.Request, q *db.Queries, en db.Enrollment) {
    if en.Status == "completed" {
        return
    }
    lessons, err := q.ListLessons(r.Context(), en.CourseVersionID)
    if err != nil || len(lessons) == 0 {
        return
    }
    done, err := q.CountCompletedLessons(r.Context(), en.ID)
    if err != nil || done < int64(len(lessons)) {
        return
    }
    if err := q.MarkEnrollmentCompleted(r.Context(), en.ID); err != nil {
        return
    }
    if updated, err := q.GetEnrollment(r.Context(), en.ID); err == nil {
        issueCertificateIfNeeded(r.Context(), q, updated)
    }
}