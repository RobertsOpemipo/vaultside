package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type Config struct {
    Env           string
    Port          string
    DatabaseURL   string
    DevAuth       bool
    SessionSecret string
    IdleTimeout   time.Duration
    AbsoluteTTL   time.Duration
    AppBaseURL    string
    APIBaseURL    string

    S3Endpoint  string
    S3Region    string
    S3Bucket    string
    S3AccessKey string
    S3SecretKey string

    SSOSecretKeyB64 string
    SAMLCertB64     string
    SAMLKeyB64      string
}

func Load() (Config, error) {
    LoadDotEnv()

    c := Config{
        Env:           getenv("APP_ENV", "development"),
        Port:          getenv("PORT", "8080"),
        DatabaseURL:   os.Getenv("DATABASE_URL"),
        DevAuth:       getbool("DEV_AUTH", true),
        SessionSecret: os.Getenv("SESSION_SECRET"),
        IdleTimeout:   12 * time.Hour,
        AbsoluteTTL:   7 * 24 * time.Hour,
        AppBaseURL:    getenv("APP_BASE_URL", "http://localhost:3000"),
        APIBaseURL:    getenv("API_BASE_URL", "http://localhost:8080"),

        S3Endpoint:  getenv("S3_ENDPOINT", "http://localhost:9000"),
        S3Region:    getenv("S3_REGION", "us-east-1"),
        S3Bucket:    getenv("S3_BUCKET", "vaultside-dev"),
        S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
        S3SecretKey: os.Getenv("S3_SECRET_KEY"),

        SSOSecretKeyB64: os.Getenv("SSO_SECRET_KEY"),
        SAMLCertB64:     os.Getenv("SAML_CERT_B64"),
        SAMLKeyB64:      os.Getenv("SAML_KEY_B64"),
    }
    if c.DatabaseURL == "" {
        return c, fmt.Errorf("DATABASE_URL is required")
    }
    if c.Env == "production" && c.DevAuth {
        return c, fmt.Errorf("DEV_AUTH must be false in production")
    }
    if c.S3AccessKey == "" || c.S3SecretKey == "" {
        return c, fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required")
    }
    if c.SSOSecretKeyB64 == "" {
        return c, fmt.Errorf("SSO_SECRET_KEY is required")
    }
    if c.SessionSecret == "" {
        return c, fmt.Errorf("SESSION_SECRET is required")
    }
    if c.SAMLCertB64 == "" || c.SAMLKeyB64 == "" {
        return c, fmt.Errorf("SAML_CERT_B64 and SAML_KEY_B64 are required")
    }
    return c, nil
}

func getenv(k, def string) string {
    if v := os.Getenv(k); v != "" {
        return v
    }
    return def
}

func getbool(k string, def bool) bool {
    v := os.Getenv(k)
    if v == "" {
        return def
    }
    b, err := strconv.ParseBool(v)
    if err != nil {
        return def
    }
    return b
}