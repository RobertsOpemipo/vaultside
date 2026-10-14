package auth

import (
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"
)

const (
    OIDCStateCookieName = "vaultside_oidc_state"
    oidcStateTTL        = 10 * time.Minute
)

var (
    ErrStateBadFormat    = errors.New("oidc state: bad format")
    ErrStateBadSignature = errors.New("oidc state: signature mismatch")
    ErrStateExpired      = errors.New("oidc state: expired")
)

// OIDCState is the signed payload carried in a cookie across the OIDC redirect.
type OIDCState struct {
    State        string    `json:"state"`
    Nonce        string    `json:"nonce"`
    TenantID     uuid.UUID `json:"tenant_id"`
    ConnectionID uuid.UUID `json:"connection_id"`
    ReturnTo     string    `json:"return_to,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
}

// NewOIDCState generates state and nonce, and returns the populated struct.
func NewOIDCState(tenantID, connectionID uuid.UUID, returnTo string) (*OIDCState, error) {
    state, err := randomURLToken(24)
    if err != nil {
        return nil, err
    }
    nonce, err := randomURLToken(24)
    if err != nil {
        return nil, err
    }
    return &OIDCState{
        State:        state,
        Nonce:        nonce,
        TenantID:     tenantID,
        ConnectionID: connectionID,
        ReturnTo:     returnTo,
        CreatedAt:    time.Now(),
    }, nil
}

func randomURLToken(n int) (string, error) {
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.RawURLEncoding.EncodeToString(b), nil
}

func signPayload(payload, key []byte) []byte {
    h := hmac.New(sha256.New, key)
    h.Write(payload)
    return h.Sum(nil)
}

// EncodeOIDCState returns base64url(payload).base64url(hmac).
func EncodeOIDCState(s *OIDCState, key []byte) (string, error) {
    payload, err := json.Marshal(s)
    if err != nil {
        return "", err
    }
    sig := signPayload(payload, key)
    return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// DecodeOIDCState verifies signature, checks age, and returns the state.
func DecodeOIDCState(encoded string, key []byte) (*OIDCState, error) {
    parts := strings.SplitN(encoded, ".", 2)
    if len(parts) != 2 {
        return nil, ErrStateBadFormat
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[0])
    if err != nil {
        return nil, ErrStateBadFormat
    }
    sig, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return nil, ErrStateBadFormat
    }
    expected := signPayload(payload, key)
    if !hmac.Equal(sig, expected) {
        return nil, ErrStateBadSignature
    }
    var s OIDCState
    if err := json.Unmarshal(payload, &s); err != nil {
        return nil, ErrStateBadFormat
    }
    if time.Since(s.CreatedAt) > oidcStateTTL {
        return nil, ErrStateExpired
    }
    return &s, nil
}

// SetStateCookie writes the state cookie with conservative attributes.
func SetStateCookie(w http.ResponseWriter, encoded string, secure bool) {
    http.SetCookie(w, &http.Cookie{
        Name:     OIDCStateCookieName,
        Value:    encoded,
        Path:     "/v1/auth/oidc",
        MaxAge:   int(oidcStateTTL.Seconds()),
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteLaxMode,
    })
}

// ClearStateCookie expires the state cookie on the client.
func ClearStateCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name:     OIDCStateCookieName,
        Value:    "",
        Path:     "/v1/auth/oidc",
        MaxAge:   -1,
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
    })
}

// ReadStateCookie returns the raw cookie value, if present.
func ReadStateCookie(r *http.Request) (string, error) {
    c, err := r.Cookie(OIDCStateCookieName)
    if err != nil {
        return "", err
    }
    return c.Value, nil
}