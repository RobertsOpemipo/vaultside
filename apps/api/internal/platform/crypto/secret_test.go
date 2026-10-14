package crypto

import (
    "bytes"
    "encoding/base64"
    "strings"
    "testing"
)

func TestRoundTrip(t *testing.T) {
    key := bytes.Repeat([]byte{0x42}, 32)
    cases := []string{
        "",
        "hunter2",
        strings.Repeat("x", 1000),
        "with\nnewlines\tand\ttabs",
        "unicode: 日本語 🔐",
    }
    for _, plaintext := range cases {
        sealed, err := Encrypt([]byte(plaintext), key)
        if err != nil {
            t.Fatalf("encrypt %q: %v", plaintext, err)
        }
        got, err := Decrypt(sealed, key)
        if err != nil {
            t.Fatalf("decrypt %q: %v", plaintext, err)
        }
        if string(got) != plaintext {
            t.Errorf("round trip mismatch: got %q want %q", got, plaintext)
        }
    }
}

func TestEncryptIsNonDeterministic(t *testing.T) {
    key := bytes.Repeat([]byte{0x42}, 32)
    a, _ := Encrypt([]byte("same input"), key)
    b, _ := Encrypt([]byte("same input"), key)
    if bytes.Equal(a, b) {
        t.Error("two encryptions of the same plaintext produced identical output")
    }
}

func TestDecryptWrongKeyFails(t *testing.T) {
    key1 := bytes.Repeat([]byte{0x42}, 32)
    key2 := bytes.Repeat([]byte{0x43}, 32)
    sealed, _ := Encrypt([]byte("secret"), key1)
    if _, err := Decrypt(sealed, key2); err == nil {
        t.Error("decrypt with wrong key succeeded")
    }
}

func TestDecryptTamperedFails(t *testing.T) {
    key := bytes.Repeat([]byte{0x42}, 32)
    sealed, _ := Encrypt([]byte("secret"), key)
    sealed[len(sealed)-1] ^= 0x01
    if _, err := Decrypt(sealed, key); err == nil {
        t.Error("decrypt of tampered ciphertext succeeded")
    }
}

func TestDecryptShortInputFails(t *testing.T) {
    key := bytes.Repeat([]byte{0x42}, 32)
    if _, err := Decrypt([]byte{0x01}, key); err == nil {
        t.Error("decrypt of short input succeeded")
    }
}

func TestDecryptUnknownVersionFails(t *testing.T) {
    key := bytes.Repeat([]byte{0x42}, 32)
    sealed, _ := Encrypt([]byte("secret"), key)
    sealed[0] = 0xFF
    if _, err := Decrypt(sealed, key); err == nil {
        t.Error("decrypt with unknown version succeeded")
    }
}

func TestParseKey(t *testing.T) {
    good := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x01}, 32))
    if _, err := ParseKey(good); err != nil {
        t.Errorf("good key rejected: %v", err)
    }
    bad := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x01}, 16))
    if _, err := ParseKey(bad); err == nil {
        t.Error("16-byte key accepted")
    }
    if _, err := ParseKey("not!valid!base64"); err == nil {
        t.Error("invalid base64 accepted")
    }
}

func TestInvalidKeySizeRejected(t *testing.T) {
    if _, err := Encrypt([]byte("x"), bytes.Repeat([]byte{0x01}, 16)); err != ErrInvalidKey {
        t.Errorf("Encrypt accepted 16-byte key: %v", err)
    }
    if _, err := Decrypt([]byte{0x01}, bytes.Repeat([]byte{0x01}, 16)); err != ErrInvalidKey {
        t.Errorf("Decrypt accepted 16-byte key: %v", err)
    }
}