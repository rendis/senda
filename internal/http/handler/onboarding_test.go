package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// --- Mocks for onboarding ---

type mockMemberStoreOnb struct {
	countAllFn func(ctx context.Context) (int64, error)
	createFn   func(ctx context.Context, member *domain.Member) error
	addRoleFn  func(ctx context.Context, role *domain.MemberRole) error
}

func (m *mockMemberStoreOnb) Create(ctx context.Context, member *domain.Member) error {
	if m.createFn != nil {
		return m.createFn(ctx, member)
	}
	return nil
}
func (m *mockMemberStoreOnb) GetByEmail(_ context.Context, _ string) (*domain.Member, error) {
	return nil, nil
}
func (m *mockMemberStoreOnb) GetByID(_ context.Context, _ uuid.UUID) (*domain.Member, error) {
	return nil, nil
}
func (m *mockMemberStoreOnb) CountAll(ctx context.Context) (int64, error) {
	if m.countAllFn != nil {
		return m.countAllFn(ctx)
	}
	return 0, nil
}
func (m *mockMemberStoreOnb) AddRole(ctx context.Context, role *domain.MemberRole) error {
	if m.addRoleFn != nil {
		return m.addRoleFn(ctx, role)
	}
	return nil
}
func (m *mockMemberStoreOnb) RemoveRole(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockMemberStoreOnb) GetRoles(_ context.Context, _ uuid.UUID) ([]*domain.MemberRole, error) {
	return nil, nil
}
func (m *mockMemberStoreOnb) GetRolesInScope(_ context.Context, _ uuid.UUID, _ domain.ScopeType, _ *uuid.UUID) ([]*domain.MemberRole, error) {
	return nil, nil
}

type mockTenantStoreOnb struct {
	createFn func(ctx context.Context, t *domain.Tenant) error
}

func (m *mockTenantStoreOnb) Create(ctx context.Context, t *domain.Tenant) error {
	if m.createFn != nil {
		return m.createFn(ctx, t)
	}
	return nil
}
func (m *mockTenantStoreOnb) GetByID(_ context.Context, _ uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockTenantStoreOnb) GetByCode(_ context.Context, _ string) (*domain.Tenant, error) {
	return nil, nil
}
func (m *mockTenantStoreOnb) List(_ context.Context, _ port.ListOptions) ([]*domain.Tenant, string, error) {
	return nil, "", nil
}
func (m *mockTenantStoreOnb) Update(_ context.Context, _ *domain.Tenant) error { return nil }
func (m *mockTenantStoreOnb) SoftDelete(_ context.Context, _ uuid.UUID) error  { return nil }
func (m *mockTenantStoreOnb) Purge(_ context.Context, _ uuid.UUID) error       { return nil }

type mockWorkspaceStoreOnb struct {
	createFn func(ctx context.Context, ws *domain.Workspace) error
}

func (m *mockWorkspaceStoreOnb) Create(ctx context.Context, ws *domain.Workspace) error {
	if m.createFn != nil {
		return m.createFn(ctx, ws)
	}
	return nil
}
func (m *mockWorkspaceStoreOnb) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreOnb) GetByTenantAndCode(_ context.Context, _ uuid.UUID, _ string) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreOnb) GetSystemWorkspace(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceStoreOnb) ListByTenant(_ context.Context, _ uuid.UUID, _ port.ListOptions) ([]*domain.Workspace, string, error) {
	return nil, "", nil
}
func (m *mockWorkspaceStoreOnb) Update(_ context.Context, _ *domain.Workspace) error { return nil }
func (m *mockWorkspaceStoreOnb) SoftDelete(_ context.Context, _ uuid.UUID) error     { return nil }

type mockAuditStoreOnb struct{}

func (m *mockAuditStoreOnb) Append(_ context.Context, _ *domain.AuditLog) error { return nil }
func (m *mockAuditStoreOnb) Query(_ context.Context, _ port.AuditFilter, _ port.ListOptions) (*port.PageResult[domain.AuditLog], error) {
	return nil, nil
}

type mockOIDCVerifier struct {
	verifyFn func(ctx context.Context, rawToken string) (*port.OIDCClaims, error)
}

func (m *mockOIDCVerifier) Verify(ctx context.Context, rawToken string) (*port.OIDCClaims, error) {
	if m.verifyFn != nil {
		return m.verifyFn(ctx, rawToken)
	}
	return nil, nil
}

// --- Helpers ---

func setupOnboardingTest(ms port.MemberStore, ts port.TenantStore, ws port.WorkspaceStore, as port.AuditLogStore, v port.OIDCVerifier) (*echo.Echo, *handler.OnboardingHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler

	svc := service.NewOnboardingService(ms, ts, ws, as)
	h := handler.NewOnboardingHandler(svc, v)

	e.GET("/api/v1/onboarding/status", h.Status)
	e.POST("/api/v1/onboarding/setup", h.Setup)
	return e, h
}

// --- Status Tests ---

func TestOnboardingHandler_Status_NeedsOnboarding(t *testing.T) {
	ms := &mockMemberStoreOnb{
		countAllFn: func(_ context.Context) (int64, error) { return 0, nil },
	}
	e, _ := setupOnboardingTest(ms, &mockTenantStoreOnb{}, &mockWorkspaceStoreOnb{}, &mockAuditStoreOnb{}, &mockOIDCVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.OnboardingStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.NeedsOnboarding {
		t.Fatal("expected needs_onboarding=true")
	}
}

func TestOnboardingHandler_Status_AlreadyOnboarded(t *testing.T) {
	ms := &mockMemberStoreOnb{
		countAllFn: func(_ context.Context) (int64, error) { return 1, nil },
	}
	e, _ := setupOnboardingTest(ms, &mockTenantStoreOnb{}, &mockWorkspaceStoreOnb{}, &mockAuditStoreOnb{}, &mockOIDCVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.OnboardingStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.NeedsOnboarding {
		t.Fatal("expected needs_onboarding=false")
	}
}

func TestOnboardingHandler_Status_StoreError(t *testing.T) {
	ms := &mockMemberStoreOnb{
		countAllFn: func(_ context.Context) (int64, error) { return 0, errors.New("db error") },
	}
	e, _ := setupOnboardingTest(ms, &mockTenantStoreOnb{}, &mockWorkspaceStoreOnb{}, &mockAuditStoreOnb{}, &mockOIDCVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Setup Tests ---

func TestOnboardingHandler_Setup_Success(t *testing.T) {
	countCalls := 0
	ms := &mockMemberStoreOnb{
		countAllFn: func(_ context.Context) (int64, error) {
			countCalls++
			if countCalls == 1 {
				return 0, nil // Initial check
			}
			return 1, nil // Post-create check: only our member
		},
	}
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, rawToken string) (*port.OIDCClaims, error) {
			if rawToken != "valid-token" {
				return nil, errors.New("invalid token")
			}
			return &port.OIDCClaims{
				Subject: "oidc-sub-123",
				Email:   "admin@example.com",
				Issuer:  "https://auth.example.com",
			}, nil
		},
	}

	e, _ := setupOnboardingTest(ms, &mockTenantStoreOnb{}, &mockWorkspaceStoreOnb{}, &mockAuditStoreOnb{}, v)

	body := `{"tenant_code":"acme","tenant_name":"Acme Corp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.OnboardingSetupResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Member.Email != "admin@example.com" {
		t.Fatalf("expected member email 'admin@example.com', got %q", resp.Member.Email)
	}
	if resp.Tenant.Code != "acme" {
		t.Fatalf("expected tenant code 'acme', got %q", resp.Tenant.Code)
	}
	if resp.Tenant.Name != "Acme Corp" {
		t.Fatalf("expected tenant name 'Acme Corp', got %q", resp.Tenant.Name)
	}
	if resp.Workspace.Code != "_system" {
		t.Fatalf("expected workspace code '_system', got %q", resp.Workspace.Code)
	}
}

func TestOnboardingHandler_Setup_MissingAuthHeader(t *testing.T) {
	e, _ := setupOnboardingTest(
		&mockMemberStoreOnb{},
		&mockTenantStoreOnb{},
		&mockWorkspaceStoreOnb{},
		&mockAuditStoreOnb{},
		&mockOIDCVerifier{},
	)

	body := `{"tenant_code":"acme","tenant_name":"Acme Corp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardingHandler_Setup_InvalidToken(t *testing.T) {
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return nil, errors.New("invalid token")
		},
	}

	e, _ := setupOnboardingTest(
		&mockMemberStoreOnb{},
		&mockTenantStoreOnb{},
		&mockWorkspaceStoreOnb{},
		&mockAuditStoreOnb{},
		v,
	)

	body := `{"tenant_code":"acme","tenant_name":"Acme Corp"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardingHandler_Setup_InvalidBody(t *testing.T) {
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{Subject: "s", Email: "a@b.com", Issuer: "i"}, nil
		},
	}

	e, _ := setupOnboardingTest(
		&mockMemberStoreOnb{},
		&mockTenantStoreOnb{},
		&mockWorkspaceStoreOnb{},
		&mockAuditStoreOnb{},
		v,
	)

	body := `not-json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardingHandler_Setup_ValidationError_MissingTenantCode(t *testing.T) {
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{Subject: "s", Email: "a@b.com", Issuer: "i"}, nil
		},
	}

	e, _ := setupOnboardingTest(
		&mockMemberStoreOnb{},
		&mockTenantStoreOnb{},
		&mockWorkspaceStoreOnb{},
		&mockAuditStoreOnb{},
		v,
	)

	body := `{"tenant_code":"","tenant_name":"Acme"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardingHandler_Setup_ValidationError_MissingTenantName(t *testing.T) {
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{Subject: "s", Email: "a@b.com", Issuer: "i"}, nil
		},
	}

	e, _ := setupOnboardingTest(
		&mockMemberStoreOnb{},
		&mockTenantStoreOnb{},
		&mockWorkspaceStoreOnb{},
		&mockAuditStoreOnb{},
		v,
	)

	body := `{"tenant_code":"acme","tenant_name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardingHandler_Setup_Conflict(t *testing.T) {
	ms := &mockMemberStoreOnb{
		countAllFn: func(_ context.Context) (int64, error) { return 1, nil },
	}
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{Subject: "s", Email: "a@b.com", Issuer: "i"}, nil
		},
	}

	e, _ := setupOnboardingTest(ms, &mockTenantStoreOnb{}, &mockWorkspaceStoreOnb{}, &mockAuditStoreOnb{}, v)

	body := `{"tenant_code":"acme","tenant_name":"Acme"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOnboardingHandler_Setup_InvalidSlug(t *testing.T) {
	v := &mockOIDCVerifier{
		verifyFn: func(_ context.Context, _ string) (*port.OIDCClaims, error) {
			return &port.OIDCClaims{Subject: "s", Email: "a@b.com", Issuer: "i"}, nil
		},
	}

	e, _ := setupOnboardingTest(
		&mockMemberStoreOnb{},
		&mockTenantStoreOnb{},
		&mockWorkspaceStoreOnb{},
		&mockAuditStoreOnb{},
		v,
	)

	body := `{"tenant_code":"AB","tenant_name":"Acme"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}
