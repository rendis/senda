package testauth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken creates a HS256-signed JWT for E2E testing.
func GenerateToken(email, subject, issuer, secret string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   subject,
		"email": email,
		"iss":   issuer,
		"iat":   now.Unix(),
		"exp":   now.Add(expiry).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("testauth: sign token: %w", err)
	}
	return signed, nil
}
