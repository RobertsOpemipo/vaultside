package scorm

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
)

// Sentinel errors. Handlers map ErrNotFound -> 404, ErrNoEnrollment -> 403,
// ErrBadKey -> 400.
var (
    ErrNotFound     = errors.New("scorm: not found")
    ErrNoEnrollment = errors.New("scorm: no active enrollment")
    ErrBadKey       = errors.New("scorm: key not writable")
)

// Runtime is the domain service. It never opens its own transaction; callers
// pass a tenant-scoped pgx.Tx (the existing session middleware for Launch, the
// SCORM token middleware for the rest).
type Runtime struct{}

// LaunchInput identifies what to launch.
type LaunchInput struct {
    TenantID uuid.UUID
    UserID   uuid.UUID
    LessonID uuid.UUID
    Version  Version // defaults to V12 when empty
}

// LaunchResult is returned to the client to bootstrap the SCORM API shim.
type LaunchResult struct {
    AttemptID     uuid.UUID         `json:"attempt_id"`
    AttemptNumber int               `json:"attempt_number"`
    Version       Version           `json:"version"`
    CMI           map[string]string `json:"cmi"`
    EntryPoint    string            `json:"entry_point"`
    PackageID     uuid.UUID         `json:"package_id"`
    Resumed       bool              `json:"resumed"`
    SessionToken  string            `json:"session_token"`
    ExpiresAt     time.Time         `json:"expires_at"`
}

// Snapshot is the current attempt state.
type Snapshot struct {
    AttemptID        uuid.UUID         `json:"attempt_id"`
    Status           string            `json:"status"`
    Version          Version           `json:"version"`
    CMI              map[string]string `json:"cmi"`
    ScoreRaw         *float64          `json:"score_raw,omitempty"`
    ScoreMin         *float64          `json:"score_min,omitempty"`
    ScoreMax         *float64          `json:"score_max,omitempty"`
    CompletionStatus string            `json:"completion_status,omitempty"`
    SuccessStatus    string            `json:"success_status,omitempty"`
    SessionTimeS     int               `json:"session_time_s"`
    StartedAt        time.Time         `json:"started_at"`
    LastCommitAt     time.Time         `json:"last_commit_at"`
    CompletedAt      *time.Time        `json:"completed_at,omitempty"`
}

// Launch opens (or resumes) a SCORM attempt for a user and lesson.
//
// Schema assumption: scorm_packages has (id, tenant_id, lesson_id, entry_point).
// If your FK is lessons.scorm_package_id instead, change the join in step 1.
func (r *Runtime) Launch(ctx context.Context, tx pgx.Tx, in LaunchInput, secret []byte, now time.Time, ttl time.Duration) (*LaunchResult, error) {
    if in.Version == "" {
        in.Version = V12
    }

    // 1. Resolve lesson -> package + course version.
    var (
        courseVersionID uuid.UUID
        packageID       uuid.UUID
        entryPoint      string
    )
    err := tx.QueryRow(ctx, `
        SELECT l.course_version_id, sp.id, COALESCE(sp.entry_point, 'index.html')
        FROM lessons l
        JOIN scorm_packages sp ON sp.lesson_id = l.id
        WHERE l.id = $1
        LIMIT 1
    `, in.LessonID).Scan(&courseVersionID, &packageID, &entryPoint)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("scorm: resolve lesson: %w", err)
    }

    // 2. Active enrollment for the learner on this course version.
    var enrollmentID uuid.UUID
    err = tx.QueryRow(ctx, `
        SELECT id FROM enrollments
        WHERE user_id = $1 AND course_version_id = $2
        LIMIT 1
    `, in.UserID, courseVersionID).Scan(&enrollmentID)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNoEnrollment
    }
    if err != nil {
        return nil, fmt.Errorf("scorm: resolve enrollment: %w", err)
    }

    // 3. Learner display name for CMI.
    var learnerName string
    _ = tx.QueryRow(ctx, `SELECT full_name FROM users WHERE id = $1`, in.UserID).Scan(&learnerName)
    learnerID := in.UserID.String()

    // 4. Is there an in-progress or abandoned attempt we should resume?
    var (
        prevID     uuid.UUID
        prevCMIRaw []byte
        prevNumber int
        prevStatus string
    )
    err = tx.QueryRow(ctx, `
        SELECT id, cmi_data, attempt_number, status
        FROM scorm_attempts
        WHERE user_id = $1 AND scorm_package_id = $2
        ORDER BY attempt_number DESC
        LIMIT 1
    `, in.UserID, packageID).Scan(&prevID, &prevCMIRaw, &prevNumber, &prevStatus)
    resume := err == nil && (prevStatus == StatusInProgress || prevStatus == StatusAbandoned)
    if err != nil && !errors.Is(err, pgx.ErrNoRows) {
        return nil, fmt.Errorf("scorm: lookup previous attempt: %w", err)
    }

    if resume {
        cmi := decodeCMI(prevCMIRaw)
        version := DetectVersion(cmi)
        // Refresh learner fields in case they changed since last launch.
        mergeLearner(cmi, version, learnerID, learnerName)
        token, exp := signFor(secret, in, prevID, now, ttl)
        return &LaunchResult{
            AttemptID:     prevID,
            AttemptNumber: prevNumber,
            Version:       version,
            CMI:           cmi,
            EntryPoint:    entryPoint,
            PackageID:     packageID,
            Resumed:       true,
            SessionToken:  token,
            ExpiresAt:     exp,
        }, nil
    }

    // 5. Insert a fresh attempt.
    newID := uuid.New()
    cmi := InitialCMI(in.Version, learnerID, learnerName, false)
    encoded, _ := encodeCMI(cmi)
    nextNumber := prevNumber + 1
    if prevNumber == 0 {
        nextNumber = 1
    }
    _, err = tx.Exec(ctx, `
        INSERT INTO scorm_attempts
            (id, tenant_id, user_id, scorm_package_id, enrollment_id, lesson_id,
             attempt_number, status, cmi_data)
        VALUES ($1, $2, $3, $4, $5, $6, $7, 'in_progress', $8)
    `, newID, in.TenantID, in.UserID, packageID, enrollmentID, in.LessonID,
        nextNumber, encoded)
    if err != nil {
        return nil, fmt.Errorf("scorm: insert attempt: %w", err)
    }

    token, exp := signFor(secret, in, newID, now, ttl)
    return &LaunchResult{
        AttemptID:     newID,
        AttemptNumber: nextNumber,
        Version:       in.Version,
        CMI:           cmi,
        EntryPoint:    entryPoint,
        PackageID:     packageID,
        SessionToken:  token,
        ExpiresAt:     exp,
    }, nil
}

// Get returns the current attempt snapshot.
func (r *Runtime) Get(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID) (*Snapshot, error) {
    row := tx.QueryRow(ctx, `
        SELECT cmi_data, status, score_raw, score_min, score_max,
               completion_status, success_status, session_time_s,
               started_at, last_commit_at, completed_at
        FROM scorm_attempts
        WHERE id = $1
    `, attemptID)
    return scanSnapshot(row)
}

// Commit applies a batch of writable CMI keys and persists typed columns.
func (r *Runtime) Commit(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, values map[string]string, now time.Time) (*Snapshot, error) {
    return r.apply(ctx, tx, attemptID, values, false, now)
}

// Finish applies a final batch, marks the attempt completed/passed/failed,
// and stamps completed_at.
func (r *Runtime) Finish(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, values map[string]string, now time.Time) (*Snapshot, error) {
    return r.apply(ctx, tx, attemptID, values, true, now)
}

// Abandon marks an attempt as abandoned (learner closed the tab without finish).
func (r *Runtime) Abandon(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID) error {
    _, err := tx.Exec(ctx, `
        UPDATE scorm_attempts SET status = 'abandoned'
        WHERE id = $1 AND status = 'in_progress'
    `, attemptID)
    if err != nil {
        return fmt.Errorf("scorm: abandon: %w", err)
    }
    return nil
}

func (r *Runtime) apply(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, values map[string]string, finish bool, now time.Time) (*Snapshot, error) {
    // Read current CMI.
    var raw []byte
    var status string
    err := tx.QueryRow(ctx,
        `SELECT cmi_data, status FROM scorm_attempts WHERE id = $1`, attemptID).
        Scan(&raw, &status)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("scorm: load attempt: %w", err)
    }

    cmi := decodeCMI(raw)
    version := DetectVersion(cmi)

    // Validate and merge incoming keys.
    for k, v := range values {
        if !IsWritable(version, k) {
            return nil, fmt.Errorf("%w: %s", ErrBadKey, k)
        }
        cmi[k] = v
    }

    d := Derive(version, cmi)
    newStatus := StatusForDerived(d)
    if finish {
        switch newStatus {
        case StatusPassed, StatusFailed, StatusCompleted:
            // keep
        default:
            newStatus = StatusCompleted
        }
        _, err = tx.Exec(ctx, `
            UPDATE scorm_attempts SET
                cmi_data = $2,
                status = $3,
                score_raw = $4, score_min = $5, score_max = $6,
                completion_status = NULLIF($7, ''),
                success_status    = NULLIF($8, ''),
                suspend_data      = NULLIF($9, ''),
                session_time_s    = $10,
                last_commit_at    = $11,
                completed_at      = COALESCE(completed_at, $11)
            WHERE id = $1
        `, attemptID, mustJSON(cmi), newStatus,
            d.ScoreRaw, d.ScoreMin, d.ScoreMax,
            d.CompletionStatus, d.SuccessStatus, d.SuspendData,
            d.SessionTimeS, now)
    } else {
        _, err = tx.Exec(ctx, `
            UPDATE scorm_attempts SET
                cmi_data = $2,
                status = $3,
                score_raw = $4, score_min = $5, score_max = $6,
                completion_status = NULLIF($7, ''),
                success_status    = NULLIF($8, ''),
                suspend_data      = NULLIF($9, ''),
                session_time_s    = $10,
                last_commit_at    = $11
            WHERE id = $1
        `, attemptID, mustJSON(cmi), newStatus,
            d.ScoreRaw, d.ScoreMin, d.ScoreMax,
            d.CompletionStatus, d.SuccessStatus, d.SuspendData,
            d.SessionTimeS, now)
    }
    if err != nil {
        return nil, fmt.Errorf("scorm: update attempt: %w", err)
    }

    row := tx.QueryRow(ctx, `
        SELECT cmi_data, status, score_raw, score_min, score_max,
               completion_status, success_status, session_time_s,
               started_at, last_commit_at, completed_at
        FROM scorm_attempts WHERE id = $1
    `, attemptID)
    return scanSnapshot(row)
}

// ── helpers ─────────────────────────────────────────────────────────────────

func signFor(secret []byte, in LaunchInput, attemptID uuid.UUID, now time.Time, ttl time.Duration) (string, time.Time) {
    if ttl <= 0 {
        ttl = 4 * time.Hour
    }
    exp := now.Add(ttl)
    return SignAttemptToken(secret, AttemptClaims{
        AttemptID: attemptID,
        TenantID:  in.TenantID,
        UserID:    in.UserID,
        ExpiresAt: exp.Unix(),
    }), exp
}

func decodeCMI(raw []byte) map[string]string {
    out := map[string]string{}
    if len(raw) > 0 {
        _ = json.Unmarshal(raw, &out)
    }
    return out
}

func encodeCMI(cmi map[string]string) ([]byte, error) {
    return json.Marshal(cmi)
}

func mustJSON(v any) []byte {
    b, _ := json.Marshal(v)
    return b
}

func mergeLearner(cmi map[string]string, v Version, id, name string) {
    if v == V2004 {
        cmi["cmi.learner_id"] = id
        cmi["cmi.learner_name"] = name
        return
    }
    cmi["cmi.core.student_id"] = id
    cmi["cmi.core.student_name"] = name
}

type rowScanner interface {
    Scan(dest ...any) error
}

func scanSnapshot(row rowScanner) (*Snapshot, error) {
    var (
        raw              []byte
        s                Snapshot
        scoreRaw         *float64
        scoreMin         *float64
        scoreMax         *float64
        completionStatus *string
        successStatus    *string
    )
    err := row.Scan(&raw, &s.Status, &scoreRaw, &scoreMin, &scoreMax,
        &completionStatus, &successStatus, &s.SessionTimeS,
        &s.StartedAt, &s.LastCommitAt, &s.CompletedAt)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    s.CMI = decodeCMI(raw)
    s.Version = DetectVersion(s.CMI)
    s.ScoreRaw = scoreRaw
    s.ScoreMin = scoreMin
    s.ScoreMax = scoreMax
    if completionStatus != nil {
        s.CompletionStatus = *completionStatus
    }
    if successStatus != nil {
        s.SuccessStatus = *successStatus
    }
    return &s, nil
}