package handlers

import (
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "net/http"
    "path/filepath"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/db"
    "github.com/vaultside/vaultside/apps/api/internal/httpx"
    "github.com/vaultside/vaultside/apps/api/internal/platform/antivirus"
    "github.com/vaultside/vaultside/apps/api/internal/platform/storage"
)

type UploadHandler struct {
    Storage *storage.Client
    Scanner antivirus.Scanner
    PutTTL  time.Duration
    GetTTL  time.Duration
}

func NewUploadHandler(s *storage.Client, scan antivirus.Scanner) *UploadHandler {
    return &UploadHandler{
        Storage: s,
        Scanner: scan,
        PutTTL:  15 * time.Minute,
        GetTTL:  5 * time.Minute,
    }
}

var allowedUploadKinds = map[string]string{
    "video": "video/",
    "pdf":   "application/pdf",
    "image": "image/",
    "audio": "audio/",
}

var allowedMimes = map[string]bool{
    "video/mp4":       true,
    "video/webm":      true,
    "application/pdf": true,
    "image/png":       true,
    "image/jpeg":      true,
    "image/svg+xml":   true,
    "audio/mpeg":      true,
    "audio/wav":       true,
}

type presignRequest struct {
    Filename  string `json:"filename"`
    MimeType  string `json:"mime_type"`
    SizeBytes int64  `json:"size_bytes"`
    Kind      string `json:"kind"`
}

type presignResponse struct {
    AssetID    string `json:"asset_id"`
    UploadURL  string `json:"upload_url"`
    StorageKey string `json:"storage_key"`
    ExpiresAt  string `json:"expires_at"`
}

func (h *UploadHandler) Presign(w http.ResponseWriter, r *http.Request) {
    claims := auth.MustFromContext(r.Context())
    q := httpx.MustQueries(r.Context())

    var req presignRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_json", "")
        return
    }
    if req.Filename == "" || req.MimeType == "" || req.Kind == "" {
        httpx.Error(w, http.StatusBadRequest, "missing_fields", "filename, mime_type, kind required")
        return
    }
    if !allowedMimes[req.MimeType] {
        httpx.Error(w, http.StatusBadRequest, "bad_mime", "mime type not allowed")
        return
    }
    if _, ok := allowedUploadKinds[req.Kind]; !ok {
        httpx.Error(w, http.StatusBadRequest, "bad_kind", "unknown kind")
        return
    }
    if req.SizeBytes <= 0 || req.SizeBytes > 5*1024*1024*1024 {
        httpx.Error(w, http.StatusBadRequest, "bad_size", "size must be between 1 byte and 5 GB")
        return
    }

    assetID := uuid.Must(uuid.NewV7())
    safeName := strings.ReplaceAll(filepath.Base(req.Filename), " ", "_")
    storageKey := fmt.Sprintf("tenants/%s/assets/%s/%s", claims.TenantID, assetID, safeName)

    _, err := q.CreateContentAsset(r.Context(), db.CreateContentAssetParams{
        ID:         assetID,
        TenantID:   claims.TenantID,
        Kind:       req.Kind,
        StorageKey: storageKey,
        MimeType:   req.MimeType,
        SizeBytes:  req.SizeBytes,
        DurationS:  nil,
        UploadedBy: pgtype.UUID{Bytes: claims.UserID, Valid: true},
    })
    if err != nil {
        log.Printf("uploads: create asset: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "asset_create_failed", "")
        return
    }

    url, err := h.Storage.PresignPut(r.Context(), storageKey, req.MimeType, h.PutTTL)
    if err != nil {
        log.Printf("uploads: presign: %v", err)
        httpx.Error(w, http.StatusInternalServerError, "presign_failed", "")
        return
    }

    httpx.JSON(w, http.StatusOK, presignResponse{
        AssetID:    assetID.String(),
        UploadURL:  url,
        StorageKey: storageKey,
        ExpiresAt:  time.Now().Add(h.PutTTL).Format(time.RFC3339),
    })
}

func (h *UploadHandler) Complete(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())

    assetID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid asset id")
        return
    }

    asset, err := q.GetContentAsset(r.Context(), assetID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "asset not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }

    head, err := h.Storage.Head(r.Context(), asset.StorageKey)
    if err != nil {
        log.Printf("uploads: head %s: %v", asset.StorageKey, err)
        httpx.Error(w, http.StatusConflict, "not_uploaded", "object not found in storage")
        return
    }

    size := int64(0)
    if head.ContentLength != nil {
        size = *head.ContentLength
    }

    res, err := h.Scanner.Scan(r.Context(), asset.StorageKey, size)
    if err != nil {
        _ = q.MarkAssetClean(r.Context(), db.MarkAssetCleanParams{
            ID:              assetID,
            TranscodeStatus: "failed",
        })
        httpx.Error(w, http.StatusInternalServerError, "scan_failed", "")
        return
    }

    switch res.Status {
    case "clean":
        transcode := "n/a"
        if asset.Kind == "video" {
            transcode = "pending"
        }
        if err := q.MarkAssetClean(r.Context(), db.MarkAssetCleanParams{
            ID:              assetID,
            TranscodeStatus: transcode,
        }); err != nil {
            httpx.Error(w, http.StatusInternalServerError, "mark_failed", "")
            return
        }
        httpx.JSON(w, http.StatusOK, map[string]any{
            "asset_id":         assetID.String(),
            "scan_status":      "clean",
            "transcode_status": transcode,
            "size_bytes":       size,
        })
    default:
        httpx.Error(w, http.StatusUnprocessableEntity, "scan_rejected", "asset failed scanning")
    }
}

func (h *UploadHandler) AssetURL(w http.ResponseWriter, r *http.Request) {
    q := httpx.MustQueries(r.Context())
    assetID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        httpx.Error(w, http.StatusBadRequest, "bad_id", "invalid asset id")
        return
    }
    asset, err := q.GetContentAsset(r.Context(), assetID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            httpx.Error(w, http.StatusNotFound, "not_found", "asset not found")
            return
        }
        httpx.Error(w, http.StatusInternalServerError, "get_failed", "")
        return
    }
    if asset.ScanStatus != "clean" {
        httpx.Error(w, http.StatusConflict, "asset_not_ready", "asset is not available")
        return
    }
    url, err := h.Storage.PresignGet(r.Context(), asset.StorageKey, h.GetTTL)
    if err != nil {
        httpx.Error(w, http.StatusInternalServerError, "presign_failed", "")
        return
    }
    httpx.JSON(w, http.StatusOK, map[string]any{
        "asset_id":   assetID.String(),
        "url":        url,
        "expires_in": int(h.GetTTL.Seconds()),
    })
}