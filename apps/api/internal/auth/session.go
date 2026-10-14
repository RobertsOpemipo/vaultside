package auth

import (
    "context"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "errors"
    "fmt"
    "net/http"
    "net/netip"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/db"
)

const (
    SessionCookieName  = "vaultside_session"
    SessionIdleTTL     = 12 * time.Hour
    SessionAbsoluteTTL = 7 * 24 * time.Hour
)

var ErrSessionNotFound = errors.New("auth: session not found")

// GenerateSessionToken returns (raw, hash) where raw is what the client
// receives in a cookie and hash is the hex SHA-256 that we persist.
func GenerateSessionToken() (string, string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", "", fmt.Errorf("auth: token rand: %w", err)
    }
    raw := base64.RawURLEncoding.EncodeToString(b)
    sum := sha256.Sum256([]byte(raw))
    return raw, hex.EncodeToString(sum[:]), nil
}

// HashToken returns the hex SHA-256 of a raw session token.
func HashToken(raw string) string {
    sum := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(sum[:])
}

// CreateSession inserts a new session and returns (sessionID, rawToken, expiresAt).
func CreateSession(
    ctx context.Context,
    q *db.Queries,
    tenantID, userID uuid.UUID,
    ip *netip.Addr,
    userAgent *string,
    ttl time.Duration,
) (uuid.UUID, string, time.Time, error) {
    raw, hash, err := GenerateSessionToken()
    if err != nil {
        return uuid.Nil, "", time.Time{}, err
    }
    expires := time.Now().Add(ttl)
    row, err := q.CreateSession(ctx, db.CreateSessionParams{
        ID:        uuid.Must(uuid.NewV7()),
        TenantID:  tenantID,
        UserID:    userID,
        TokenHash: hash,
        Ip:        ip,
        UserAgent: userAgent,
        ExpiresAt: expires,
    })
    if err != nil {
        return uuid.Nil, "", time.Time{}, err
    }
    return row.ID, raw, row.ExpiresAt, nil
}

// ResolvedSession is what the middleware needs after resolving a cookie.
type ResolvedSession struct {
    SessionID uuid.UUID
    TenantID  uuid.UUID
    UserID    uuid.UUID
    Email     string
    FullName  string
    ExpiresAt time.Time
}

// ResolveSessionFromToken uses the SECURITY DEFINER resolver so it works
// before app.tenant_id is set.
func ResolveSessionFromToken(ctx context.Context, pool *pgxpool.Pool, rawToken string) (*ResolvedSession, error) {
    hash := HashToken(rawToken)

    var (
        sid      uuid.UUID
        tid      uuid.UUID
        uid      uuid.UUID
        expires  time.Time
        email    string
        fullName string
    )
    err := pool.QueryRow(ctx,
        `SELECT session_id, tenant_id, user_id, expires_at, user_email, user_full_name
           FROM resolve_session_by_hash($1)`,
        hash,
    ).Scan(&sid, &tid, &uid, &expires, &email, &fullName)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrSessionNotFound
        }
        return nil, err
    }
    return &ResolvedSession{
        SessionID: sid,
        TenantID:  tid,
        UserID:    uid,
        Email:     email,
        FullName:  fullName,
        ExpiresAt: expires,
    }, nil
}

// SetSessionCookie writes the session cookie with conservative attributes.
// The `secure` flag should be true in production behind HTTPS.
func SetSessionCookie(w http.ResponseWriter, rawToken string, expires time.Time, secure bool) {
    maxAge := int(time.Until(expires).Seconds())
    if maxAge < 1 {
        maxAge = 1
    }
    http.SetCookie(w, &http.Cookie{
        Name:     SessionCookieName,
        Value:    rawToken,
        Path:     "/",
        Expires:  expires,
        MaxAge:   maxAge,
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteLaxMode,
    })
}

// ClearSessionCookie expires the cookie on the client.
func ClearSessionCookie(w http.ResponseWriter) {
    http.SetCookie(w, &http.Cookie{
        Name:     SessionCookieName,
        Value:    "",
        Path:     "/",
        MaxAge:   -1,
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
    })
}