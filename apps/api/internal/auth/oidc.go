package auth

import (
    "context"
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/coreos/go-oidc/v3/oidc"
    "golang.org/x/oauth2"
)

// providerCache is a package-level cache of OIDC providers keyed by issuer URL.
// Discovery docs are fetched once per issuer per process, then reused for an hour.
type cachedProvider struct {
    provider *oidc.Provider
    cachedAt time.Time
}

var (
    providerCache sync.Map
    providerTTL   = 1 * time.Hour
)

// ResolveOIDCProvider returns a cached provider for the given issuer, doing
// discovery on first use or after the TTL expires.
func ResolveOIDCProvider(ctx context.Context, issuer string) (*oidc.Provider, error) {
    issuer = strings.TrimRight(issuer, "/")
    if v, ok := providerCache.Load(issuer); ok {
        cp := v.(*cachedProvider)
        if time.Since(cp.cachedAt) < providerTTL {
            return cp.provider, nil
        }
    }
    p, err := oidc.NewProvider(ctx, issuer)
    if err != nil {
        return nil, fmt.Errorf("oidc discovery for %s: %w", issuer, err)
    }
    providerCache.Store(issuer, &cachedProvider{provider: p, cachedAt: time.Now()})
    return p, nil
}

// BuildOAuth2Config constructs a fresh oauth2.Config for a connection.
func BuildOAuth2Config(provider *oidc.Provider, clientID, clientSecret, redirectURL, scopes string) *oauth2.Config {
    fields := strings.Fields(scopes)
    if len(fields) == 0 {
        fields = []string{oidc.ScopeOpenID, "email", "profile"}
    }
    return &oauth2.Config{
        ClientID:     clientID,
        ClientSecret: clientSecret,
        RedirectURL:  redirectURL,
        Endpoint:     provider.Endpoint(),
        Scopes:       fields,
    }
}

// IDTokenClaims is the subset of ID token claims we care about.
type IDTokenClaims struct {
    Subject           string `json:"sub"`
    Email             string `json:"email"`
    EmailVerified     bool   `json:"email_verified"`
    Name              string `json:"name"`
    PreferredUsername string `json:"preferred_username"`
}

// VerifyIDToken verifies the signature, audience, issuer, and nonce; returns claims.
func VerifyIDToken(ctx context.Context, provider *oidc.Provider, clientID, rawIDToken, expectedNonce string) (*IDTokenClaims, error) {
    verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
    tok, err := verifier.Verify(ctx, rawIDToken)
    if err != nil {
        return nil, fmt.Errorf("verify id token: %w", err)
    }
    if tok.Nonce != expectedNonce {
        return nil, fmt.Errorf("id token nonce mismatch")
    }
    var claims IDTokenClaims
    if err := tok.Claims(&claims); err != nil {
        return nil, fmt.Errorf("parse id token claims: %w", err)
    }
    return &claims, nil
}