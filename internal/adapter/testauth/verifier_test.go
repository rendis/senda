package testauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-at-least-32-characters-long"

func TestGenerateAndVerify_RoundTrip(t *testing.T) {
	token, err := GenerateToken("admin@test.com", "user-123", "senda-test", testSecret, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	v := NewTestOIDCVerifier(testSecret)
	claims, err := v.Verify(context.Background(), token)
	require.NoError(t, err)

	require.Equal(t, "admin@test.com", claims.Email)
	require.Equal(t, "user-123", claims.Subject)
	require.Equal(t, "senda-test", claims.Issuer)
}

func TestVerify_ExpiredToken(t *testing.T) {
	token, err := GenerateToken("admin@test.com", "user-123", "senda-test", testSecret, -time.Hour)
	require.NoError(t, err)

	v := NewTestOIDCVerifier(testSecret)
	_, err = v.Verify(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token is expired")
}

func TestVerify_InvalidSignature(t *testing.T) {
	token, err := GenerateToken("admin@test.com", "user-123", "senda-test", testSecret, time.Hour)
	require.NoError(t, err)

	v := NewTestOIDCVerifier("wrong-secret-that-is-long-enough")
	_, err = v.Verify(context.Background(), token)
	require.Error(t, err)
}

func TestVerify_MissingEmail(t *testing.T) {
	token, err := GenerateToken("", "user-123", "senda-test", testSecret, time.Hour)
	require.NoError(t, err)

	v := NewTestOIDCVerifier(testSecret)
	_, err = v.Verify(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing email")
}

func TestVerify_GarbageToken(t *testing.T) {
	v := NewTestOIDCVerifier(testSecret)
	_, err := v.Verify(context.Background(), "not-a-jwt")
	require.Error(t, err)
}
