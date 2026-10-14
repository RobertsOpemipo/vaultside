package handlers

import (
    "encoding/json"
    "errors"
    "net"
    "net/http"
    "net/netip"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
)

// ---------------- Regulations ----------------

type regulationItem struct {
    ID          string `json:"id"`
    Key         string `json:"key"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Authority   string `json:"authority"`
    Version     int32  `json:"version"`
}

func ListRegulations(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListRegulations(r.Context())
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]regulationItem, 0, len(rows))
    for _, row := range rows {
        out = append(out, regulationItem{
            ID:          row.ID.String(),
            Key:         row.Key,
            Name:        row.Name,
            Description: row.Description,
            Authority:   row.Authority,
            Version:     row.Version,
        })
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"regulations": out})
}

// ---------------- Attestations ----------------

type attestationItem struct {
    ID                string `json:"id"`
    RegulationID      string `json:"regulation_id"`
    RegulationKey     string `json:"regulation_key"`
    RegulationName    string `json:"regulation_name"`
    RegulationVersion int32  `json:"regulation_version"`
    TextHash          string `json:"text_hash"`
    SignedAt          string `json:"signed_at"`
}

func ListMyAttestations(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListAttestationsByUser(r.Context(), claims.UserID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]attestationItem, 0, len(rows))
    for _, row := range rows {
        out = append(out, attestationItem{
            ID:                row.ID.String(),
            RegulationID:      row.RegulationID.String(),
            RegulationKey:     row.RegulationKey,
            RegulationName:    row.RegulationName,
            RegulationVersion: row.RegulationVersion,
            TextHash:          row.TextHash,
            SignedAt:          row.SignedAt.Format("2006-01-02T15:04:05Z07:00"),
        })
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"attestations": out})
}

type signAttestationRequest struct {
    TextHash string `json:"text_hash"`
}

type signAttestationResponse struct {
    ID                string `json:"id"`
    RegulationKey     string `json:"regulation_key"`
    RegulationVersion int32  `json:"regulation_version"`
    SignedAt          string `json:"signed_at"`
}

func SignAttestation(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    regID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid regulation id")
        return
    }

    reg, err := q.GetRegulation(r.Context(), regID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "regulation not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    var req signAttestationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    if req.TextHash == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_text_hash", "text_hash is required")
        return
    }

    // Idempotent: if the user already attested to this exact version, return it.
    n, err := q.CountAttestationsForUserRegulation(r.Context(),
        db.CountAttestationsForUserRegulationParams{
            UserID:            claims.UserID,
            RegulationID:      reg.ID,
            RegulationVersion: reg.Version,
        })
    if err == nil && n > 0 {
        httpx.Error(w, http.StatusConflict, "already_attested",
            "you have already attested to this version")
        return
    }

    var ipPtr *netip.Addr
    if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
        if ip, err := netip.ParseAddr(host); err == nil {
            ipPtr = &ip
        }
    }
    var ua *string
    if x := r.UserAgent(); x != "" {
        ua = &x
    }

    row, err := q.CreateAttestation(r.Context(), db.CreateAttestationParams{
        ID:                uuid.Must(uuid.NewV7()),
        TenantID:          claims.TenantID,
        UserID:            claims.UserID,
        RegulationID:      reg.ID,
        RegulationVersion: reg.Version,
        TextHash:          req.TextHash,
        Ip:                ipPtr,
        UserAgent:         ua,
    })
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "attest_failed", "")
        return
    }

    httpx.JSON(w, http.StatusOK, signAttestationResponse{
        ID:                row.ID.String(),
        RegulationKey:     reg.Key,
        RegulationVersion: reg.Version,
        SignedAt:          row.SignedAt.Format("2006-01-02T15:04:05Z07:00"),
    })
}

// ---------------- My certificates ----------------

type certificateItem struct {
    ID              string `json:"id"`
    CourseID        string `json:"course_id"`
    CourseVersionID string `json:"course_version_id"`
    IssuedAt        string `json:"issued_at"`
    ExpiresAt       string `json:"expires_at,omitempty"`
    VerifyCode      string `json:"verify_code"`
    Revoked         bool   `json:"revoked"`
}

func ListMyCertificates(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())
    rows, err := q.ListCertificatesForUser(r.Context(), claims.UserID)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "list_failed", "")
        return
    }
    out := make([]certificateItem, 0, len(rows))
    for _, row := range rows {
        item := certificateItem{
            ID:              row.ID.String(),
            CourseID:        row.CourseID.String(),
            CourseVersionID: row.CourseVersionID.String(),
            IssuedAt:        row.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
            VerifyCode:      row.VerifyCode,
            Revoked:         row.RevokedAt.Valid,
        }
        if row.ExpiresAt.Valid {
            item.ExpiresAt = row.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00")
        }
        out = append(out, item)
    }
    httpx.JSON(w, http.StatusOK, map[string]any{"certificates": out})
}