//go:build integration

package postgres_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

func TestAdapterGrantRepo_ListVisibleAdaptersForWorkspace_IncludesOwnedAndShared(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	adapterRepo := pgadapter.NewAdapterRepo(pool)
	identityRepo := pgadapter.NewAdapterIdentityRepo(pool)
	grantRepo := pgadapter.NewAdapterGrantRepo(pool)
	identityGrantRepo := pgadapter.NewAdapterIdentityGrantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	systemWS := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true}
	workspace := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "alpha", Name: "Alpha"}
	if err := wsRepo.Create(ctx, systemWS); err != nil {
		t.Fatalf("create system workspace: %v", err)
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	ownedAdapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &workspace.ID,
		Name:               "Owned Gmail",
		AdapterType:        domain.AdapterTypeGmail,
		ConfigEncrypted:    []byte("owned"),
		RateLimitPerSecond: 10,
	}
	sharedGmail := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System Gmail",
		AdapterType:        domain.AdapterTypeGmail,
		ConfigEncrypted:    []byte("gmail"),
		RateLimitPerSecond: 10,
	}
	sharedSES := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System SES",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("ses"),
		RateLimitPerSecond: 10,
	}
	sharedSMTP := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System SMTP",
		AdapterType:        domain.AdapterTypeSMTP,
		ConfigEncrypted:    []byte("smtp"),
		RateLimitPerSecond: 10,
	}
	for _, adapter := range []*domain.Adapter{ownedAdapter, sharedGmail, sharedSES, sharedSMTP} {
		if err := adapterRepo.Create(ctx, adapter); err != nil {
			t.Fatalf("create adapter %s: %v", adapter.Name, err)
		}
	}

	emailIdentity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      sharedSES.ID,
		Identity:       "a@example.dev",
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := identityRepo.Create(ctx, emailIdentity); err != nil {
		t.Fatalf("create email identity: %v", err)
	}
	smtpIdentity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      sharedSMTP.ID,
		Identity:       "smtp@example.dev",
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := identityRepo.Create(ctx, smtpIdentity); err != nil {
		t.Fatalf("create smtp identity: %v", err)
	}

	if err := grantRepo.ReplaceAdapterWorkspaceGrants(ctx, sharedGmail.ID, []uuid.UUID{workspace.ID}); err != nil {
		t.Fatalf("grant gmail adapter: %v", err)
	}
	if err := identityGrantRepo.ReplaceIdentityWorkspaceGrants(ctx, emailIdentity.ID, []uuid.UUID{workspace.ID}); err != nil {
		t.Fatalf("grant ses identity: %v", err)
	}
	if err := identityGrantRepo.ReplaceIdentityWorkspaceGrants(ctx, smtpIdentity.ID, []uuid.UUID{workspace.ID}); err != nil {
		t.Fatalf("grant smtp identity: %v", err)
	}

	page, err := grantRepo.ListVisibleAdaptersForWorkspace(ctx, workspace.ID, port.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListVisibleAdaptersForWorkspace() error: %v", err)
	}
	if len(page.Items) != 4 {
		t.Fatalf("expected 4 visible adapters, got %d", len(page.Items))
	}

	names := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		names = append(names, item.Name)
	}
	sort.Strings(names)
	expected := []string{"Owned Gmail", "System Gmail", "System SES", "System SMTP"}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("expected visible adapters %v, got %v", expected, names)
		}
	}
}

func TestAdapterIdentityGrantRepo_ListGrantedIdentitiesForWorkspace_FiltersGrantedEmails(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	adapterRepo := pgadapter.NewAdapterRepo(pool)
	identityRepo := pgadapter.NewAdapterIdentityRepo(pool)
	identityGrantRepo := pgadapter.NewAdapterIdentityGrantRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	systemWS := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true}
	workspace := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "alpha", Name: "Alpha"}
	if err := wsRepo.Create(ctx, systemWS); err != nil {
		t.Fatalf("create system workspace: %v", err)
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System SES",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("ses"),
		RateLimitPerSecond: 10,
	}
	if err := adapterRepo.Create(ctx, adapter); err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	domainIdentity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      adapter.ID,
		Identity:       "example.dev",
		IdentityType:   domain.IdentityTypeDomain,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceProvider,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	grantedIdentity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      adapter.ID,
		Identity:       "a@example.dev",
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	otherIdentity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      adapter.ID,
		Identity:       "b@example.dev",
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	for _, identity := range []*domain.AdapterIdentity{domainIdentity, grantedIdentity, otherIdentity} {
		if err := identityRepo.Create(ctx, identity); err != nil {
			t.Fatalf("create identity %s: %v", identity.Identity, err)
		}
	}
	if err := identityGrantRepo.ReplaceIdentityWorkspaceGrants(ctx, grantedIdentity.ID, []uuid.UUID{workspace.ID}); err != nil {
		t.Fatalf("grant identity: %v", err)
	}

	identities, err := identityGrantRepo.ListGrantedIdentitiesForWorkspace(ctx, adapter.ID, workspace.ID)
	if err != nil {
		t.Fatalf("ListGrantedIdentitiesForWorkspace() error: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected 1 granted identity, got %d", len(identities))
	}
	if identities[0].Identity != "a@example.dev" {
		t.Fatalf("expected a@example.dev, got %q", identities[0].Identity)
	}
}

func TestAdapterAccessService_ReplaceIdentityWorkspaceAccess_BlocksRevokeWhenInUse_Integration(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	adapterRepo := pgadapter.NewAdapterRepo(pool)
	identityRepo := pgadapter.NewAdapterIdentityRepo(pool)
	grantRepo := pgadapter.NewAdapterGrantRepo(pool)
	identityGrantRepo := pgadapter.NewAdapterIdentityGrantRepo(pool)
	templateRepo := pgadapter.NewTemplateRepo(pool)
	usageRepo := pgadapter.NewTemplateTypeUsageRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	systemWS := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true}
	workspace := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "alpha", Name: "Alpha"}
	if err := wsRepo.Create(ctx, systemWS); err != nil {
		t.Fatalf("create system workspace: %v", err)
	}
	if err := wsRepo.Create(ctx, workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System SES",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("ses"),
		RateLimitPerSecond: 10,
	}
	if err := adapterRepo.Create(ctx, adapter); err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	identity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      adapter.ID,
		Identity:       "a@example.dev",
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := identityRepo.Create(ctx, identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	if err := identityGrantRepo.ReplaceIdentityWorkspaceGrants(ctx, identity.ID, []uuid.UUID{workspace.ID}); err != nil {
		t.Fatalf("grant identity: %v", err)
	}

	templateType := &domain.TemplateType{
		ID:               uuid.New(),
		Slug:             "welcome",
		Name:             "Welcome",
		WorkspaceID:      &workspace.ID,
		AdapterID:        &adapter.ID,
		SenderIdentityID: &identity.ID,
	}
	if err := templateRepo.CreateType(ctx, templateType); err != nil {
		t.Fatalf("create template type: %v", err)
	}

	accessSvc := service.NewAdapterAccessService(adapterRepo, identityRepo, wsRepo, grantRepo, identityGrantRepo, usageRepo)
	err := accessSvc.ReplaceIdentityWorkspaceAccess(ctx, systemWS, adapter.ID, identity.ID, nil)
	if err == nil || err != domain.ErrSharedGrantInUse {
		t.Fatalf("expected ErrSharedGrantInUse, got %v", err)
	}
}

func TestTemplateTypeUsageRepo_ListWorkspacesUsingSenderIdentity_Integration(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	adapterRepo := pgadapter.NewAdapterRepo(pool)
	identityRepo := pgadapter.NewAdapterIdentityRepo(pool)
	templateRepo := pgadapter.NewTemplateRepo(pool)
	usageRepo := pgadapter.NewTemplateTypeUsageRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	systemWS := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true}
	wsA := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "alpha", Name: "Alpha"}
	wsB := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "beta", Name: "Beta"}
	wsC := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "gamma", Name: "Gamma"}
	for _, ws := range []*domain.Workspace{systemWS, wsA, wsB, wsC} {
		if err := wsRepo.Create(ctx, ws); err != nil {
			t.Fatalf("create workspace %s: %v", ws.Code, err)
		}
	}

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System SES",
		AdapterType:        domain.AdapterTypeSES,
		ConfigEncrypted:    []byte("ses"),
		RateLimitPerSecond: 10,
	}
	if err := adapterRepo.Create(ctx, adapter); err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	identity := &domain.AdapterIdentity{
		ID:             uuid.New(),
		AdapterID:      adapter.ID,
		Identity:       "a@example.dev",
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := identityRepo.Create(ctx, identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}

	templateTypeA := &domain.TemplateType{
		ID:               uuid.New(),
		Slug:             "welcome-a",
		Name:             "Welcome A",
		WorkspaceID:      &wsA.ID,
		AdapterID:        &adapter.ID,
		SenderIdentityID: &identity.ID,
	}
	templateTypeB := &domain.TemplateType{
		ID:               uuid.New(),
		Slug:             "welcome-b",
		Name:             "Welcome B",
		WorkspaceID:      &wsB.ID,
		AdapterID:        &adapter.ID,
		SenderIdentityID: &identity.ID,
	}
	for _, tt := range []*domain.TemplateType{templateTypeA, templateTypeB} {
		if err := templateRepo.CreateType(ctx, tt); err != nil {
			t.Fatalf("create template type %s: %v", tt.Slug, err)
		}
	}

	identityGrantRepo := pgadapter.NewAdapterIdentityGrantRepo(pool)
	if err := identityGrantRepo.ReplaceIdentityWorkspaceGrants(ctx, identity.ID, []uuid.UUID{wsA.ID, wsB.ID}); err != nil {
		t.Fatalf("grant identity: %v", err)
	}

	got, err := usageRepo.ListWorkspacesUsingSenderIdentity(ctx, identity.ID, []uuid.UUID{wsA.ID, wsC.ID, wsB.ID, wsA.ID})
	if err != nil {
		t.Fatalf("ListWorkspacesUsingSenderIdentity() error: %v", err)
	}

	want := []uuid.UUID{wsA.ID, wsB.ID}
	if len(got) != len(want) {
		t.Fatalf("expected %d workspaces in use, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("workspace[%d] = %s, want %s (got=%v)", i, got[i], want[i], got)
		}
	}
}

func TestTemplateTypeUsageRepo_ListWorkspacesUsingAdapter_Integration(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	adapterRepo := pgadapter.NewAdapterRepo(pool)
	templateRepo := pgadapter.NewTemplateRepo(pool)
	usageRepo := pgadapter.NewTemplateTypeUsageRepo(pool)

	tenant := createTestTenant(ctx, t, tenantRepo)
	systemWS := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "_system", Name: "System", IsSystem: true}
	wsA := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "alpha", Name: "Alpha"}
	wsB := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "beta", Name: "Beta"}
	wsC := &domain.Workspace{ID: uuid.New(), TenantID: tenant.ID, Code: "gamma", Name: "Gamma"}
	for _, ws := range []*domain.Workspace{systemWS, wsA, wsB, wsC} {
		if err := wsRepo.Create(ctx, ws); err != nil {
			t.Fatalf("create workspace %s: %v", ws.Code, err)
		}
	}

	adapter := &domain.Adapter{
		ID:                 uuid.New(),
		WorkspaceID:        &systemWS.ID,
		Name:               "System Gmail",
		AdapterType:        domain.AdapterTypeGmail,
		ConfigEncrypted:    []byte("gmail"),
		RateLimitPerSecond: 10,
	}
	if err := adapterRepo.Create(ctx, adapter); err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	for _, tt := range []*domain.TemplateType{
		{ID: uuid.New(), Slug: "welcome-a", Name: "Welcome A", WorkspaceID: &wsA.ID, AdapterID: &adapter.ID},
		{ID: uuid.New(), Slug: "welcome-b", Name: "Welcome B", WorkspaceID: &wsB.ID, AdapterID: &adapter.ID},
	} {
		if err := templateRepo.CreateType(ctx, tt); err != nil {
			t.Fatalf("create template type %s: %v", tt.Slug, err)
		}
	}

	got, err := usageRepo.ListWorkspacesUsingAdapter(ctx, adapter.ID, []uuid.UUID{wsC.ID, wsA.ID, wsB.ID, wsA.ID})
	if err != nil {
		t.Fatalf("ListWorkspacesUsingAdapter() error: %v", err)
	}

	want := []uuid.UUID{wsA.ID, wsB.ID}
	if len(got) != len(want) {
		t.Fatalf("expected %d workspaces in use, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("workspace[%d] = %s, want %s (got=%v)", i, got[i], want[i], got)
		}
	}
}
