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
	encryptFn func(plaintext []byte) ([]byte, error)
	decryptFn func(ciphertext []byte) ([]byte, error)
}

func (m *mockCrypto) Encrypt(plaintext []byte) ([]byte, error) {
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
	getDefaultFn func(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error)
}

func (m *mockAdapterIdentityStore) Create(_ context.Context, _ *domain.AdapterIdentity) error { return nil }
func (m *mockAdapterIdentityStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
	return nil, nil
}
func (m *mockAdapterIdentityStore) Update(_ context.Context, _ *domain.AdapterIdentity) error {
	return nil
}
func (m *mockAdapterIdentityStore) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockAdapterIdentityStore) ListByAdapter(_ context.Context, _ uuid.UUID) ([]*domain.AdapterIdentity, error) {
	return nil, nil
}
func (m *mockAdapterIdentityStore) GetDefault(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
	if m.getDefaultFn != nil {
		return m.getDefaultFn(ctx, adapterID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockAdapterIdentityStore) SetDefault(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *mockAdapterIdentityStore) UpsertBatch(_ context.Context, _ uuid.UUID, _ []*domain.AdapterIdentity) error {
	return nil
}
func (m *mockAdapterIdentityStore) DeleteStale(_ context.Context, _ uuid.UUID, _ []string) error {
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

// --- Helpers ---

func noopSenderFactory(_ context.Context, _ *domain.Adapter, _ []byte) (port.EmailSender, error) {
	return &mockEmailSender{}, nil
}

func setupAdapterTest(as port.AdapterStore, crypto port.Crypto, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.AdapterHandler) {
	return setupAdapterTestFull(as, crypto, ts, ws, noopSenderFactory, &mockAdapterIdentityStore{})
}

func setupAdapterTestFull(
	as port.AdapterStore,
	crypto port.Crypto,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	sf port.SenderFactory,
	is port.AdapterIdentityStore,
) (*echo.Echo, *handler.AdapterHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewAdapterHandler(as, crypto, ts, ws, sf, is)

	// Workspace-scoped routes.
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters", h.Create)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id", h.Get)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id", h.Update)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id", h.SoftDelete)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/test", h.TestConnection)

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

func TestAdapterHandler_Create_InvalidType(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupAdapterTest(&mockAdapterStore{}, &mockCrypto{}, ts, wsStore)

	body := `{"name":"Bad Adapter","adapter_type":"smtp","config":{"host":"localhost"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
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

	e, _ := setupAdapterTestFull(as, &mockCrypto{}, ts, wsStore, sf, is)

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

	e, _ := setupAdapterTestFull(as, &mockCrypto{}, ts, wsStore, noopSenderFactory, is)

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
