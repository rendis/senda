package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/port"
)

// --- Manual Mocks ---

type mockAPIKeyStore struct {
	getByHashFn     func(ctx context.Context, hash string) (*domain.APIKey, error)
	touchLastUsedFn func(ctx context.Context, id uuid.UUID) error
}

func (m *mockAPIKeyStore) Create(_ context.Context, _ *domain.APIKey) error { return nil }
func (m *mockAPIKeyStore) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	return m.getByHashFn(ctx, hash)
}
func (m *mockAPIKeyStore) Revoke(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockAPIKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	if m.touchLastUsedFn != nil {
		return m.touchLastUsedFn(ctx, id)
	}
	return nil
}
func (m *mockAPIKeyStore) ListByWorkspace(_ context.Context, _ uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.APIKey], error) {
	return nil, nil
}

type mockMemberStore struct {
	getByEmailFn func(ctx context.Context, email string) (*domain.Member, error)
	getRolesFn   func(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error)
}

func (m *mockMemberStore) Create(_ context.Context, _ *domain.Member) error        { return nil }
func (m *mockMemberStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Member, error) {
	return nil, nil
}
func (m *mockMemberStore) GetByEmail(ctx context.Context, email string) (*domain.Member, error) {
	return m.getByEmailFn(ctx, email)
}
func (m *mockMemberStore) CountAll(_ context.Context) (int64, error) { return 0, nil }
func (m *mockMemberStore) AddRole(_ context.Context, _ *domain.MemberRole) error   { return nil }
func (m *mockMemberStore) RemoveRole(_ context.Context, _ uuid.UUID) error          { return nil }
func (m *mockMemberStore) GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error) {
	return m.getRolesFn(ctx, memberID)
}
func (m *mockMemberStore) GetRolesInScope(_ context.Context, _ uuid.UUID, _ domain.ScopeType, _ *uuid.UUID) ([]*domain.MemberRole, error) {
	return nil, nil
}
func (m *mockMemberStore) ListAll(_ context.Context, _ port.ListOptions) ([]*domain.Member, string, error) {
	return nil, "", nil
}

type mockOIDCVerifier struct {
	verifyFn func(ctx context.Context, rawToken string) (*port.OIDCClaims, error)
}

func (m *mockOIDCVerifier) Verify(ctx context.Context, rawToken string) (*port.OIDCClaims, error) {
	return m.verifyFn(ctx, rawToken)
}

// --- Tests ---

func TestAuth_MissingAuthorizationHeader(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Auth(&mockAPIKeyStore{}, &mockMemberStore{}, &mockOIDCVerifier{}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_EmptyBearerToken(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Auth(&mockAPIKeyStore{}, &mockMemberStore{}, &mockOIDCVerifier{}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_InvalidFormat(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Auth(&mockAPIKeyStore{}, &mockMemberStore{}, &mockOIDCVerifier{}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_ValidAPIKey(t *testing.T) {
	keyID := uuid.New()
	wsID := uuid.New()

	store := &mockAPIKeyStore{
		getByHashFn: func(_ context.Context, _ string) (*domain.APIKey, error) {
			return &domain.APIKey{
				ID:          keyID,
				WorkspaceID: wsID,
				Name:        "test-key",
			}, nil
		},
	}

	var gotAuthType, gotWSID any

	e := echo.New()
	e.Use(middleware.Auth(store, &mockMemberStore{}, &mockOIDCVerifier{}))
	e.GET("/test", func(c *echo.Context) error {
		gotAuthType = c.Get(middleware.ContextKeyAuthType)
		gotWSID = c.Get(middleware.ContextKeyWorkspaceID)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer senda_live_abc123xyz")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotAuthType != "apikey" {
		t.Fatalf("expected auth_type=apikey, got %v", gotAuthType)
	}
	if gotWSID != wsID {
		t.Fatalf("expected workspace_id=%s, got %v", wsID, gotWSID)
	}
}

func TestAuth_RevokedAPIKey(t *testing.T) {
	now := time.Now()
	store := &mockAPIKeyStore{
		getByHashFn: func(_ context.Context, _ string) (*domain.APIKey, error) {
			return &domain.APIKey{
				ID:        uuid.New(),
				RevokedAt: &now,
			}, nil
		},
	}

	e := echo.New()
	e.Use(middleware.Auth(store, &mockMemberStore{}, &mockOIDCVerifier{}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer senda_live_abc123xyz")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_UnknownAPIKey(t *testing.T) {
	store := &mockAPIKeyStore{
		getByHashFn: func(_ context.Context, _ string) (*domain.APIKey, error) {
			return nil, domain.ErrNotFound
		},
	}

	e := echo.New()
	e.Use(middleware.Auth(store, &mockMemberStore{}, &mockOIDCVerifier{}))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer senda_live_unknownkey")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_ValidOIDCToken(t *testing.T) {
	memberID := uuid.New()
	member := &domain.Member{
		ID:    memberID,
		Email: "alice@example.com",
	}
	roles := []*domain.MemberRole{
		{
			ID:       uuid.New(),
			MemberID: memberID,
			Role:     domain.RoleSuperadmin,
		},
	}

	verifier := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{
				Subject: "sub-123",
				Email:   "alice@example.com",
				Issuer:  "https://auth.example.com",
			}, nil
		},
	}
	memberStore := &mockMemberStore{
		getByEmailFn: func(_ context.Context, _ string) (*domain.Member, error) {
			return member, nil
		},
		getRolesFn: func(_ context.Context, _ uuid.UUID) ([]*domain.MemberRole, error) {
			return roles, nil
		},
	}

	var gotAuthType any
	var gotMember any
	var gotRoles any

	e := echo.New()
	e.Use(middleware.Auth(&mockAPIKeyStore{}, memberStore, verifier))
	e.GET("/test", func(c *echo.Context) error {
		gotAuthType = c.Get(middleware.ContextKeyAuthType)
		gotMember = c.Get(middleware.ContextKeyMember)
		gotRoles = c.Get(middleware.ContextKeyRoles)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiJ9.valid-jwt")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotAuthType != "oidc" {
		t.Fatalf("expected auth_type=oidc, got %v", gotAuthType)
	}
	m, ok := gotMember.(*domain.Member)
	if !ok || m.ID != memberID {
		t.Fatalf("expected member with ID %s, got %v", memberID, gotMember)
	}
	r, ok := gotRoles.([]*domain.MemberRole)
	if !ok || len(r) != 1 {
		t.Fatalf("expected 1 role, got %v", gotRoles)
	}
}

func TestAuth_InvalidOIDCToken(t *testing.T) {
	verifier := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return nil, errors.New("token expired")
		},
	}

	e := echo.New()
	e.Use(middleware.Auth(&mockAPIKeyStore{}, &mockMemberStore{}, verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer expired-jwt-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_OIDCEmailNotRegistered(t *testing.T) {
	verifier := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{
				Subject: "sub-999",
				Email:   "unknown@example.com",
				Issuer:  "https://auth.example.com",
			}, nil
		},
	}
	memberStore := &mockMemberStore{
		getByEmailFn: func(_ context.Context, _ string) (*domain.Member, error) {
			return nil, domain.ErrNotFound
		},
	}

	e := echo.New()
	e.Use(middleware.Auth(&mockAPIKeyStore{}, memberStore, verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-oidc-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAuth_APIKeyPrefixDetection(t *testing.T) {
	// Token without "senda_live_" prefix should go through OIDC path, not API key path.
	verifier := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return nil, errors.New("invalid token")
		},
	}
	apiKeyStore := &mockAPIKeyStore{
		getByHashFn: func(_ context.Context, _ string) (*domain.APIKey, error) {
			t.Fatal("API key store should not be called for non-prefixed token")
			return nil, nil
		},
	}

	e := echo.New()
	e.Use(middleware.Auth(apiKeyStore, &mockMemberStore{}, verifier))
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some-jwt-looking-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Should get 401 from OIDC path, not from API key path
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
