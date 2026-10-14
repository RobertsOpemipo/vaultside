package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

type auditPackRequest struct {
    From string `json:"from"`
    To   string `json:"to"`
}

type auditPackUserRow struct {
    UserID             string `json:"user_id"`
    Email              string `json:"email"`
    FullName           string `json:"full_name"`
    Department         string `json:"department"`
    HiredOn            string `json:"hired_on,omitempty"`
    Status             string `json:"status"`
    ActiveCertificates int64  `json:"active_certificates"`
    TotalAttestations  int64  `json:"total_attestations"`
}

type auditPackAttestation struct {
    ID                string `json:"id"`
    UserID            string `json:"user_id"`
    UserEmail         string `json:"user_email"`
    UserFullName      string `json:"user_full_name"`
    RegulationKey     string `json:"regulation_key"`
    RegulationName    string `json:"regulation_name"`
    RegulationVersion int32  `json:"regulation_version"`
    TextHash          string `json:"text_hash"`
    SignedAt          string `json:"signed_at"`
}

type auditPackCertificate struct {
    ID           string `json:"id"`
    UserID       string `json:"user_id"`
    UserEmail    string `json:"user_email"`
    UserFullName string `json:"user_full_name"`
    CourseTitle  string `json:"course_title"`
    CourseKind   string `json:"course_kind"`
    VerifyCode   string `json:"verify_code"`
    IssuedAt     string `json:"issued_at"`
    ExpiresAt    string `json:"expires_at,omitempty"`
    Revoked      bool   `json:"revoked"`
}

type auditPackAttempt struct {
    ID              string `json:"id"`
    UserEmail       string `json:"user_email"`
    UserFullName    string `json:"user_full_name"`
    AssessmentTitle string `json:"assessment_title"`
    AttemptNumber   int32  `json:"attempt_number"`
    Status          string `json:"status"`
    Score           *int32 `json:"score,omitempty"`
    Passed          *bool  `json:"passed,omitempty"`
    StartedAt       string `json:"started_at"`
    SubmittedAt     string `json:"submitted_at,omitempty"`
}

type auditPackMetadata struct {
    GeneratedAt   string `json:"generated_at"`
    TenantID      string `json:"tenant_id"`
    TenantSlug    string `json:"tenant_slug"`
    From          string `json:"from"`
    To            string `json:"to"`
    GeneratedBy   string `json:"generated_by"`
    SchemaVersion string `json:"schema_version"`
}

type auditPackSummary struct {
    UserCount        int `json:"user_count"`
    AttestationCount int `json:"attestation_count"`
    CertificateCount int `json:"certificate_count"`
    AttemptCount     int `json:"attempt_count"`
}

type auditPackBundle struct {
    Metadata     auditPackMetadata      `json:"metadata"`
    Summary      auditPackSummary       `json:"summary"`
    Users        []auditPackUserRow     `json:"users"`
    Attestations []auditPackAttestation `json:"attestations"`
    Certificates []auditPackCertificate `json:"certificates"`
    Attempts     []auditPackAttempt     `json:"attempts"`
}

func GenerateAuditPack(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    var req auditPackRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    from, err := time.Parse(time.RFC3339, req.From)
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_from", "from must be RFC3339")
        return
    }
    to, err := time.Parse(time.RFC3339, req.To)
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_to", "to must be RFC3339")
        return
    }
    if !to.After(from) {
        httpx.Error(w, http.StatusBadRequest, "bad_range", "to must be after from")
        return
    }

    bundle := auditPackBundle{
        Metadata: auditPackMetadata{
            GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
            TenantID:      claims.TenantID.String(),
            TenantSlug:    claims.TenantSlug,
            From:          from.UTC().Format(time.RFC3339),
            To:            to.UTC().Format(time.RFC3339),
            GeneratedBy:   claims.UserEmail,
            SchemaVersion: "1.0",
        },
        Users:        []auditPackUserRow{},
        Attestations: []auditPackAttestation{},
        Certificates: []auditPackCertificate{},
        Attempts:     []auditPackAttempt{},
    }

    // Users snapshot
    users, err := q.ListUserComplianceSnapshot(r.Context())
    if err != nil {
        log.Printf("audit_pack: users: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "users_failed", "")
        return
    }
    for _, u := range users {
        row := auditPackUserRow{
            UserID:             u.UserID.String(),
            Email:              u.Email,
            FullName:           u.FullName,
            Department:         u.Department,
            Status:             u.Status,
            ActiveCertificates: u.ActiveCertificates,
            TotalAttestations:  u.TotalAttestations,
        }
        if u.HiredOn.Valid {
            row.HiredOn = u.HiredOn.Time.Format("2006-01-02")
        }
        bundle.Users = append(bundle.Users, row)
    }

    // Attestations in range
    atts, err := q.ListAttestationsInRange(r.Context(), db.ListAttestationsInRangeParams{
        SignedAt:   from,
        SignedAt_2: to,
    })
    if err != nil {
        log.Printf("audit_pack: attestations: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "attestations_failed", "")
        return
    }
    for _, a := range atts {
        bundle.Attestations = append(bundle.Attestations, auditPackAttestation{
            ID:                a.ID.String(),
            UserID:            a.UserID.String(),
            UserEmail:         a.UserEmail,
            UserFullName:      a.UserFullName,
            RegulationKey:     a.RegulationKey,
            RegulationName:    a.RegulationName,
            RegulationVersion: a.RegulationVersion,
            TextHash:          a.TextHash,
            SignedAt:          a.SignedAt.UTC().Format(time.RFC3339),
        })
    }

    // Certificates in range
    certs, err := q.ListCertificatesInRange(r.Context(), db.ListCertificatesInRangeParams{
        IssuedAt:   from,
        IssuedAt_2: to,
    })
    if err != nil {
        log.Printf("audit_pack: certificates: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "certificates_failed", "")
        return
    }
    for _, c := range certs {
        row := auditPackCertificate{
            ID:           c.ID.String(),
            UserID:       c.UserID.String(),
            UserEmail:    c.UserEmail,
            UserFullName: c.UserFullName,
            CourseTitle:  c.CourseTitle,
            CourseKind:   c.CourseKind,
            VerifyCode:   c.VerifyCode,
            IssuedAt:     c.IssuedAt.UTC().Format(time.RFC3339),
            Revoked:      c.RevokedAt.Valid,
        }
        if c.ExpiresAt.Valid {
            row.ExpiresAt = c.ExpiresAt.Time.Format("2006-01-02")
        }
        bundle.Certificates = append(bundle.Certificates, row)
    }

    // Attempts in range
    attempts, err := q.ListAttemptsInRange(r.Context(), db.ListAttemptsInRangeParams{
        StartedAt:   from,
        StartedAt_2: to,
    })
    if err != nil {
        log.Printf("audit_pack: attempts: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "attempts_failed", "")
        return
    }
    for _, a := range attempts {
        row := auditPackAttempt{
            ID:              a.ID.String(),
            UserEmail:       a.UserEmail,
            UserFullName:    a.UserFullName,
            AssessmentTitle: a.AssessmentTitle,
            AttemptNumber:   a.AttemptNumber,
            Status:          a.Status,
            StartedAt:       a.StartedAt.UTC().Format(time.RFC3339),
            Score:           a.Score,
            Passed:          a.Passed,
        }
        if a.SubmittedAt.Valid {
            row.SubmittedAt = a.SubmittedAt.Time.UTC().Format(time.RFC3339)
        }
        bundle.Attempts = append(bundle.Attempts, row)
    }

    bundle.Summary = auditPackSummary{
        UserCount:        len(bundle.Users),
        AttestationCount: len(bundle.Attestations),
        CertificateCount: len(bundle.Certificates),
        AttemptCount:     len(bundle.Attempts),
    }

    duration := time.Since(start)
    log.Printf("audit_pack: generated for %s [%s → %s] in %s",
        claims.TenantSlug, req.From, req.To, duration)

    filename := fmt.Sprintf("audit-pack-%s-%s-to-%s.json",
        claims.TenantSlug,
        from.UTC().Format("20060102"),
        to.UTC().Format("20060102"))
    w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
    httpx.JSON(w, http.StatusOK, bundle)
}