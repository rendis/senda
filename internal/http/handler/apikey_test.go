package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

// --- Mock APIKeyStore ---

type mockAPIKeyStore struct {
	createFn          func(ctx context.Context, key *domain.APIKey) error
	getByIDFn         func(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
	getByHashFn       func(ctx context.Context, hash string) (*domain.APIKey, error)
	revokeFn          func(ctx context.Context, id uuid.UUID) error
	touchLastUsedFn   func(ctx context.Context, id uuid.UUID) error
	listByWorkspaceFn func(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.APIKey], error)
}

func (m *mockAPIKeyStore) Create(ctx context.Context, key *domain.APIKey) error {
	if m.createFn != nil {
		return m.createFn(ctx, key)
	}
	return nil
}
func (m *mockAPIKeyStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockAPIKeyStore) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	if m.getByHashFn != nil {
		return m.getByHashFn(ctx, hash)
	}
	return nil, nil
}
func (m *mockAPIKeyStore) Revoke(ctx context.Context, id uuid.UUID) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, id)
	}
	return nil
}
func (m *mockAPIKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	if m.touchLastUsedFn != nil {
		return m.touchLastUsedFn(ctx, id)
	}
	return nil
}
func (m *mockAPIKeyStore) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.APIKey], error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, opts)
	}
	return &port.PageResult[domain.APIKey]{Items: []*domain.APIKey{}}, nil
}

// --- Helpers ---

// fakeMember creates a middleware that injects a fake authenticated member into context.
func fakeMember(member *domain.Member) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("member", member)
			return next(c)
		}
	}
}

func setupAPIKeyTest(aks port.APIKeyStore, ts port.TenantStore, ws port.WorkspaceStore, member *domain.Member) (*echo.Echo, *handler.APIKeyHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())
	if member != nil {
		e.Use(fakeMember(member))
	}

	svc := service.NewAPIKeyService(aks, "test-pepper")
	h := handler.NewAPIKeyHandler(svc, ts, ws)

	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/api-keys", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/api-keys", h.List)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/api-keys/:id", h.Revoke)

	return e, h
}

// --- Tests ---

func TestAPIKeyHandler_Create_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	memberID := uuid.Must(uuid.NewV7())
	member := &domain.Member{ID: memberID, Email: "admin@acme.com"}

	var created *domain.APIKey
	aks := &mockAPIKeyStore{
		createFn: func(_ context.Context, key *domain.APIKey) error {
			created = key
			return nil
		},
	}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	body := `{"name":"Production Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected API key to be created")
	}
	if created.WorkspaceID != ws.ID {
		t.Fatalf("expected workspace ID %s, got %s", ws.ID, created.WorkspaceID)
	}
	if created.CreatedBy != memberID {
		t.Fatalf("expected created_by %s, got %s", memberID, created.CreatedBy)
	}

	// Verify response contains the full key.
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	key, ok := resp["key"].(string)
	if !ok || key == "" {
		t.Fatal("response must contain 'key' field")
	}
	if !strings.HasPrefix(key, "senda_prod_") {
		t.Fatalf("expected key prefix 'senda_prod_', got %q", key)
	}
	if resp["hint"] == nil || resp["hint"] == "" {
		t.Fatal("response must contain 'hint' field")
	}
}

func TestAPIKeyHandler_Create_EmptyName(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	memberID := uuid.Must(uuid.NewV7())
	member := &domain.Member{ID: memberID, Email: "admin@acme.com"}

	aks := &mockAPIKeyStore{}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	// Empty name should fail validation — name is required.
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Create_NameTooLong(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	aks := &mockAPIKeyStore{}
	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	longName := strings.Repeat("a", 101)
	body := `{"name":"` + longName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Create_NameWithControlChars(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	aks := &mockAPIKeyStore{}
	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	body := `{"name":"bad\u0000name"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Create_NoAuth(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	aks := &mockAPIKeyStore{}

	// No member injected.
	e, _ := setupAPIKeyTest(aks, ts, wsStore, nil)

	body := `{"name":"Test Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Create_InvalidWorkspace(t *testing.T) {
	memberID := uuid.Must(uuid.NewV7())
	member := &domain.Member{ID: memberID, Email: "admin@acme.com"}

	// TenantStore returns not found.
	ts := &mockTenantStore{
		getByCodeFn: func(_ context.Context, _ string) (*domain.Tenant, error) {
			return nil, domain.ErrNotFound
		},
	}
	wsStore := &mockWorkspaceStore{}
	aks := &mockAPIKeyStore{}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	body := `{"name":"Test Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/nonexistent/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Create_ResponseNeverExposesHash(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	aks := &mockAPIKeyStore{}
	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	body := `{"name":"Secure Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["key_hash"]; ok {
		t.Fatal("response must NOT contain 'key_hash' field")
	}
	if _, ok := raw["hash"]; ok {
		t.Fatal("response must NOT contain 'hash' field")
	}
}

func TestAPIKeyHandler_List_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	now := time.Now().UTC()
	memberID := uuid.Must(uuid.NewV7())
	aks := &mockAPIKeyStore{
		listByWorkspaceFn: func(_ context.Context, wID uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.APIKey], error) {
			if wID != ws.ID {
				t.Fatalf("expected workspace ID %s, got %s", ws.ID, wID)
			}
			return &port.PageResult[domain.APIKey]{
				Items: []*domain.APIKey{
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: ws.ID, Name: "key1", KeyHash: "secret_hash", KeyPrefix: "abcd1234", KeyHint: "xyz12345", CreatedBy: memberID, CreatedAt: now},
				},
				HasMore: false,
			}, nil
		},
	}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify no hash or key in response.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw["items"], &items); err != nil {
		t.Fatalf("decode items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if _, ok := items[0]["key"]; ok {
		t.Fatal("list response must NOT contain 'key' field")
	}
	if _, ok := items[0]["key_hash"]; ok {
		t.Fatal("list response must NOT contain 'key_hash' field")
	}
	if _, ok := items[0]["hash"]; ok {
		t.Fatal("list response must NOT contain 'hash' field")
	}
}

func TestAPIKeyHandler_List_NeverExposesHash(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	now := time.Now().UTC()
	used := now.Add(-1 * time.Hour)
	revoked := now.Add(-30 * time.Minute)
	aks := &mockAPIKeyStore{
		listByWorkspaceFn: func(_ context.Context, _ uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.APIKey], error) {
			return &port.PageResult[domain.APIKey]{
				Items: []*domain.APIKey{
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: ws.ID, Name: "active", KeyHash: "hash1", KeyPrefix: "pre1", KeyHint: "hint1234", CreatedBy: uuid.Must(uuid.NewV7()), LastUsedAt: &used, CreatedAt: now},
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: ws.ID, Name: "revoked", KeyHash: "hash2", KeyPrefix: "pre2", KeyHint: "hint5678", CreatedBy: uuid.Must(uuid.NewV7()), RevokedAt: &revoked, CreatedAt: now},
				},
				HasMore: true,
				NextCursor: "cursor123",
			}, nil
		},
	}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/api-keys", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor"`
		HasMore    bool             `json:"has_more"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
	if resp.NextCursor != "cursor123" {
		t.Fatalf("expected next_cursor 'cursor123', got %q", resp.NextCursor)
	}

	// Verify fields for active key.
	active := resp.Items[0]
	if active["name"] != "active" {
		t.Fatalf("expected name 'active', got %v", active["name"])
	}
	if active["hint"] != "hint1234" {
		t.Fatalf("expected hint 'hint1234', got %v", active["hint"])
	}
	if active["last_used_at"] == nil {
		t.Fatal("expected last_used_at to be present")
	}

	// Verify fields for revoked key.
	revokedKey := resp.Items[1]
	if revokedKey["revoked_at"] == nil {
		t.Fatal("expected revoked_at to be present")
	}
}

func TestAPIKeyHandler_Revoke_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	keyID := uuid.Must(uuid.NewV7())
	var revokedID uuid.UUID
	aks := &mockAPIKeyStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.APIKey, error) {
			if id == keyID {
				return &domain.APIKey{ID: keyID, WorkspaceID: ws.ID}, nil
			}
			return nil, domain.ErrNotFound
		},
		revokeFn: func(_ context.Context, id uuid.UUID) error {
			revokedID = id
			return nil
		},
	}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/api-keys/"+keyID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if revokedID != keyID {
		t.Fatalf("expected revoked ID %s, got %s", keyID, revokedID)
	}
}

func TestAPIKeyHandler_Revoke_CrossWorkspace(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	otherWsID := uuid.Must(uuid.NewV7())
	otherKeyID := uuid.Must(uuid.NewV7())
	aks := &mockAPIKeyStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.APIKey, error) {
			// Key exists but belongs to a different workspace.
			return &domain.APIKey{ID: id, WorkspaceID: otherWsID}, nil
		},
	}

	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/api-keys/"+otherKeyID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-workspace revoke, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Revoke_InvalidID(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	aks := &mockAPIKeyStore{}
	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/api-keys/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPIKeyHandler_Revoke_NotFound(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	member := &domain.Member{ID: uuid.Must(uuid.NewV7()), Email: "admin@acme.com"}

	keyID := uuid.Must(uuid.NewV7())
	aks := &mockAPIKeyStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.APIKey, error) {
			return &domain.APIKey{ID: id, WorkspaceID: ws.ID}, nil
		},
		revokeFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrNotFound
		},
	}
	e, _ := setupAPIKeyTest(aks, ts, wsStore, member)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/api-keys/"+keyID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
