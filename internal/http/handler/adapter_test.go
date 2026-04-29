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

// --- Mock AdapterStore ---

type mockAdapterStore struct {
	createFn          func(ctx context.Context, adapter *domain.Adapter) error
	getByIDFn         func(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
	updateFn          func(ctx context.Context, adapter *domain.Adapter) error
	softDeleteFn      func(ctx context.Context, id uuid.UUID) error
	listInChainFn     func(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Adapter, error)
	listByWorkspaceFn func(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error)
}

func (m *mockAdapterStore) Create(ctx context.Context, adapter *domain.Adapter) error {
	if m.createFn != nil {
		return m.createFn(ctx, adapter)
	}
	return nil
}
func (m *mockAdapterStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockAdapterStore) Update(ctx context.Context, adapter *domain.Adapter) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, adapter)
	}
	return nil
}
func (m *mockAdapterStore) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockAdapterStore) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Adapter, error) {
	if m.listInChainFn != nil {
		return m.listInChainFn(ctx, scopes)
	}
	return nil, nil
}
func (m *mockAdapterStore) ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Adapter], error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, opts)
	}
	return &port.PageResult[domain.Adapter]{Items: []*domain.Adapter{}}, nil
}

// --- Mock Crypto ---

type mockCrypto struct {
	encryptFn     func(plaintext []byte) ([]byte, error)
	decryptFn     func(ciphertext []byte) ([]byte, error)
	lastPlaintext []byte
}

func (m *mockCrypto) Encrypt(plaintext []byte) ([]byte, error) {
	m.lastPlaintext = append([]byte(nil), plaintext...)
	if m.encryptFn != nil {
		return m.encryptFn(plaintext)
	}
	return append([]byte("enc:"), plaintext...), nil
}
func (m *mockCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	if m.decryptFn != nil {
		return m.decryptFn(ciphertext)
	}
	return ciphertext[4:], nil
}

// --- Mock AdapterIdentityStore ---

type mockAdapterIdentityStore struct {
	createFn        func(ctx context.Context, identity *domain.AdapterIdentity) error
	getByIDFn       func(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error)
	updateFn        func(ctx context.Context, identity *domain.AdapterIdentity) error
	deleteFn        func(ctx context.Context, id uuid.UUID) error
	listByAdapterFn func(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error)
	getDefaultFn    func(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error)
	setDefaultFn    func(ctx context.Context, adapterID, identityID uuid.UUID) error
	upsertBatchFn   func(ctx context.Context, adapterID uuid.UUID, identities []*domain.AdapterIdentity) error
	deleteStaleFn   func(ctx context.Context, adapterID uuid.UUID, keepIdentities []string) error
}

func (m *mockAdapterIdentityStore) Create(ctx context.Context, identity *domain.AdapterIdentity) error {
	if m.createFn != nil {
		return m.createFn(ctx, identity)
	}
	return nil
}
func (m *mockAdapterIdentityStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockAdapterIdentityStore) Update(ctx context.Context, identity *domain.AdapterIdentity) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, identity)
	}
	return nil
}
func (m *mockAdapterIdentityStore) Delete(ctx context.Context, id uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m *mockAdapterIdentityStore) ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	if m.listByAdapterFn != nil {
		return m.listByAdapterFn(ctx, adapterID)
	}
	return nil, nil
}
func (m *mockAdapterIdentityStore) GetDefault(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
	if m.getDefaultFn != nil {
		return m.getDefaultFn(ctx, adapterID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockAdapterIdentityStore) SetDefault(ctx context.Context, adapterID uuid.UUID, identityID uuid.UUID) error {
	if m.setDefaultFn != nil {
		return m.setDefaultFn(ctx, adapterID, identityID)
	}
	return nil
}
func (m *mockAdapterIdentityStore) UpsertBatch(ctx context.Context, adapterID uuid.UUID, identities []*domain.AdapterIdentity) error {
	if m.upsertBatchFn != nil {
		return m.upsertBatchFn(ctx, adapterID, identities)
	}
	return nil
}
func (m *mockAdapterIdentityStore) DeleteStale(ctx context.Context, adapterID uuid.UUID, keepIdentities []string) error {
	if m.deleteStaleFn != nil {
		return m.deleteStaleFn(ctx, adapterID, keepIdentities)
	}
	return nil
}

// --- Mock EmailSender ---

type mockEmailSender struct {
	sendFn func(ctx context.Context, msg *port.OutgoingEmail) (string, error)
}

func (m *mockEmailSender) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return "mock-msg-id", nil
}
func (m *mockEmailSender) Name() string                        { return "mock" }
func (m *mockEmailSender) HealthCheck(_ context.Context) error { return nil }

type mockDeprovisioner struct {
	deprovisionFn func(ctx context.Context, adapterID uuid.UUID) error
}

func (m *mockDeprovisioner) Deprovision(ctx context.Context, adapterID uuid.UUID) error {
	if m.deprovisionFn != nil {
		return m.deprovisionFn(ctx, adapterID)
	}
	return nil
}

// --- Helpers ---

func noopSenderFactory(_ context.Context, _ *domain.Adapter, _ []byte) (port.EmailSender, error) {
	return &mockEmailSender{}, nil
}

func setupAdapterTest(as port.AdapterStore, crypto port.Crypto, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.AdapterHandler) {
	return setupAdapterTestFull(as, crypto, ts, ws, noopSenderFactory, &mockAdapterIdentityStore{}, nil)
}

func setupAdapterTestFull(
	as port.AdapterStore,
	crypto port.Crypto,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	sf port.SenderFactory,
	is port.AdapterIdentityStore,
	deprov port.Deprovisioner,
) (*echo.Echo, *handler.AdapterHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewAdapterHandler(as, crypto, ts, ws, sf, is, deprov, nil)

	// Workspace-scoped routes.
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id", h.Update)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id", h.SoftDelete)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/test", h.TestConnection)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/workspace-access", h.GetWorkspaceAccess)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/workspace-access", h.UpdateWorkspaceAccess)

	// Global routes.
	e.POST("/api/v1/manage/global/adapters", h.CreateGlobal)
	e.GET("/api/v1/manage/global/adapters", h.ListGlobal)
	e.GET("/api/v1/manage/global/adapters/:id", h.GetGlobal)
	e.PUT("/api/v1/manage/global/adapters/:id", h.UpdateGlobal)
	e.DELETE("/api/v1/manage/global/adapters/:id", h.SoftDeleteGlobal)
	e.POST("/api/v1/manage/global/adapters/:id/test", h.TestConnectionGlobal)

	return e, h
}

// --- Tests ---

func TestAdapterHandler_Create_EncryptsConfig(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	var created *domain.Adapter
	as := &mockAdapterStore{
		createFn: func(_ context.Context, a *domain.Adapter) error {
			created = a
			return nil
		},
	}
	crypto := &mockCrypto{}

	e, _ := setupAdapterTest(as, crypto, ts, wsStore)

	body := `{"name":"SES Production","adapter_type":"ses","config":{"region":"us-east-1"},"is_default":true,"rate_limit_per_second":50}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected adapter to be created")
	}
	if created.WorkspaceID == nil || *created.WorkspaceID != ws.ID {
		t.Fatalf("expected workspace ID %s", ws.ID)
	}
	// Verify config was encrypted (mock prefixes with "enc:")
	if len(created.ConfigEncrypted) == 0 || string(created.ConfigEncrypted[:4]) != "enc:" {
		t.Fatalf("expected encrypted config, got %q", string(created.ConfigEncrypted))
	}
}

func TestAdapterHandler_Update_SharedAdapterIsReadOnly(t *testing.T) {
	tenant, ws, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	wsStore.getByIDFn = func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
		switch id {
		case systemWSID:
			return &domain.Workspace{
				ID:       systemWSID,
				TenantID: tenant.ID,
				Code:     "_system",
				Name:     "System",
				IsSystem: true,
			}, nil
		case ws.ID:
			return ws, nil
		default:
			return nil, domain.ErrNotFound
		}
	}

	var updated bool
	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWSID,
				Name:        "Shared Gmail",
				AdapterType: domain.AdapterTypeGmail,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Adapter) error {
			updated = true
			return nil
		},
	}

	e, h := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)

	accessSvc := service.NewAdapterAccessService(
		as,
		&mockAdapterIdentityStore{},
		wsStore,
		&mockAdapterGrantStoreHandler{
			hasAdapterWorkspaceGrantFn: func(_ context.Context, id uuid.UUID, workspaceID uuid.UUID) (bool, error) {
				return id == adapterID && workspaceID == ws.ID, nil
			},
		},
		&mockIdentityGrantStoreHandler{},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	body := `{"name":"Attempted Rename"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated {
		t.Fatal("shared adapter update should not reach store.Update")
	}
}

func TestAdapterHandler_Update_SMTPKeepsPasswordWhenBlank(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:              adapterID,
				WorkspaceID:     &ws.ID,
				Name:            "SMTP Relay",
				AdapterType:     domain.AdapterTypeSMTP,
				ConfigEncrypted: []byte("encrypted"),
				CreatedAt:       now,
				UpdatedAt:       now,
			}, nil
		},
	}
	crypto := &mockCrypto{
		decryptFn: func(_ []byte) ([]byte, error) {
			return []byte(`{"host":"smtp.example.com","port":587,"tls_mode":"starttls","auth_mode":"plain","username":"apikey","password":"old-secret"}`), nil
		},
	}
	e, _ := setupAdapterTest(as, crypto, ts, wsStore)

	body := `{"config":{"host":"smtp.example.com","port":587,"tls_mode":"starttls","auth_mode":"plain","username":"apikey","password":""}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var encryptedConfig map[string]any
	if err := json.Unmarshal(crypto.lastPlaintext, &encryptedConfig); err != nil {
		t.Fatalf("updated config JSON error = %v", err)
	}
	if encryptedConfig["password"] != "old-secret" {
		t.Fatalf("password = %v, want old-secret", encryptedConfig["password"])
	}
}

func TestAdapterHandler_Update_SMTPValidatesMergedConfig(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()
	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var updated bool
	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:              adapterID,
				WorkspaceID:     &ws.ID,
				Name:            "SMTP Relay",
				AdapterType:     domain.AdapterTypeSMTP,
				ConfigEncrypted: []byte("encrypted"),
				CreatedAt:       now,
				UpdatedAt:       now,
			}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Adapter) error {
			updated = true
			return nil
		},
	}
	crypto := &mockCrypto{
		decryptFn: func(_ []byte) ([]byte, error) {
			return []byte(`{"host":"smtp.example.com","port":587,"tls_mode":"starttls","auth_mode":"plain","username":"apikey","password":"old-secret"}`), nil
		},
	}
	e, _ := setupAdapterTest(as, crypto, ts, wsStore)

	body := `{"config":{"tls_mode":"ssl-ish"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if updated {
		t.Fatal("invalid SMTP config should not reach store.Update")
	}
}

func TestAdapterHandler_GetWorkspaceAccess_SystemScope(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	workspaceAID := uuid.Must(uuid.NewV7())
	workspaceBID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID == tenant.ID && code == "_system" {
			return &domain.Workspace{
				ID:       systemWSID,
				TenantID: tenant.ID,
				Code:     "_system",
				Name:     "System",
				IsSystem: true,
			}, nil
		}
		return nil, domain.ErrNotFound
	}
	wsStore.listByTenantFn = func(_ context.Context, tenantID uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
		if tenantID != tenant.ID {
			return nil, "", domain.ErrNotFound
		}
		return []*domain.Workspace{
			{ID: systemWSID, TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true},
			{ID: workspaceAID, TenantID: tenant.ID, Code: "alpha", Name: "Alpha"},
			{ID: workspaceBID, TenantID: tenant.ID, Code: "beta", Name: "Beta"},
		}, "", nil
	}

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWSID,
				Name:        "Shared Gmail",
				AdapterType: domain.AdapterTypeGmail,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
	}

	e, h := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)
	accessSvc := service.NewAdapterAccessService(
		as,
		&mockAdapterIdentityStore{},
		wsStore,
		&mockAdapterGrantStoreHandler{
			listAdapterWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
				if id != adapterID {
					return nil, domain.ErrNotFound
				}
				return []uuid.UUID{workspaceBID}, nil
			},
		},
		&mockIdentityGrantStoreHandler{},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/_system/adapters/"+adapterID.String()+"/workspace-access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"alpha"`) || !strings.Contains(rec.Body.String(), `"code":"beta"`) {
		t.Fatalf("expected workspace access payload to include tenant workspaces, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"is_granted":true`) {
		t.Fatalf("expected one granted workspace in payload, got %s", rec.Body.String())
	}
}

func TestAdapterHandler_UpdateWorkspaceAccess_WritesAudit(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	workspaceAID := uuid.Must(uuid.NewV7())
	memberID := uuid.Must(uuid.NewV7())

	systemWS := &domain.Workspace{ID: systemWSID, TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true}
	workspaceA := &domain.Workspace{ID: workspaceAID, TenantID: tenant.ID, Code: "alpha", Name: "Alpha"}

	wsStore.getByTenantAndCodeFn = func(_ context.Context, tenantID uuid.UUID, code string, _ domain.Environment) (*domain.Workspace, error) {
		if tenantID != tenant.ID {
			return nil, domain.ErrNotFound
		}
		switch code {
		case "_system":
			return systemWS, nil
		case "alpha":
			return workspaceA, nil
		default:
			return nil, domain.ErrNotFound
		}
	}
	wsStore.listByTenantFn = func(_ context.Context, tenantID uuid.UUID, _ domain.Environment, _ port.ListOptions) ([]*domain.Workspace, string, error) {
		if tenantID != tenant.ID {
			return nil, "", domain.ErrNotFound
		}
		return []*domain.Workspace{systemWS, workspaceA}, "", nil
	}

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWSID,
				Name:        "Shared Gmail",
				AdapterType: domain.AdapterTypeGmail,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
	}

	e, h := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(middleware.ContextKeyMember, &domain.Member{ID: memberID, Email: "tenant-admin@example.com"})
			return next(c)
		}
	})

	currentGrants := []uuid.UUID{}
	accessSvc := service.NewAdapterAccessService(
		as,
		&mockAdapterIdentityStore{},
		wsStore,
		&mockAdapterGrantStoreHandler{
			listAdapterWorkspaceGrantsFn: func(_ context.Context, gotAdapterID uuid.UUID) ([]uuid.UUID, error) {
				if gotAdapterID != adapterID {
					return nil, domain.ErrNotFound
				}
				return append([]uuid.UUID(nil), currentGrants...), nil
			},
			replaceAdapterWorkspaceGrantsFn: func(_ context.Context, gotAdapterID uuid.UUID, workspaceIDs []uuid.UUID) error {
				if gotAdapterID != adapterID {
					return domain.ErrNotFound
				}
				currentGrants = append([]uuid.UUID(nil), workspaceIDs...)
				return nil
			},
		},
		&mockIdentityGrantStoreHandler{},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	auditStore := &mockAuditLogStoreHandler{}
	h.SetAuditStore(auditStore)

	body := `{"workspace_ids":["` + workspaceAID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/_system/adapters/"+adapterID.String()+"/workspace-access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(auditStore.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditStore.entries))
	}
	entry := auditStore.entries[0]
	if entry.Action != domain.AuditUpdate {
		t.Fatalf("expected audit update, got %s", entry.Action)
	}
	if entry.EntityType != "adapter" || entry.EntityID != adapterID {
		t.Fatalf("unexpected audit entity: %s %s", entry.EntityType, entry.EntityID)
	}
	if got := entry.ActorEmail; got != "tenant-admin@example.com" {
		t.Fatalf("expected actor email tenant-admin@example.com, got %s", got)
	}
	if after, ok := entry.Changes["after_workspace_ids"].([]string); !ok || len(after) != 1 || after[0] != workspaceAID.String() {
		t.Fatalf("unexpected after_workspace_ids: %#v", entry.Changes["after_workspace_ids"])
	}
	if granted, ok := entry.Changes["granted_workspace_ids"].([]string); !ok || len(granted) != 1 || granted[0] != workspaceAID.String() {
		t.Fatalf("unexpected granted_workspace_ids: %#v", entry.Changes["granted_workspace_ids"])
	}
}

func TestAdapterHandler_List_NoConfigField(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	as := &mockAdapterStore{
		listByWorkspaceFn: func(_ context.Context, _ *uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Adapter], error) {
			return &port.PageResult[domain.Adapter]{
				Items: []*domain.Adapter{
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: &ws.ID, Name: "SES", AdapterType: domain.AdapterTypeSES, ConfigEncrypted: []byte("secret"), CreatedAt: now, UpdatedAt: now},
				},
				HasMore: false,
			}, nil
		},
	}

	e, _ := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/adapters", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify no "config" or "config_encrypted" field in JSON response.
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
	if _, ok := items[0]["config"]; ok {
		t.Fatal("response should not contain 'config' field")
	}
	if _, ok := items[0]["config_encrypted"]; ok {
		t.Fatal("response should not contain 'config_encrypted' field")
	}
}

func TestAdapterHandler_Get_NoConfigField(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			return &domain.Adapter{
				ID: adapterID, WorkspaceID: &ws.ID, Name: "SES", AdapterType: domain.AdapterTypeSES,
				ConfigEncrypted: []byte("secret"), CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["config"]; ok {
		t.Fatal("response should not contain 'config' field")
	}
	if _, ok := raw["config_encrypted"]; ok {
		t.Fatal("response should not contain 'config_encrypted' field")
	}
}

func TestAdapterHandler_Update_ReencryptsConfig(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var updatedAdapter *domain.Adapter
	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return &domain.Adapter{
				ID: adapterID, WorkspaceID: &ws.ID, Name: "SES", AdapterType: domain.AdapterTypeSES,
				ConfigEncrypted: []byte(`enc:{"region":"us-east-1"}`), IsDefault: false, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		updateFn: func(_ context.Context, a *domain.Adapter) error {
			updatedAdapter = a
			return nil
		},
	}

	e, _ := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)

	body := `{"config":{"region":"eu-west-1"},"is_default":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if updatedAdapter == nil {
		t.Fatal("expected adapter to be updated")
	}
	if !updatedAdapter.IsDefault {
		t.Fatal("expected is_default=true")
	}
	// Config should be re-encrypted.
	if string(updatedAdapter.ConfigEncrypted[:4]) != "enc:" {
		t.Fatal("expected re-encrypted config")
	}
}

func TestAdapterHandler_SoftDelete_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var deletedID uuid.UUID
	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return &domain.Adapter{
				ID: adapterID, WorkspaceID: &ws.ID, Name: "SES", AdapterType: domain.AdapterTypeSES,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		softDeleteFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	e, _ := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if deletedID != adapterID {
		t.Fatalf("expected deleted ID %s, got %s", adapterID, deletedID)
	}
}

func TestAdapterHandler_SoftDelete_ContinuesAfterDeprovisionPanic(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var deletedID uuid.UUID
	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &ws.ID,
				Name:        "SES",
				AdapterType: domain.AdapterTypeSES,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
		softDeleteFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}
	deprov := &mockDeprovisioner{
		deprovisionFn: func(_ context.Context, _ uuid.UUID) error {
			panic("boom")
		},
	}

	e, _ := setupAdapterTestFull(as, &mockCrypto{}, ts, wsStore, noopSenderFactory, &mockAdapterIdentityStore{}, deprov)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if deletedID != adapterID {
		t.Fatalf("expected deleted ID %s, got %s", adapterID, deletedID)
	}
}

func TestAdapterHandler_Create_InvalidType(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupAdapterTest(&mockAdapterStore{}, &mockCrypto{}, ts, wsStore)

	body := `{"name":"Bad Adapter","adapter_type":"mailgun","config":{"host":"localhost"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdapterHandler_Create_SMTPValidatesAndStoresSafeMeta(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var created *domain.Adapter
	as := &mockAdapterStore{
		createFn: func(_ context.Context, adapter *domain.Adapter) error {
			created = adapter
			return nil
		},
	}
	e, _ := setupAdapterTest(as, &mockCrypto{}, ts, wsStore)

	body := `{
		"name":"SMTP Relay",
		"adapter_type":"smtp",
		"config":{
			"host":"smtp.example.com",
			"port":587,
			"tls_mode":"starttls",
			"auth_mode":"plain",
			"username":"apikey",
			"password":"secret"
		},
		"rate_limit_per_second":10
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected adapter to be created")
	}
	if created.AdapterType != domain.AdapterTypeSMTP {
		t.Fatalf("adapter type = %q, want smtp", created.AdapterType)
	}
	if created.ConfigMeta["host"] != "smtp.example.com" {
		t.Fatalf("host meta = %q", created.ConfigMeta["host"])
	}
	if created.ConfigMeta["port"] != "587" {
		t.Fatalf("port meta = %q", created.ConfigMeta["port"])
	}
	if created.ConfigMeta["tls_mode"] != "starttls" {
		t.Fatalf("tls_mode meta = %q", created.ConfigMeta["tls_mode"])
	}
	if created.ConfigMeta["auth_mode"] != "plain" {
		t.Fatalf("auth_mode meta = %q", created.ConfigMeta["auth_mode"])
	}
	if _, leaked := created.ConfigMeta["password"]; leaked {
		t.Fatal("password must not be stored in config_meta")
	}
	if _, leaked := created.ConfigMeta["username"]; leaked {
		t.Fatal("username must not be stored in config_meta")
	}
}

func TestAdapterHandler_GlobalCreate_Success(t *testing.T) {
	var created *domain.Adapter
	as := &mockAdapterStore{
		createFn: func(_ context.Context, a *domain.Adapter) error {
			created = a
			return nil
		},
	}

	e, _ := setupAdapterTest(as, &mockCrypto{}, &mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"name":"Global SES","adapter_type":"ses","config":{"region":"us-east-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/global/adapters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected adapter to be created")
	}
	if created.WorkspaceID != nil {
		t.Fatal("expected nil workspace ID for global adapter")
	}
}

func TestAdapterHandler_TestSend_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return &domain.Adapter{
				ID: adapterID, WorkspaceID: &ws.ID, Name: "SES", AdapterType: domain.AdapterTypeSES,
				ConfigEncrypted: []byte("enc:test"), CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	var sentMsg *port.OutgoingEmail
	sf := func(_ context.Context, _ *domain.Adapter, _ []byte) (port.EmailSender, error) {
		return &mockEmailSender{
			sendFn: func(_ context.Context, msg *port.OutgoingEmail) (string, error) {
				sentMsg = msg
				return "test-provider-id-123", nil
			},
		}, nil
	}

	displayName := "Test Sender"
	is := &mockAdapterIdentityStore{
		getDefaultFn: func(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
			return &domain.AdapterIdentity{
				ID:          uuid.Must(uuid.NewV7()),
				AdapterID:   adapterID,
				Identity:    "sender@example.com",
				DisplayName: &displayName,
				IsDefault:   true,
			}, nil
		},
	}

	e, _ := setupAdapterTestFull(as, &mockCrypto{}, ts, wsStore, sf, is, nil)

	body := `{"to":"recipient@example.com","subject":"Test Email","body":"<h1>Hello</h1>"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String()+"/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["status"] != "sent" {
		t.Fatalf("expected status=sent, got %s", resp["status"])
	}
	if resp["provider_message_id"] != "test-provider-id-123" {
		t.Fatalf("expected provider_message_id=test-provider-id-123, got %s", resp["provider_message_id"])
	}
	if resp["from"] != "sender@example.com" {
		t.Fatalf("expected from=sender@example.com, got %s", resp["from"])
	}

	if sentMsg == nil {
		t.Fatal("expected email to be sent")
	}
	if sentMsg.To.Address != "recipient@example.com" {
		t.Fatalf("expected to=recipient@example.com, got %s", sentMsg.To.Address)
	}
	if sentMsg.Subject != "Test Email" {
		t.Fatalf("expected subject=Test Email, got %s", sentMsg.Subject)
	}
	if sentMsg.From.Name != "Test Sender" {
		t.Fatalf("expected from name=Test Sender, got %s", sentMsg.From.Name)
	}
}

func TestAdapterHandler_TestSend_MissingFields(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupAdapterTest(&mockAdapterStore{}, &mockCrypto{}, ts, wsStore)

	adapterID := uuid.Must(uuid.NewV7())
	body := `{"to":"","subject":"","body":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String()+"/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdapterHandler_TestSend_NoDefaultIdentity(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	adapterID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
			return &domain.Adapter{
				ID: adapterID, WorkspaceID: &ws.ID, Name: "SES", AdapterType: domain.AdapterTypeSES,
				ConfigEncrypted: []byte("enc:test"), CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	is := &mockAdapterIdentityStore{} // GetDefault returns ErrNotFound by default

	e, _ := setupAdapterTestFull(as, &mockCrypto{}, ts, wsStore, noopSenderFactory, is, nil)

	body := `{"to":"recipient@example.com","subject":"Test","body":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String()+"/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdapterHandler_Create_MissingName(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupAdapterTest(&mockAdapterStore{}, &mockCrypto{}, ts, wsStore)

	body := `{"adapter_type":"ses","config":{"region":"us-east-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}
