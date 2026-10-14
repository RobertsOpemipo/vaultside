package auth

import (
    "context"

    "github.com/google/uuid"
)

type Claims struct {
    TenantID    uuid.UUID
    TenantSlug  string
    UserID      uuid.UUID
    UserEmail   string
    SessionID   uuid.UUID
    UserRoles   []string
    Permissions []string
}

func (c *Claims) Has(permission string) bool {
    for _, p := range c.Permissions {
        if p == permission {
            return true
        }
    }
    return false
}

type ctxKey int

const claimsKey ctxKey = 1

func WithClaims(ctx context.Context, c *Claims) context.Context {
    return context.WithValue(ctx, claimsKey, c)
}

func FromContext(ctx context.Context) (*Claims, bool) {
    c, ok := ctx.Value(claimsKey).(*Claims)
    return c, ok
}

func MustFromContext(ctx context.Context) *Claims {
    c, ok := FromContext(ctx)
    if !ok {
        panic("auth: no claims in context")
    }
    return c
}