package handlers

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
)

// generateVerifyCode produces a short, human-readable code for QR verification.
// Format: VLT-XXXXXXXXXXXXXXXX (16 hex chars, uppercase).
func generateVerifyCode() string {
    b := make([]byte, 8)
    _, _ = rand.Read(b)
    return "VLT-" + strings.ToUpper(hex.EncodeToString(b))
}

// issueCertificateIfNeeded is called after an enrollment transitions to
// completed. It is idempotent: if the user already holds an active
// certificate for this course version, it does nothing.
//
// Expiry rules:
//   - compliance → 365 days
//   - onboarding → no expiry
//   - skills     → no expiry
func issueCertificateIfNeeded(ctx context.Context, q *db.Queries, en db.Enrollment) {
    claims, ok := auth.FromContext(ctx)
    if !ok {
        return
    }
    if en.Status != "completed" {
        return
    }

    n, err := q.CountActiveCertificatesForUserVersion(ctx,
        db.CountActiveCertificatesForUserVersionParams{
            UserID:          en.UserID,
            CourseVersionID: en.CourseVersionID,
        })
    if err != nil || n > 0 {
        return
    }

    course, err := q.GetCourse(ctx, en.CourseID)
    if err != nil {
        return
    }

    var expiresAt pgtype.Timestamptz
    if course.Kind == "compliance" {
        t := time.Now().AddDate(1, 0, 0)
        expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
    }

    verifyCode := generateVerifyCode()
    cert, err := q.CreateCertificate(ctx, db.CreateCertificateParams{
        ID:              uuid.Must(uuid.NewV7()),
        TenantID:        claims.TenantID,
        UserID:          en.UserID,
        CourseID:        en.CourseID,
        CourseVersionID: en.CourseVersionID,
        ExpiresAt:       expiresAt,
        VerifyCode:      verifyCode,
        IssuedBy:        pgtype.UUID{},
    })
    if err != nil {
        return
    }

    detail, _ := json.Marshal(map[string]any{
        "course_id":     en.CourseID.String(),
        "course_kind":   course.Kind,
        "expires_at":    expiresAt.Time,
        "issued_source": "auto",
    })
    _, _ = q.CreateCertificateEvent(ctx, db.CreateCertificateEventParams{
        TenantID:      claims.TenantID,
        CertificateID: cert.ID,
        EventType:     "issued",
        Detail:        detail,
        ActorID:       pgtype.UUID{},
    })
}