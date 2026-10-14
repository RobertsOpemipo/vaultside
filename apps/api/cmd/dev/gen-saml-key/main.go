// Command gen-saml-key generates a self-signed RSA keypair for SAML SP use
// and prints base64-encoded PEM values for SAML_CERT_B64 and SAML_KEY_B64.
//
// Usage:
//   cd apps/api
//   go run ./cmd/dev/gen-saml-key
package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/base64"
    "encoding/pem"
    "fmt"
    "log"
    "math/big"
    "time"
)

func main() {
    key, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        log.Fatalf("rsa: %v", err)
    }

    tmpl := x509.Certificate{
        SerialNumber: big.NewInt(time.Now().UnixNano()),
        Subject: pkix.Name{
            CommonName:   "vaultside-saml-sp",
            Organization: []string{"Vaultside"},
        },
        NotBefore:             time.Now().Add(-1 * time.Hour),
        NotAfter:              time.Now().AddDate(10, 0, 0),
        KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
        ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
        BasicConstraintsValid: true,
        IsCA:                  false,
        DNSNames:              []string{"localhost"},
    }

    der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
    if err != nil {
        log.Fatalf("create cert: %v", err)
    }
    certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

    keyDER, err := x509.MarshalPKCS8PrivateKey(key)
    if err != nil {
        log.Fatalf("marshal key: %v", err)
    }
    keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

    fmt.Println("SAML_CERT_B64=" + base64.StdEncoding.EncodeToString(certPEM))
    fmt.Println("SAML_KEY_B64=" + base64.StdEncoding.EncodeToString(keyPEM))
}