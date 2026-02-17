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
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// --- Mock DomainStore (handler test) ---

type mockDomainStoreH struct {
	createFn          func(ctx context.Context, d *domain.Domain) error
	getByIDFn         func(ctx context.Context, id uuid.UUID) (*domain.Domain, error)
	updateFn          func(ctx context.Context, d *domain.Domain) error
	softDeleteFn      func(ctx context.Context, id uuid.UUID) error
	listInChainFn     func(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error)
	listByWorkspaceFn func(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Domain], error)
	getPendingFn      func(ctx context.Context, limit int) ([]*domain.Domain, error)
}

func (m *mockDomainStoreH) Create(ctx context.Context, d *domain.Domain) error {
	if m.createFn != nil {
		return m.createFn(ctx, d)
	}
	return nil
}
func (m *mockDomainStoreH) GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockDomainStoreH) Update(ctx context.Context, d *domain.Domain) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, d)
	}
	return nil
}
func (m *mockDomainStoreH) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockDomainStoreH) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error) {
	if m.listInChainFn != nil {
		return m.listInChainFn(ctx, scopes)
	}
	return nil, nil
}
func (m *mockDomainStoreH) ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Domain], error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, opts)
	}
	return &port.PageResult[domain.Domain]{Items: []*domain.Domain{}}, nil
}
func (m *mockDomainStoreH) GetPendingVerifications(ctx context.Context, limit int) ([]*domain.Domain, error) {
	if m.getPendingFn != nil {
		return m.getPendingFn(ctx, limit)
	}
	return nil, nil
}

// --- Mock JobQueue (handler test) ---

type mockJobQueueH struct {
	enqueueSendFn        func(ctx context.Context, job *port.SendJob) error
	enqueueDomainCheckFn func(ctx context.Context, domainID uuid.UUID) error
	enqueueWebhookFn     func(ctx context.Context, job *port.WebhookJob) error
}

func (m *mockJobQueueH) EnqueueSend(ctx context.Context, job *port.SendJob) error {
	if m.enqueueSendFn != nil {
		return m.enqueueSendFn(ctx, job)
	}
	return nil
}
func (m *mockJobQueueH) EnqueueDomainCheck(ctx context.Context, domainID uuid.UUID) error {
	if m.enqueueDomainCheckFn != nil {
		return m.enqueueDomainCheckFn(ctx, domainID)
	}
	return nil
}
func (m *mockJobQueueH) EnqueueWebhook(ctx context.Context, job *port.WebhookJob) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, job)
	}
	return nil
}

// --- Helpers ---

func setupDomainTest(ds port.DomainStore, crypto port.Crypto, jq port.JobQueue, ts port.TenantStore, ws port.WorkspaceStore) (*echo.Echo, *handler.DomainHTTPHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	svc := service.NewDomainService(ds, crypto, jq)
	h := handler.NewDomainHTTPHandler(svc, ds, ts, ws)

	// Workspace-scoped routes.
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/domains", h.Register)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/domains", h.List)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/domains/:id", h.Get)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/domains/:id/verify", h.VerifyNow)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/domains/:id", h.SoftDelete)

	// Global routes.
	e.POST("/api/v1/manage/global/domains", h.RegisterGlobal)
	e.GET("/api/v1/manage/global/domains", h.ListGlobal)
	e.GET("/api/v1/manage/global/domains/:id", h.GetGlobal)
	e.DELETE("/api/v1/manage/global/domains/:id", h.SoftDeleteGlobal)

	return e, h
}

// --- Tests ---

func TestDomainHTTPHandler_Register_GeneratesDKIM(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	var created *domain.Domain
	ds := &mockDomainStoreH{
		createFn: func(_ context.Context, d *domain.Domain) error {
			created = d
			return nil
		},
	}
	crypto := &mockCrypto{}
	jq := &mockJobQueueH{}

	e, _ := setupDomainTest(ds, crypto, jq, ts, wsStore)

	body := `{"domain_name":"example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/domains", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected domain to be created")
	}
	if created.DKIMPublicKey == "" {
		t.Fatal("expected non-empty DKIM public key")
	}
	if len(created.DNSRecords) != 1 {
		t.Fatalf("expected 1 DNS record, got %d", len(created.DNSRecords))
	}
	if created.Status != domain.DomainStatusPending {
		t.Fatalf("expected status 'pending', got %q", created.Status)
	}

	// Verify response contains DNS records but no private key.
	var resp map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := resp["dkim_private_key_encrypted"]; ok {
		t.Fatal("response should not contain dkim_private_key_encrypted")
	}
	if _, ok := resp["dns_records"]; !ok {
		t.Fatal("response should contain dns_records")
	}
}

func TestDomainHTTPHandler_List_NoPrivateKey(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	now := time.Now().UTC()
	ds := &mockDomainStoreH{
		listByWorkspaceFn: func(_ context.Context, _ *uuid.UUID, _ port.ListOptions) (*port.PageResult[domain.Domain], error) {
			return &port.PageResult[domain.Domain]{
				Items: []*domain.Domain{
					{
						ID: uuid.Must(uuid.NewV7()), WorkspaceID: &ws.ID, DomainName: "example.com",
						DKIMSelector: "senda", DKIMPublicKey: "pubkey", DKIMPrivateKeyEncrypted: []byte("secret"),
						DNSRecords: []map[string]any{{"type": "TXT"}}, Status: domain.DomainStatusPending,
						CreatedAt: now, UpdatedAt: now,
					},
				},
			}, nil
		},
	}

	e, _ := setupDomainTest(ds, &mockCrypto{}, &mockJobQueueH{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/domains", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

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
	if _, ok := items[0]["dkim_private_key_encrypted"]; ok {
		t.Fatal("response should not contain dkim_private_key_encrypted")
	}
}

func TestDomainHTTPHandler_Get_NoPrivateKey(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	domainID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	ds := &mockDomainStoreH{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Domain, error) {
			return &domain.Domain{
				ID: domainID, WorkspaceID: &ws.ID, DomainName: "example.com",
				DKIMSelector: "senda", DKIMPublicKey: "pubkey", DKIMPrivateKeyEncrypted: []byte("secret"),
				DNSRecords: []map[string]any{{"type": "TXT"}}, Status: domain.DomainStatusVerified,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	e, _ := setupDomainTest(ds, &mockCrypto{}, &mockJobQueueH{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/domains/"+domainID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["dkim_private_key_encrypted"]; ok {
		t.Fatal("response should not contain dkim_private_key_encrypted")
	}
}

func TestDomainHTTPHandler_VerifyNow_EnqueuesJob(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	domainID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	ds := &mockDomainStoreH{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Domain, error) {
			return &domain.Domain{
				ID: domainID, WorkspaceID: &ws.ID, DomainName: "example.com",
				DKIMSelector: "senda", Status: domain.DomainStatusPending,
				DNSRecords: []map[string]any{}, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	var enqueuedID uuid.UUID
	jq := &mockJobQueueH{
		enqueueDomainCheckFn: func(_ context.Context, id uuid.UUID) error {
			enqueuedID = id
			return nil
		},
	}

	e, _ := setupDomainTest(ds, &mockCrypto{}, jq, ts, wsStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/domains/"+domainID.String()+"/verify", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if enqueuedID != domainID {
		t.Fatalf("expected enqueued ID %s, got %s", domainID, enqueuedID)
	}
}

func TestDomainHTTPHandler_SoftDelete_Success(t *testing.T) {
	_, ws, ts, wsStore := testTenantAndWorkspace()

	domainID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var deletedID uuid.UUID
	ds := &mockDomainStoreH{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Domain, error) {
			return &domain.Domain{
				ID: domainID, WorkspaceID: &ws.ID, DomainName: "example.com",
				DKIMSelector: "senda", Status: domain.DomainStatusPending,
				DNSRecords: []map[string]any{}, CreatedAt: now, UpdatedAt: now,
			}, nil
		},
		softDeleteFn: func(_ context.Context, id uuid.UUID) error {
			deletedID = id
			return nil
		},
	}

	e, _ := setupDomainTest(ds, &mockCrypto{}, &mockJobQueueH{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/manage/tenants/acme/workspaces/default/domains/"+domainID.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if deletedID != domainID {
		t.Fatalf("expected deleted ID %s, got %s", domainID, deletedID)
	}
}

func TestDomainHTTPHandler_Register_InvalidDomain(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	e, _ := setupDomainTest(&mockDomainStoreH{}, &mockCrypto{}, &mockJobQueueH{}, ts, wsStore)

	body := `{"domain_name":"notadomain"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/domains", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDomainHTTPHandler_GlobalRegister_Success(t *testing.T) {
	var created *domain.Domain
	ds := &mockDomainStoreH{
		createFn: func(_ context.Context, d *domain.Domain) error {
			created = d
			return nil
		},
	}

	e, _ := setupDomainTest(ds, &mockCrypto{}, &mockJobQueueH{}, &mockTenantStore{}, &mockWorkspaceStore{})

	body := `{"domain_name":"global.example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/global/domains", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if created == nil {
		t.Fatal("expected domain to be created")
	}
	if created.WorkspaceID != nil {
		t.Fatal("expected nil workspace ID for global domain")
	}
}

func TestDomainHTTPHandler_Get_NotFound(t *testing.T) {
	_, _, ts, wsStore := testTenantAndWorkspace()

	ds := &mockDomainStoreH{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Domain, error) {
			return nil, domain.ErrNotFound
		},
	}

	e, _ := setupDomainTest(ds, &mockCrypto{}, &mockJobQueueH{}, ts, wsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/domains/"+uuid.Must(uuid.NewV7()).String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
