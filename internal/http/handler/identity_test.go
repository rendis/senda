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

func setupIdentityTest(
	svc *service.IdentityService,
	store port.AdapterIdentityStore,
	ts port.TenantStore,
	ws port.WorkspaceStore,
) (*echo.Echo, *handler.IdentityHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewIdentityHandler(svc, store, ts, ws)

	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities", h.List)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities", h.Create)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities/sync", h.Sync)
	e.DELETE("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities/:identity_id", h.Delete)
	e.POST("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities/:identity_id/set-default", h.SetDefault)
	e.GET("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities/:identity_id/workspace-access", h.GetWorkspaceAccess)
	e.PUT("/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/adapters/:id/identities/:identity_id/workspace-access", h.UpdateWorkspaceAccess)

	e.GET("/api/v1/manage/global/adapters/:id/identities", h.ListGlobal)
	e.POST("/api/v1/manage/global/adapters/:id/identities", h.CreateGlobal)
	e.POST("/api/v1/manage/global/adapters/:id/identities/sync", h.SyncGlobal)
	e.DELETE("/api/v1/manage/global/adapters/:id/identities/:identity_id", h.DeleteGlobal)
	e.POST("/api/v1/manage/global/adapters/:id/identities/:identity_id/set-default", h.SetDefaultGlobal)

	return e, h
}

func TestIdentityHandler_List_SharedSESFiltersGrantedEmailIdentities(t *testing.T) {
	tenant, ws, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	grantedIdentityID := uuid.Must(uuid.NewV7())

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

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWSID,
				Name:        "Shared SES",
				AdapterType: domain.AdapterTypeSES,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStore{}
	identitySvc := service.NewIdentityService(identityStore, as, &mockCrypto{}, nil)

	e, h := setupIdentityTest(identitySvc, identityStore, ts, wsStore)
	accessSvc := service.NewAdapterAccessService(
		as,
		identityStore,
		wsStore,
		&mockAdapterGrantStoreHandler{},
		&mockIdentityGrantStoreHandler{
			listGrantedIdentitiesFn: func(_ context.Context, gotAdapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error) {
				if gotAdapterID != adapterID || workspaceID != ws.ID {
					return nil, domain.ErrNotFound
				}
				return []*domain.AdapterIdentity{
					{
						ID:           grantedIdentityID,
						AdapterID:    adapterID,
						Identity:     "a@example.dev",
						IdentityType: domain.IdentityTypeEmail,
						Status:       domain.IdentityStatusVerified,
						CreatedAt:    time.Now().UTC(),
						UpdatedAt:    time.Now().UTC(),
					},
				}, nil
			},
		},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String()+"/identities", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"identity":"a@example.dev"`) {
		t.Fatalf("expected granted identity in response, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"identity_type":"domain"`) {
		t.Fatalf("expected shared SES list to exclude domain identities, got %s", rec.Body.String())
	}
}

func TestIdentityHandler_Create_SharedAdapterIsReadOnly(t *testing.T) {
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

	var created bool
	identityStore := &mockAdapterIdentityStore{
		createFn: func(_ context.Context, _ *domain.AdapterIdentity) error {
			created = true
			return nil
		},
	}

	as := &mockAdapterStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:          adapterID,
				WorkspaceID: &systemWSID,
				Name:        "Shared SES",
				AdapterType: domain.AdapterTypeSES,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
	}

	identitySvc := service.NewIdentityService(identityStore, as, &mockCrypto{}, nil)
	e, h := setupIdentityTest(identitySvc, identityStore, ts, wsStore)
	accessSvc := service.NewAdapterAccessService(
		as,
		identityStore,
		wsStore,
		&mockAdapterGrantStoreHandler{},
		&mockIdentityGrantStoreHandler{
			listGrantedIdentitiesFn: func(_ context.Context, gotAdapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error) {
				if gotAdapterID != adapterID || workspaceID != ws.ID {
					return nil, domain.ErrNotFound
				}
				return []*domain.AdapterIdentity{
					{ID: uuid.Must(uuid.NewV7()), AdapterID: adapterID, Identity: "a@example.dev", IdentityType: domain.IdentityTypeEmail},
				}, nil
			},
		},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	body := `{"identity":"new@example.dev"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/manage/tenants/acme/workspaces/default/adapters/"+adapterID.String()+"/identities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if created {
		t.Fatal("shared adapter create should not reach identity service")
	}
}

func TestIdentityHandler_GetWorkspaceAccess_SystemScope(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	workspaceAID := uuid.Must(uuid.NewV7())
	workspaceBID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())

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
				Name:        "Shared SES",
				AdapterType: domain.AdapterTypeSES,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}, nil
		},
	}

	identitySvc := service.NewIdentityService(identityStore, as, &mockCrypto{}, nil)
	e, h := setupIdentityTest(identitySvc, identityStore, ts, wsStore)
	accessSvc := service.NewAdapterAccessService(
		as,
		identityStore,
		wsStore,
		&mockAdapterGrantStoreHandler{},
		&mockIdentityGrantStoreHandler{
			listIdentityWorkspaceGrantsFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
				if id != identityID {
					return nil, domain.ErrNotFound
				}
				return []uuid.UUID{workspaceBID}, nil
			},
		},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/tenants/acme/workspaces/_system/adapters/"+adapterID.String()+"/identities/"+identityID.String()+"/workspace-access", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected non-empty workspace access payload")
	}
	if !strings.Contains(rec.Body.String(), `"code":"alpha"`) || !strings.Contains(rec.Body.String(), `"code":"beta"`) {
		t.Fatalf("expected tenant workspaces in payload, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"is_granted":true`) {
		t.Fatalf("expected granted workspace in payload, got %s", rec.Body.String())
	}
}

func TestIdentityHandler_UpdateWorkspaceAccess_WritesAudit(t *testing.T) {
	tenant, _, ts, wsStore := testTenantAndWorkspace()
	systemWSID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	identityID := uuid.Must(uuid.NewV7())
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
				Name:        "Shared SES",
				AdapterType: domain.AdapterTypeSES,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != identityID {
				return nil, domain.ErrNotFound
			}
			return &domain.AdapterIdentity{
				ID:           identityID,
				AdapterID:    adapterID,
				Identity:     "a@example.dev",
				IdentityType: domain.IdentityTypeEmail,
				Status:       domain.IdentityStatusVerified,
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}, nil
		},
	}
	identitySvc := service.NewIdentityService(identityStore, as, &mockCrypto{}, nil)

	e, h := setupIdentityTest(identitySvc, identityStore, ts, wsStore)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(middleware.ContextKeyMember, &domain.Member{ID: memberID, Email: "tenant-admin@example.com"})
			return next(c)
		}
	})

	currentGrants := []uuid.UUID{}
	accessSvc := service.NewAdapterAccessService(
		as,
		identityStore,
		wsStore,
		&mockAdapterGrantStoreHandler{},
		&mockIdentityGrantStoreHandler{
			listIdentityWorkspaceGrantsFn: func(_ context.Context, gotIdentityID uuid.UUID) ([]uuid.UUID, error) {
				if gotIdentityID != identityID {
					return nil, domain.ErrNotFound
				}
				return append([]uuid.UUID(nil), currentGrants...), nil
			},
			replaceIdentityWorkspaceGrantsFn: func(_ context.Context, gotIdentityID uuid.UUID, workspaceIDs []uuid.UUID) error {
				if gotIdentityID != identityID {
					return domain.ErrNotFound
				}
				currentGrants = append([]uuid.UUID(nil), workspaceIDs...)
				return nil
			},
		},
		&mockTemplateTypeUsageStoreHandler{},
	)
	h.SetAdapterAccessService(accessSvc)

	auditStore := &mockAuditLogStoreHandler{}
	h.SetAuditStore(auditStore)

	body := `{"workspace_ids":["` + workspaceAID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/tenants/acme/workspaces/_system/adapters/"+adapterID.String()+"/identities/"+identityID.String()+"/workspace-access", strings.NewReader(body))
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
	if entry.EntityType != "adapter_identity" || entry.EntityID != identityID {
		t.Fatalf("unexpected audit entity: %s %s", entry.EntityType, entry.EntityID)
	}
	if got := entry.ActorEmail; got != "tenant-admin@example.com" {
		t.Fatalf("expected actor email tenant-admin@example.com, got %s", got)
	}
	if after, ok := entry.Changes["after_workspace_ids"].([]string); !ok || len(after) != 1 || after[0] != workspaceAID.String() {
		t.Fatalf("unexpected after_workspace_ids: %#v", entry.Changes["after_workspace_ids"])
	}
	if metaIdentity, ok := entry.Metadata["identity"].(string); !ok || metaIdentity != "a@example.dev" {
		t.Fatalf("unexpected audit metadata identity: %#v", entry.Metadata["identity"])
	}
}
