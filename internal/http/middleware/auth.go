package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

const (
	// ContextKeyAuthType holds the authentication method used ("apikey" or "oidc").
	ContextKeyAuthType = "auth_type"

	// ContextKeyMember holds the authenticated *domain.Member (OIDC path only).
	ContextKeyMember = "member"

	// ContextKeyRoles holds []*domain.MemberRole for the authenticated member (OIDC path only).
	ContextKeyRoles = "roles"

	// ContextKeyWorkspaceID holds the uuid.UUID of the workspace (API key path only).
	ContextKeyWorkspaceID = "workspace_id"

	apiKeyPrefix = "senda_live_"
)

// Auth returns middleware that authenticates requests via API key or OIDC bearer token.
func Auth(apiKeyStore port.APIKeyStore, memberStore port.MemberStore, oidcVerifier port.OIDCVerifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization")
			}

			if !strings.HasPrefix(header, "Bearer ") {
				return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid authorization format")
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == "" {
				return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization")
			}

			ctx := c.Request().Context()

			if strings.HasPrefix(token, apiKeyPrefix) {
				return authenticateAPIKey(c, ctx, token, apiKeyStore, next)
			}

			return authenticateOIDC(c, ctx, token, oidcVerifier, memberStore, next)
		}
	}
}

func authenticateAPIKey(c *echo.Context, ctx context.Context, token string, store port.APIKeyStore, next echo.HandlerFunc) error {
	hash := sha256Hex(token)

	key, err := store.GetByHash(ctx, hash)
	if err != nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid API key")
	}

	if key.RevokedAt != nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "API key revoked")
	}

	// Fire-and-forget: update last used timestamp.
	go store.TouchLastUsed(context.Background(), key.ID)

	c.Set(ContextKeyAuthType, "apikey")
	c.Set(ContextKeyWorkspaceID, key.WorkspaceID)

	return next(c)
}

func authenticateOIDC(c *echo.Context, ctx context.Context, token string, verifier port.OIDCVerifier, memberStore port.MemberStore, next echo.HandlerFunc) error {
	claims, err := verifier.Verify(ctx, token)
	if err != nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}

	member, err := memberStore.GetByEmail(ctx, claims.Email)
	if err != nil {
		return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "email not registered")
	}

	roles, err := memberStore.GetRoles(ctx, member.ID)
	if err != nil {
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load roles")
	}

	c.Set(ContextKeyAuthType, "oidc")
	c.Set(ContextKeyMember, member)
	c.Set(ContextKeyRoles, roles)

	return next(c)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
