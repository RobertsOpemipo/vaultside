package httpx

import (
    "net/http"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
)

func RequirePermission(perm string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := auth.FromContext(r.Context())
            if !ok {
                Error(w, http.StatusUnauthorized, "unauthorized", "no claims")
                return
            }
            if !claims.Has(perm) {
                Error(w, http.StatusForbidden, "forbidden", "missing permission: "+perm)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}