package scorm

import (
    "testing"
    "time"

    "github.com/google/uuid"
)

func TestSignParseRoundTrip(t *testing.T) {
    secret := []byte("a-very-secret-key-that-is-long-enough")
    c := AttemptClaims{
        AttemptID: uuid.New(),
        TenantID:  uuid.New(),
        UserID:    uuid.New(),
        ExpiresAt: time.Now().Add(time.Hour).Unix(),
    }
    tok := SignAttemptToken(secret, c)
    got, err := ParseAttemptToken(secret, tok, time.Now())
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    if got.AttemptID != c.AttemptID || got.TenantID != c.TenantID || got.UserID != c.UserID {
        t.Fatalf("claims mismatch: %+v vs %+v", got, c)
    }
}

func TestParseRejectsTamperedSignature(t *testing.T) {
    secret := []byte("a-very-secret-key-that-is-long-enough")
    c := AttemptClaims{AttemptID: uuid.New(), TenantID: uuid.New(), UserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
    tok := SignAttemptToken(secret, c)
    tampered := tok[:len(tok)-2] + "AA"
    if _, err := ParseAttemptToken(secret, tampered, time.Now()); err == nil {
        t.Fatal("expected signature failure")
    }
}

func TestParseRejectsExpired(t *testing.T) {
    secret := []byte("k")
    c := AttemptClaims{AttemptID: uuid.New(), TenantID: uuid.New(), UserID: uuid.New(), ExpiresAt: time.Now().Add(-time.Minute).Unix()}
    tok := SignAttemptToken(secret, c)
    if _, err := ParseAttemptToken(secret, tok, time.Now()); err != ErrExpired {
        t.Fatalf("expected ErrExpired, got %v", err)
    }
}

func TestParseRejectsMalformed(t *testing.T) {
    for _, s := range []string{"", "nope", "a.b", ".", "abc."} {
        if _, err := ParseAttemptToken([]byte("k"), s, time.Now()); err == nil {
            t.Errorf("expected error for %q", s)
        }
    }
}