package port

import "context"

// OIDCClaims holds the verified claims extracted from an OIDC token.
type OIDCClaims struct {
	Subject string
	Email   string
	Issuer  string
}

// OIDCVerifier validates OIDC bearer tokens and returns the claims.
type OIDCVerifier interface {
	Verify(ctx context.Context, rawToken string) (*OIDCClaims, error)
}
