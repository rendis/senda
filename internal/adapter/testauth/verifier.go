package testauth

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"github.com/senda-app/senda/internal/port"
)

// TestOIDCVerifier implements port.OIDCVerifier using HS256 JWTs
// for E2E testing without a real OIDC provider.
type TestOIDCVerifier struct {
	secret []byte
}

// NewTestOIDCVerifier creates a verifier that validates HS256-signed JWTs.
func NewTestOIDCVerifier(secret string) *TestOIDCVerifier {
	return &TestOIDCVerifier{secret: []byte(secret)}
}

// Verify validates a HS256 JWT and returns OIDC claims.
func (v *TestOIDCVerifier) Verify(_ context.Context, rawToken string) (*port.OIDCClaims, error) {
	token, err := jwt.Parse(rawToken, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("testauth: unexpected signing method: %v", t.Header["alg"])
		}
		return v.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, fmt.Errorf("testauth: invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("testauth: invalid claims")
	}

	email, _ := claims["email"].(string)
	subject, _ := claims["sub"].(string)
	issuer, _ := claims["iss"].(string)

	if email == "" {
		return nil, fmt.Errorf("testauth: missing email claim")
	}

	return &port.OIDCClaims{
		Subject: subject,
		Email:   email,
		Issuer:  issuer,
	}, nil
}

// Compile-time interface check.
var _ port.OIDCVerifier = (*TestOIDCVerifier)(nil)
