package testauth

import (
	"context"

	"github.com/rendis/senda/internal/port"
)

// ChainVerifier tries multiple OIDCVerifiers in order,
// returning the first successful result. Used in E2E to support
// both real OIDC tokens (frontend/Keycloak) and HS256 test tokens.
type ChainVerifier struct {
	verifiers []port.OIDCVerifier
}

// NewChainVerifier creates a verifier that tries each verifier in order.
func NewChainVerifier(verifiers ...port.OIDCVerifier) *ChainVerifier {
	return &ChainVerifier{verifiers: verifiers}
}

func (c *ChainVerifier) Verify(ctx context.Context, rawToken string) (*port.OIDCClaims, error) {
	var lastErr error
	for _, v := range c.verifiers {
		claims, err := v.Verify(ctx, rawToken)
		if err == nil {
			return claims, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

var _ port.OIDCVerifier = (*ChainVerifier)(nil)
