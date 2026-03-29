package oidcauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/rendis/senda/internal/port"
)

// OIDCVerifier implements port.OIDCVerifier using a real OIDC provider (e.g. Keycloak).
// It verifies RS256-signed JWTs using the provider's JWKS endpoint.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// New creates a real OIDC verifier by performing provider discovery.
// discoveryURL is the full .well-known URL or the issuer URL.
// clientID is used for audience validation.
// skipIssuerCheck controls whether issuer validation is skipped (useful when
// the backend reaches the OIDC provider via an internal hostname that differs
// from the JWT issuer URL, e.g. Docker networking).
func New(ctx context.Context, discoveryURL, clientID string, skipIssuerCheck bool) (*OIDCVerifier, error) {
	issuer := strings.TrimSuffix(discoveryURL, "/.well-known/openid-configuration")

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidcauth: provider discovery failed: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID:        clientID,
		SkipIssuerCheck: skipIssuerCheck,
	})

	return &OIDCVerifier{verifier: verifier}, nil
}

// Verify validates a JWT from the OIDC provider and returns claims.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (*port.OIDCClaims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("oidcauth: token verification failed: %w", err)
	}

	var claims struct {
		Email   string `json:"email"`
		Subject string `json:"sub"`
		Issuer  string `json:"iss"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidcauth: failed to extract claims: %w", err)
	}

	if claims.Email == "" {
		return nil, fmt.Errorf("oidcauth: missing email claim")
	}

	return &port.OIDCClaims{
		Subject: idToken.Subject,
		Email:   claims.Email,
		Issuer:  claims.Issuer,
	}, nil
}

var _ port.OIDCVerifier = (*OIDCVerifier)(nil)
