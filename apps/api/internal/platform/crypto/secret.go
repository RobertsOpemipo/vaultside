// Package crypto provides AES-256-GCM encryption for secrets at rest.
//
// Format of the sealed blob: [version=1][12-byte nonce][ciphertext+tag]
// Callers store the raw bytes in a bytea column. Version prefix lets us
// rotate algorithms later without a data migration.
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
)

const (
    versionByte = 0x01
    nonceSize   = 12
    keySize     = 32 // AES-256
)

var (
    ErrInvalidKey     = errors.New("crypto: key must be 32 bytes")
    ErrInvalidSealed  = errors.New("crypto: sealed data too short")
    ErrUnknownVersion = errors.New("crypto: unknown sealed version")
    ErrAuthFailed     = errors.New("crypto: authentication failed")
)

// ParseKey decodes a base64-encoded 32-byte key.
func ParseKey(b64 string) ([]byte, error) {
    k, err := base64.StdEncoding.DecodeString(b64)
    if err != nil {
        return nil, fmt.Errorf("crypto: base64 decode: %w", err)
    }
    if len(k) != keySize {
        return nil, ErrInvalidKey
    }
    return k, nil
}

// Encrypt returns the sealed blob for plaintext.
func Encrypt(plaintext, key []byte) ([]byte, error) {
    if len(key) != keySize {
        return nil, ErrInvalidKey
    }
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("crypto: cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("crypto: gcm: %w", err)
    }
    nonce := make([]byte, nonceSize)
    if _, err := rand.Read(nonce); err != nil {
        return nil, fmt.Errorf("crypto: nonce: %w", err)
    }
    out := make([]byte, 0, 1+nonceSize+len(plaintext)+gcm.Overhead())
    out = append(out, versionByte)
    out = append(out, nonce...)
    out = gcm.Seal(out, nonce, plaintext, nil)
    return out, nil
}

// Decrypt reverses Encrypt. Fails closed on tamper, wrong key, or unknown version.
func Decrypt(sealed, key []byte) ([]byte, error) {
    if len(key) != keySize {
        return nil, ErrInvalidKey
    }
    if len(sealed) < 1+nonceSize {
        return nil, ErrInvalidSealed
    }
    if sealed[0] != versionByte {
        return nil, ErrUnknownVersion
    }
    nonce := sealed[1 : 1+nonceSize]
    ciphertext := sealed[1+nonceSize:]
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("crypto: cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("crypto: gcm: %w", err)
    }
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, ErrAuthFailed
    }
    return plaintext, nil
}