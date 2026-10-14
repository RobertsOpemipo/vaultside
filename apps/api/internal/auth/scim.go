package auth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "net/http"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

// GenerateSCIMToken returns (raw, hash, prefix).
func GenerateSCIMToken() (string, string, string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", "", "", err
    }
    raw := "vscim_" + base64.RawURLEncoding.EncodeToString(b)
    sum := sha256.Sum256([]byte(raw))
    hash := hex.EncodeToString(sum[:])
    prefix := raw[:12]
    return raw, hash, prefix, nil
}

// SCIMAuthMiddleware authenticates a SCIM request via Bearer token.
func SCIMAuthMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := r.Header.Get("Authorization")
            if !strings.HasPrefix(h, "Bearer ") {
                writeSCIMUnauthorized(w, "missing bearer token")
                return
            }
            raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
            if raw == "" {
                writeSCIMUnauthorized(w, "empty bearer token")
                return
            }
            sum := sha256.Sum256([]byte(raw))
            hash := hex.EncodeToString(sum[:])

            var tokenID, tenantID uuid.UUID
            var tenantSlug string
            err := pool.QueryRow(r.Context(),
                `SELECT token_id, tenant_id, tenant_slug FROM resolve_scim_token($1)`,
                hash,
            ).Scan(&tokenID, &tenantID, &tenantSlug)
            if err != nil {
                writeSCIMUnauthorized(w, "invalid or revoked token")
                return
            }

            _, _ = pool.Exec(r.Context(),
                `SELECT touch_scim_token($1)`, tokenID)

            claims := &Claims{
                TenantID:    tenantID,
                TenantSlug:  tenantSlug,
                UserID:      uuid.Nil,
                UserEmail:   "scim@system",
                Permissions: []string{"scim.manage"},
            }
            next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
        })
    }
}

func writeSCIMUnauthorized(w http.ResponseWriter, detail string) {
    w.Header().Set("Content-Type", "application/scim+json")
    w.WriteHeader(http.StatusUnauthorized)
    _, _ = w.Write([]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"detail":"` + detail + `","status":"401"}`))
}