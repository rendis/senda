package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/pkg/apperr"
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
	// ContextKeyEnvironment holds the normalized request environment when present.
	ContextKeyEnvironment = "environment"
)

// Auth returns middleware that authenticates requests via API key or OIDC bearer token.
// The pepper parameter is the HMAC pepper derived from the master key, used for API key hashing.
func Auth(apiKeyStore port.APIKeyStore, memberStore port.MemberStore, oidcVerifier port.OIDCVerifier, pepper string) echo.MiddlewareFunc {
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

			if environment, ok := apiKeyEnvironment(token); ok {
				return authenticateAPIKey(c, ctx, token, environment, apiKeyStore, pepper, next)
			}

			return authenticateOIDC(c, ctx, token, oidcVerifier, memberStore, next)
		}
	}
}

func authenticateAPIKey(c *echo.Context, ctx context.Context, token string, environment domain.Environment, store port.APIKeyStore, pepper string, next echo.HandlerFunc) error {
	// Try HMAC-SHA256 hash first (new keys).
	hash := service.HashKeyHMAC(token, pepper)
	key, err := store.GetByHash(ctx, hash)
	if err != nil {
		// Fallback to plain SHA-256 for keys created before the HMAC migration.
		plainHash := service.HashKeyPlain(token)
		key, err = store.GetByHash(ctx, plainHash)
		if err != nil {
			return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid API key")
		}
		slog.Warn("API key authenticated via legacy SHA-256 hash; re-generate key to use HMAC",
			slog.String("key_prefix", key.KeyPrefix),
			slog.String("key_hint", key.KeyHint),
		)
	}

	if key.RevokedAt != nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "API key revoked")
	}

	// Fire-and-forget: update last used timestamp.
	go func() {
		_ = store.TouchLastUsed(context.Background(), key.ID)
	}()

	c.Set(ContextKeyAuthType, "apikey")
	c.Set(ContextKeyWorkspaceID, key.WorkspaceID)
	c.Set(ContextKeyEnvironment, environment)

	return next(c)
}

func apiKeyEnvironment(token string) (domain.Environment, bool) {
	for _, environment := range domain.Environments() {
		if strings.HasPrefix(token, environment.APIKeyTokenPrefix()) {
			return environment, true
		}
	}
	return "", false
}

func authenticateOIDC(c *echo.Context, ctx context.Context, token string, verifier port.OIDCVerifier, memberStore port.MemberStore, next echo.HandlerFunc) error {
	claims, err := verifier.Verify(ctx, token)
	if err != nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}
	if claims.Issuer == "" || claims.Subject == "" {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}

	member, err := memberStore.GetByOIDCIdentity(ctx, claims.Issuer, claims.Subject)
	if err != nil {
		if !apperr.IsNotFound(err) && !errors.Is(err, domain.ErrNotFound) {
			return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load member")
		}

		member, err = findUnboundMemberByEmail(ctx, memberStore, claims.Email)
		if err != nil {
			return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "identity not registered")
		}
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

func findUnboundMemberByEmail(ctx context.Context, memberStore port.MemberStore, email string) (*domain.Member, error) {
	if strings.TrimSpace(email) == "" {
		return nil, domain.ErrNotFound
	}

	member, err := memberStore.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if member.OIDCIssuer != nil || member.OIDCSubject != nil {
		return nil, domain.ErrNotFound
	}

	return member, nil
}
