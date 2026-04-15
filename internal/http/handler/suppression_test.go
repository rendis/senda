package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// --- Mock SuppressionStore ---

type mockSuppressionStore struct {
	addGlobalFn             func(ctx context.Context, entry *domain.SuppressionGlobal) error
	isGloballySuppressedFn  func(ctx context.Context, email string) (bool, error)
	removeGlobalFn          func(ctx context.Context, email string, removedBy uuid.UUID, reason string) error
	addWorkspaceFn          func(ctx context.Context, entry *domain.SuppressionWorkspace) error
	isWorkspaceSuppressedFn func(ctx context.Context, wsID uuid.UUID, email string) (bool, error)
	isSuppressedFn          func(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error)
}

func (m *mockSuppressionStore) AddGlobal(ctx context.Context, entry *domain.SuppressionGlobal) error {
	if m.addGlobalFn != nil {
		return m.addGlobalFn(ctx, entry)
	}
	return nil
}
func (m *mockSuppressionStore) IsGloballySuppressed(ctx context.Context, email string) (bool, error) {
	if m.isGloballySuppressedFn != nil {
		return m.isGloballySuppressedFn(ctx, email)
	}
	return false, nil
}
func (m *mockSuppressionStore) RemoveGlobal(ctx context.Context, email string, removedBy uuid.UUID, reason string) error {
	if m.removeGlobalFn != nil {
		return m.removeGlobalFn(ctx, email, removedBy, reason)
	}
	return nil
}
func (m *mockSuppressionStore) AddWorkspace(ctx context.Context, entry *domain.SuppressionWorkspace) error {
	if m.addWorkspaceFn != nil {
		return m.addWorkspaceFn(ctx, entry)
	}
	return nil
}
func (m *mockSuppressionStore) IsWorkspaceSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, error) {
	if m.isWorkspaceSuppressedFn != nil {
		return m.isWorkspaceSuppressedFn(ctx, wsID, email)
	}
	return false, nil
}
func (m *mockSuppressionStore) IsSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error) {
	if m.isSuppressedFn != nil {
		return m.isSuppressedFn(ctx, wsID, email)
	}
	return false, "", nil
}
func (m *mockSuppressionStore) CheckBatch(_ context.Context, _ uuid.UUID, _ []string) (map[string]string, error) {
	return map[string]string{}, nil
}

// --- Helpers ---

func setupSuppressionTest(ss port.SuppressionStore, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.SuppressionHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewSuppressionHandler(ss, ts, ws)

	base := "/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code"
	e.POST(base+"/suppression", h.Add)
	e.GET(base+"/suppression/:email", h.Check)
	e.DELETE(base+"/suppression/:email", h.Remove)

	return e, h
}

// --- Tests ---

func TestSuppressionHandler_Add_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var created *domain.SuppressionWorkspace
	ss := &mockSuppressionStore{
		addWorkspaceFn: func(_ context.Context, entry *domain.SuppressionWorkspace) error {
			created = entry
			return nil
		},
	}

	e, _ := setupSuppressionTest(ss, ts, wsStore)

	body := `{"email":"bounced@example.com","notes":"Manual suppression"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/suppression", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected suppression entry to be created")
	}
	if created.WorkspaceID != ws.ID {
		t.Fatalf("expected workspace ID %s, got %s", ws.ID, created.WorkspaceID)
	}
	if created.Email != "bounced@example.com" {
		t.Fatalf("expected email bounced@example.com, got %s", created.Email)
	}
	if created.Reason != domain.SuppressionManual {
		t.Fatalf("expected reason manual, got %s", created.Reason)
	}
}

func TestSuppressionHandler_Add_InvalidEmail(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupSuppressionTest(&mockSuppressionStore{}, ts, wsStore)

	body := `{"email":"not-an-email"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/suppression", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionHandler_Add_MissingEmail(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupSuppressionTest(&mockSuppressionStore{}, ts, wsStore)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/suppression", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionHandler_Check_Suppressed(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	ss := &mockSuppressionStore{
		isSuppressedFn: func(_ context.Context, _ uuid.UUID, email string) (bool, string, error) {
			if email == "bounced@example.com" {
				return true, "hard_bounce", nil
			}
			return false, "", nil
		},
	}

	e, _ := setupSuppressionTest(ss, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/suppression/bounced@example.com", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["suppressed"] != true {
		t.Fatal("expected suppressed=true")
	}
	if resp["reason"] != "hard_bounce" {
		t.Fatalf("expected reason hard_bounce, got %v", resp["reason"])
	}
}

func TestSuppressionHandler_Check_NotSuppressed(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	ss := &mockSuppressionStore{
		isSuppressedFn: func(_ context.Context, _ uuid.UUID, _ string) (bool, string, error) {
			return false, "", nil
		},
	}

	e, _ := setupSuppressionTest(ss, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/suppression/good@example.com", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["suppressed"] != false {
		t.Fatal("expected suppressed=false")
	}
}

func TestSuppressionHandler_Remove_Success(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	memberID := uuid.Must(uuid.NewV7())
	var removedEmail string
	ss := &mockSuppressionStore{
		removeGlobalFn: func(_ context.Context, email string, _ uuid.UUID, _ string) error {
			removedEmail = email
			return nil
		},
	}

	// Re-register routes with member + superadmin roles middleware.
	h := handler.NewSuppressionHandler(ss, ts, wsStore)
	e2 := echo.New()
	e2.HTTPErrorHandler = response.HTTPErrorHandler
	e2.Use(middleware.RequestID())
	e2.Use(middleware.Scope())
	e2.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("member", &domain.Member{ID: memberID, Email: "admin@acme.com"})
			c.Set("roles", []*domain.MemberRole{
				{Role: domain.RoleSuperadmin, ScopeType: domain.ScopeGlobal},
			})
			return next(c)
		}
	})
	e2.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/suppression/:email", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/suppression/bounced@example.com", nil)
	rec := httptest.NewRecorder()
	e2.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if removedEmail != "bounced@example.com" {
		t.Fatalf("expected removed email bounced@example.com, got %s", removedEmail)
	}
}

func TestSuppressionHandler_Remove_NonSuperadminForbidden(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	memberID := uuid.Must(uuid.NewV7())
	ss := &mockSuppressionStore{}

	h := handler.NewSuppressionHandler(ss, ts, wsStore)
	e2 := echo.New()
	e2.HTTPErrorHandler = response.HTTPErrorHandler
	e2.Use(middleware.RequestID())
	e2.Use(middleware.Scope())
	e2.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("member", &domain.Member{ID: memberID, Email: "user@acme.com"})
			c.Set("roles", []*domain.MemberRole{
				{Role: domain.RoleTenantAdmin, ScopeType: domain.ScopeTenant},
			})
			return next(c)
		}
	})
	e2.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/suppression/:email", h.Remove)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/suppression/bounced@example.com", nil)
	rec := httptest.NewRecorder()
	e2.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionHandler_Remove_NoAuth(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupSuppressionTest(&mockSuppressionStore{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/suppression/bounced@example.com", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionHandler_Add_InvalidReason(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupSuppressionTest(&mockSuppressionStore{}, ts, wsStore)

	body := `{"email":"user@example.com","reason":"unknown_reason"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/suppression", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSuppressionHandler_Add_WithCustomReason(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var created *domain.SuppressionWorkspace
	ss := &mockSuppressionStore{
		addWorkspaceFn: func(_ context.Context, entry *domain.SuppressionWorkspace) error {
			created = entry
			return nil
		},
	}

	e, _ := setupSuppressionTest(ss, ts, wsStore)

	body := `{"email":"user@example.com","reason":"hard_bounce"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/suppression", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created.Reason != domain.SuppressionHardBounce {
		t.Fatalf("expected reason hard_bounce, got %s", created.Reason)
	}
}
