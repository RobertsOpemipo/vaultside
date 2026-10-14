package auth

import (
    "crypto/rsa"
    "crypto/x509"
    "encoding/base64"
    "encoding/pem"
    "fmt"
)

type SAMLKeypair struct {
    Certificate *x509.Certificate
    PrivateKey  *rsa.PrivateKey
}

func ParseSAMLKeypair(certB64, keyB64 string) (*SAMLKeypair, error) {
    certPEM, err := base64.StdEncoding.DecodeString(certB64)
    if err != nil {
        return nil, fmt.Errorf("saml keypair: decode cert b64: %w", err)
    }
    keyPEM, err := base64.StdEncoding.DecodeString(keyB64)
    if err != nil {
        return nil, fmt.Errorf("saml keypair: decode key b64: %w", err)
    }

    certBlock, _ := pem.Decode(certPEM)
    if certBlock == nil || certBlock.Type != "CERTIFICATE" {
        return nil, fmt.Errorf("saml keypair: cert PEM invalid")
    }
    cert, err := x509.ParseCertificate(certBlock.Bytes)
    if err != nil {
        return nil, fmt.Errorf("saml keypair: parse cert: %w", err)
    }

    keyBlock, _ := pem.Decode(keyPEM)
    if keyBlock == nil {
        return nil, fmt.Errorf("saml keypair: key PEM invalid")
    }
    var priv *rsa.PrivateKey
    switch keyBlock.Type {
    case "PRIVATE KEY":
        k, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
        if err != nil {
            return nil, fmt.Errorf("saml keypair: parse pkcs8: %w", err)
        }
        var ok bool
        priv, ok = k.(*rsa.PrivateKey)
        if !ok {
            return nil, fmt.Errorf("saml keypair: pkcs8 key is not RSA")
        }
    case "RSA PRIVATE KEY":
        k, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
        if err != nil {
            return nil, fmt.Errorf("saml keypair: parse pkcs1: %w", err)
        }
        priv = k
    default:
        return nil, fmt.Errorf("saml keypair: unsupported PEM type %q", keyBlock.Type)
    }

    return &SAMLKeypair{Certificate: cert, PrivateKey: priv}, nil
}