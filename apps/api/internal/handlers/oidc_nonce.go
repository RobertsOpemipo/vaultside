package handlers

import "golang.org/x/oauth2"

// oidcNonceOption is an oauth2.AuthCodeOption that adds the `nonce` query
// parameter to the authorize URL. golang.org/x/oauth2 doesn't ship one.
func oidcNonceOption(nonce string) oauth2.AuthCodeOption {
    return oauth2.SetAuthURLParam("nonce", nonce)
}