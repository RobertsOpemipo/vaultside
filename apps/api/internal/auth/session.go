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

var (
    ErrSessionNotFound = errors.New("auth: session not found")
    ErrSessionIdle     = errors.New("auth: session idle timeout")
)

func GenerateSessionToken() (string, string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", "", fmt.Errorf("auth: token rand: %w", err)
    }
    raw := base64.RawURLEncoding.EncodeToString(b)
    sum := sha256.Sum256([]byte(raw))
    return raw, hex.EncodeToString(sum[:]), nil
}

func HashToken(raw string) string {
    sum := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(sum[:])
}

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

type ResolvedSession struct {
    SessionID  uuid.UUID
    TenantID   uuid.UUID
    UserID     uuid.UUID
    Email      string
    FullName   string
    ExpiresAt  time.Time
    LastSeenAt time.Time
}

func ResolveSessionFromToken(ctx context.Context, pool *pgxpool.Pool, rawToken string) (*ResolvedSession, error) {
    hash := HashToken(rawToken)

    var (
        sid      uuid.UUID
        tid      uuid.UUID
        uid      uuid.UUID
        expires  time.Time
        lastSeen time.Time
        email    string
        fullName string
    )
    err := pool.QueryRow(ctx,
        `SELECT session_id, tenant_id, user_id, expires_at, last_seen_at, user_email, user_full_name
           FROM resolve_session_by_hash($1)`,
        hash,
    ).Scan(&sid, &tid, &uid, &expires, &lastSeen, &email, &fullName)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrSessionNotFound
        }
        return nil, err
    }
    return &ResolvedSession{
        SessionID:  sid,
        TenantID:   tid,
        UserID:     uid,
        Email:      email,
        FullName:   fullName,
        ExpiresAt:  expires,
        LastSeenAt: lastSeen,
    }, nil
}

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