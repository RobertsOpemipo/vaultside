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
    SAMLStateCookieName = "vaultside_saml_state"
    samlStateTTL        = 10 * time.Minute
)

var (
    ErrSAMLStateBadFormat    = errors.New("saml state: bad format")
    ErrSAMLStateBadSignature = errors.New("saml state: signature mismatch")
    ErrSAMLStateExpired      = errors.New("saml state: expired")
)

type SAMLState struct {
    RequestID    string    `json:"request_id"`
    RelayState   string    `json:"relay_state"`
    TenantID     uuid.UUID `json:"tenant_id"`
    ConnectionID uuid.UUID `json:"connection_id"`
    ReturnTo     string    `json:"return_to,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
}

func NewSAMLState(tenantID, connectionID uuid.UUID, returnTo string) (*SAMLState, error) {
    reqID, err := randomURLToken(24)
    if err != nil {
        return nil, err
    }
    relay, err := randomURLToken(24)
    if err != nil {
        return nil, err
    }
    return &SAMLState{
        RequestID:    "_" + reqID,
        RelayState:   relay,
        TenantID:     tenantID,
        ConnectionID: connectionID,
        ReturnTo:     returnTo,
        CreatedAt:    time.Now(),
    }, nil
}

func signSAMLPayload(payload, key []byte) []byte {
    h := hmac.New(sha256.New, key)
    h.Write(payload)
    return h.Sum(nil)
}

func EncodeSAMLState(s *SAMLState, key []byte) (string, error) {
    payload, err := json.Marshal(s)
    if err != nil {
        return "", err
    }
    sig := signSAMLPayload(payload, key)
    return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func DecodeSAMLState(encoded string, key []byte) (*SAMLState, error) {
    parts := strings.SplitN(encoded, ".", 2)
    if len(parts) != 2 {
        return nil, ErrSAMLStateBadFormat
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[0])
    if err != nil {
        return nil, ErrSAMLStateBadFormat
    }
    sig, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return nil, ErrSAMLStateBadFormat
    }
    expected := signSAMLPayload(payload, key)
    if !hmac.Equal(sig, expected) {
        return nil, ErrSAMLStateBadSignature
    }
    var s SAMLState
    if err := json.Unmarshal(payload, &s); err != nil {
        return nil, ErrSAMLStateBadFormat
    }
    if time.Since(s.CreatedAt) > samlStateTTL {
        return nil, ErrSAMLStateExpired
    }
    return &s, nil
}

func SetSAMLStateCookie(w http.ResponseWriter, encoded string, secure bool) {
    http.SetCookie(w, &http.Cookie{
        Name:     SAMLStateCookieName,
        Value:    encoded,
        Path:     "/v1/auth/saml",
        MaxAge:   int(samlStateTTL.Seconds()),
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteLaxMode,
    })
}

func ClearSAMLStateCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name:     SAMLStateCookieName,
        Value:    "",
        Path:     "/v1/auth/saml",
        MaxAge:   -1,
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
    })
}

func ReadSAMLStateCookie(r *http.Request) (string, error) {
    c, err := r.Cookie(SAMLStateCookieName)
    if err != nil {
        return "", err
    }
    return c.Value, nil
}

var _ = rand.Read