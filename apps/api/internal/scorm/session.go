package scorm

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
)

// AttemptClaims identifies a single SCORM launch session. The token is
// HMAC-signed and short-lived; it carries the tenant and user so the
// cookie-less shim can authenticate without a session cookie.
type AttemptClaims struct {
    AttemptID uuid.UUID `json:"aid"`
    TenantID  uuid.UUID `json:"tid"`
    UserID    uuid.UUID `json:"uid"`
    ExpiresAt int64     `json:"exp"` // unix seconds
}

var (
    ErrBadToken     = errors.New("scorm: malformed token")
    ErrBadSignature = errors.New("scorm: bad signature")
    ErrExpired      = errors.New("scorm: token expired")
)

// SignAttemptToken returns a compact "payload.sig" token.
func SignAttemptToken(secret []byte, c AttemptClaims) string {
    body, _ := json.Marshal(c)
    enc := base64.RawURLEncoding.EncodeToString(body)
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(enc))
    sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
    return enc + "." + sig
}

// ParseAttemptToken verifies signature and expiry.
func ParseAttemptToken(secret []byte, token string, now time.Time) (*AttemptClaims, error) {
    dot := -1
    for i := 0; i < len(token); i++ {
        if token[i] == '.' {
            dot = i
            break
        }
    }
    if dot < 1 || dot == len(token)-1 {
        return nil, ErrBadToken
    }
    enc, sig := token[:dot], token[dot+1:]
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(enc))
    want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
    if !hmac.Equal([]byte(want), []byte(sig)) {
        return nil, ErrBadSignature
    }
    body, err := base64.RawURLEncoding.DecodeString(enc)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrBadToken, err)
    }
    var c AttemptClaims
    if err := json.Unmarshal(body, &c); err != nil {
        return nil, fmt.Errorf("%w: %v", ErrBadToken, err)
    }
    if c.ExpiresAt > 0 && now.Unix() > c.ExpiresAt {
        return nil, ErrExpired
    }
    return &c, nil
}