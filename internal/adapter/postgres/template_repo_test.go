//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

// --- TemplateType tests ---

func TestTemplateRepo_CreateType(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "welcome-email",
		Name:           "Welcome Email",
		VariableSchema: map[string]any{"type": "object"},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}
	if tt.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if tt.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestTemplateRepo_CreateType_WithWorkspace(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		WorkspaceID:    &ws.ID,
		Slug:           "ws-type",
		Name:           "WS Type",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}
}

func TestTemplateRepo_CreateType_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt1 := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "dup-type",
		Name:           "First",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt1); err != nil {
		t.Fatalf("first CreateType() error: %v", err)
	}

	tt2 := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "dup-type",
		Name:           "Second",
		VariableSchema: map[string]any{},
	}
	err := repo.CreateType(ctx, tt2)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestTemplateRepo_GetTypeBySlug(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	// Create global type
	globalType := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "resolve-slug",
		Name:           "Global",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, globalType); err != nil {
		t.Fatalf("CreateType(global) error: %v", err)
	}

	// Create workspace type with same slug
	wsType := &domain.TemplateType{
		ID:             uuid.New(),
		WorkspaceID:    &ws.ID,
		Slug:           "resolve-slug",
		Name:           "Workspace",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, wsType); err != nil {
		t.Fatalf("CreateType(ws) error: %v", err)
	}

	// Chain: workspace -> global. Should prefer workspace.
	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false},
	}

	got, err := repo.GetTypeBySlug(ctx, "resolve-slug", chain)
	if err != nil {
		t.Fatalf("GetTypeBySlug() error: %v", err)
	}
	if got.ID != wsType.ID {
		t.Errorf("expected workspace type (most specific), got ID %s", got.ID)
	}
}

func TestTemplateRepo_GetTypeBySlug_GlobalOnly(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	globalType := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "global-only-slug",
		Name:           "Global Only",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, globalType); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false},
	}

	got, err := repo.GetTypeBySlug(ctx, "global-only-slug", chain)
	if err != nil {
		t.Fatalf("GetTypeBySlug() error: %v", err)
	}
	if got.ID != globalType.ID {
		t.Errorf("expected global type ID %s, got %s", globalType.ID, got.ID)
	}
}

func TestTemplateRepo_GetTypeBySlug_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	chain := []uuid.NullUUID{{Valid: false}}
	_, err := repo.GetTypeBySlug(ctx, "nonexistent", chain)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTemplateRepo_FindTypeBySlugInScope(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "find-scope-slug",
		Name:           "Find Scope",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	got, err := repo.FindTypeBySlugInScope(ctx, "find-scope-slug", nil)
	if err != nil {
		t.Fatalf("FindTypeBySlugInScope() error: %v", err)
	}
	if got.ID != tt.ID {
		t.Errorf("expected ID %s, got %s", tt.ID, got.ID)
	}
}

func TestTemplateRepo_FindTypeBySlugInScope_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, err := repo.FindTypeBySlugInScope(ctx, "nope", nil)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

// --- Template tests ---

func TestTemplateRepo_CreateTemplate(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "tpl-type-" + uuid.New().String()[:8],
		Name:           "TPL Type",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	tpl := &domain.Template{
		ID:             uuid.New(),
		TemplateTypeID: tt.ID,
	}
	if err := repo.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("CreateTemplate() error: %v", err)
	}
	if tpl.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestTemplateRepo_CreateTemplate_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "conflict-type-" + uuid.New().String()[:8],
		Name:           "Conflict Type",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	tpl1 := &domain.Template{ID: uuid.New(), TemplateTypeID: tt.ID}
	if err := repo.CreateTemplate(ctx, tpl1); err != nil {
		t.Fatalf("first CreateTemplate() error: %v", err)
	}

	tpl2 := &domain.Template{ID: uuid.New(), TemplateTypeID: tt.ID}
	err := repo.CreateTemplate(ctx, tpl2)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 409 {
		t.Errorf("expected 409 Conflict, got: %v", err)
	}
}

func TestTemplateRepo_GetByTypeAndScope(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "scope-type-" + uuid.New().String()[:8],
		Name:           "Scope Type",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	tpl := &domain.Template{ID: uuid.New(), TemplateTypeID: tt.ID}
	if err := repo.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("CreateTemplate() error: %v", err)
	}

	got, err := repo.GetByTypeAndScope(ctx, tt.ID, nil)
	if err != nil {
		t.Fatalf("GetByTypeAndScope() error: %v", err)
	}
	if got.ID != tpl.ID {
		t.Errorf("expected ID %s, got %s", tpl.ID, got.ID)
	}
}

func TestTemplateRepo_GetByTypeAndScope_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, err := repo.GetByTypeAndScope(ctx, uuid.New(), nil)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTemplateRepo_SetDisabled_WorkspaceScope(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	tt := &domain.TemplateType{
		ID:          uuid.New(),
		WorkspaceID: &ws.ID,
		Slug:        "kill-switch-ws-" + uuid.New().String()[:8],
		Name:        "Kill Switch WS",
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	tpl := &domain.Template{
		ID:             uuid.New(),
		TemplateTypeID: tt.ID,
		WorkspaceID:    &ws.ID,
	}
	if err := repo.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("CreateTemplate() error: %v", err)
	}

	if err := repo.SetDisabled(ctx, tpl.ID, &ws.ID, true); err != nil {
		t.Fatalf("SetDisabled(true) error: %v", err)
	}

	got, err := repo.GetByTypeAndScope(ctx, tt.ID, &ws.ID)
	if err != nil {
		t.Fatalf("GetByTypeAndScope() error: %v", err)
	}
	if !got.IsDisabled {
		t.Fatal("expected template to be disabled")
	}

	if err := repo.SetDisabled(ctx, tpl.ID, &ws.ID, false); err != nil {
		t.Fatalf("SetDisabled(false) error: %v", err)
	}

	got, err = repo.GetByTypeAndScope(ctx, tt.ID, &ws.ID)
	if err != nil {
		t.Fatalf("GetByTypeAndScope() error: %v", err)
	}
	if got.IsDisabled {
		t.Fatal("expected template to be enabled")
	}
}

func TestTemplateRepo_SetDisabled_GlobalScope(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	tt := &domain.TemplateType{
		ID:   uuid.New(),
		Slug: "kill-switch-global-" + uuid.New().String()[:8],
		Name: "Kill Switch Global",
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	tpl := &domain.Template{
		ID:             uuid.New(),
		TemplateTypeID: tt.ID,
	}
	if err := repo.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("CreateTemplate() error: %v", err)
	}

	if err := repo.SetDisabled(ctx, tpl.ID, nil, true); err != nil {
		t.Fatalf("SetDisabled(true) error: %v", err)
	}

	got, err := repo.GetByTypeAndScope(ctx, tt.ID, nil)
	if err != nil {
		t.Fatalf("GetByTypeAndScope() error: %v", err)
	}
	if !got.IsDisabled {
		t.Fatal("expected global template to be disabled")
	}
}

func TestTemplateRepo_SetDisabled_ScopeMismatch(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	tt := &domain.TemplateType{
		ID:          uuid.New(),
		WorkspaceID: &ws.ID,
		Slug:        "kill-switch-mismatch-" + uuid.New().String()[:8],
		Name:        "Kill Switch Mismatch",
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	tpl := &domain.Template{
		ID:             uuid.New(),
		TemplateTypeID: tt.ID,
		WorkspaceID:    &ws.ID,
	}
	if err := repo.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("CreateTemplate() error: %v", err)
	}

	err := repo.SetDisabled(ctx, tpl.ID, nil, true)
	if err == nil {
		t.Fatal("expected not found error for scope mismatch")
	}

	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Fatalf("expected 404 not found, got %v", err)
	}
}

func TestTemplateRepo_ResolveTemplate(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)
	tenantRepo := pgadapter.NewTenantRepo(pool)
	wsRepo := pgadapter.NewWorkspaceRepo(pool)
	ws := createTestWorkspaceWith(ctx, t, tenantRepo, wsRepo)

	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "resolve-type-" + uuid.New().String()[:8],
		Name:           "Resolve Type",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}

	// Global template
	globalTpl := &domain.Template{ID: uuid.New(), TemplateTypeID: tt.ID}
	if err := repo.CreateTemplate(ctx, globalTpl); err != nil {
		t.Fatalf("CreateTemplate(global) error: %v", err)
	}

	// Workspace template
	wsTpl := &domain.Template{ID: uuid.New(), TemplateTypeID: tt.ID, WorkspaceID: &ws.ID}
	if err := repo.CreateTemplate(ctx, wsTpl); err != nil {
		t.Fatalf("CreateTemplate(ws) error: %v", err)
	}

	chain := []uuid.NullUUID{
		{UUID: ws.ID, Valid: true},
		{Valid: false},
	}

	got, err := repo.ResolveTemplate(ctx, tt.ID, chain)
	if err != nil {
		t.Fatalf("ResolveTemplate() error: %v", err)
	}
	if got.ID != wsTpl.ID {
		t.Errorf("expected workspace template (most specific), got ID %s", got.ID)
	}
}

func TestTemplateRepo_ResolveTemplate_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	chain := []uuid.NullUUID{{Valid: false}}
	_, err := repo.ResolveTemplate(ctx, uuid.New(), chain)
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

// --- Version tests ---

func createTestTemplateWithType(ctx context.Context, t *testing.T, repo *pgadapter.TemplateRepo) (*domain.TemplateType, *domain.Template) {
	t.Helper()
	tt := &domain.TemplateType{
		ID:             uuid.New(),
		Slug:           "ver-type-" + uuid.New().String()[:8],
		Name:           "Ver Type",
		VariableSchema: map[string]any{},
	}
	if err := repo.CreateType(ctx, tt); err != nil {
		t.Fatalf("CreateType() error: %v", err)
	}
	tpl := &domain.Template{ID: uuid.New(), TemplateTypeID: tt.ID}
	if err := repo.CreateTemplate(ctx, tpl); err != nil {
		t.Fatalf("CreateTemplate() error: %v", err)
	}
	return tt, tpl
}

func TestTemplateRepo_CreateVersion(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	ver := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "Welcome!",
		PreviewText:   "Check it out",
		FromName:      "Test",
		BodyMJML:      "<mjml></mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}
	if ver.VersionNumber != 1 {
		t.Errorf("expected version_number 1, got %d", ver.VersionNumber)
	}
	if ver.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestTemplateRepo_CreateVersion_AutoIncrement(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	for i := 1; i <= 3; i++ {
		ver := &domain.TemplateVersion{
			ID:            uuid.New(),
			TemplateID:    tpl.ID,
			Status:        domain.VersionStatusDraft,
			Subject:       "Subject",
			BodyMJML:      "<mjml></mjml>",
			DefaultLocale: "en",
		}
		if err := repo.CreateVersion(ctx, ver); err != nil {
			t.Fatalf("CreateVersion(%d) error: %v", i, err)
		}
		if ver.VersionNumber != i {
			t.Errorf("expected version_number %d, got %d", i, ver.VersionNumber)
		}
	}
}

func TestTemplateRepo_CreateVersion_Concurrent_NoDuplicates(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	const workers = 24
	errCh := make(chan error, workers)
	versionCh := make(chan int, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ver := &domain.TemplateVersion{
				ID:            uuid.New(),
				TemplateID:    tpl.ID,
				Status:        domain.VersionStatusDraft,
				Subject:       "Concurrent",
				BodyMJML:      "<mjml></mjml>",
				DefaultLocale: "en",
			}
			if err := repo.CreateVersion(ctx, ver); err != nil {
				errCh <- err
				return
			}
			versionCh <- ver.VersionNumber
		}()
	}

	wg.Wait()
	close(errCh)
	close(versionCh)

	for err := range errCh {
		t.Fatalf("CreateVersion(concurrent) error: %v", err)
	}

	seen := make(map[int]bool, workers)
	for v := range versionCh {
		if seen[v] {
			t.Fatalf("duplicate version number detected: %d", v)
		}
		seen[v] = true
	}

	if len(seen) != workers {
		t.Fatalf("expected %d created versions, got %d", workers, len(seen))
	}
	for i := 1; i <= workers; i++ {
		if !seen[i] {
			t.Fatalf("missing version number %d", i)
		}
	}

	versions, err := repo.ListVersions(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}
	if len(versions) != workers {
		t.Fatalf("expected %d persisted versions, got %d", workers, len(versions))
	}
}

func TestTemplateRepo_Publish(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	ver := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "Publish Me",
		BodyMJML:      "<mjml></mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}

	if err := repo.Publish(ctx, ver.ID); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	published, err := repo.GetPublishedVersion(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("GetPublishedVersion() error: %v", err)
	}
	if published.ID != ver.ID {
		t.Errorf("expected published version ID %s, got %s", ver.ID, published.ID)
	}
	if published.Status != domain.VersionStatusPublished {
		t.Errorf("expected status published, got %q", published.Status)
	}
	if published.PublishedAt == nil {
		t.Error("expected published_at to be set")
	}
}

func TestTemplateRepo_Publish_ArchivesPrevious(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	ver1 := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "V1",
		BodyMJML:      "<mjml>v1</mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver1); err != nil {
		t.Fatalf("CreateVersion(v1) error: %v", err)
	}
	if err := repo.Publish(ctx, ver1.ID); err != nil {
		t.Fatalf("Publish(v1) error: %v", err)
	}

	ver2 := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "V2",
		BodyMJML:      "<mjml>v2</mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver2); err != nil {
		t.Fatalf("CreateVersion(v2) error: %v", err)
	}
	if err := repo.Publish(ctx, ver2.ID); err != nil {
		t.Fatalf("Publish(v2) error: %v", err)
	}

	// ver2 should now be published
	published, err := repo.GetPublishedVersion(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("GetPublishedVersion() error: %v", err)
	}
	if published.ID != ver2.ID {
		t.Errorf("expected published version to be v2, got ID %s", published.ID)
	}

	// ver1 should be archived
	versions, err := repo.ListVersions(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}
	for _, v := range versions {
		if v.ID == ver1.ID && v.Status != domain.VersionStatusArchived {
			t.Errorf("expected ver1 to be archived, got status %q", v.Status)
		}
	}
}

func TestTemplateRepo_GetPublishedVersion_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, err := repo.GetPublishedVersion(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTemplateRepo_ListVersions(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	for range 3 {
		ver := &domain.TemplateVersion{
			ID:            uuid.New(),
			TemplateID:    tpl.ID,
			Status:        domain.VersionStatusDraft,
			Subject:       "Subject",
			BodyMJML:      "<mjml></mjml>",
			DefaultLocale: "en",
		}
		if err := repo.CreateVersion(ctx, ver); err != nil {
			t.Fatalf("CreateVersion() error: %v", err)
		}
	}

	versions, err := repo.ListVersions(ctx, tpl.ID)
	if err != nil {
		t.Fatalf("ListVersions() error: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Should be ordered by version_number DESC
	if versions[0].VersionNumber != 3 {
		t.Errorf("expected first version to be 3, got %d", versions[0].VersionNumber)
	}
	if versions[2].VersionNumber != 1 {
		t.Errorf("expected last version to be 1, got %d", versions[2].VersionNumber)
	}
}

// --- Locale tests ---

func TestTemplateRepo_SetLocale(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	ver := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "Subject",
		BodyMJML:      "<mjml></mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}

	subj := "Bienvenido"
	locale := &domain.TemplateVersionLocale{
		ID:                uuid.New(),
		TemplateVersionID: ver.ID,
		Locale:            "es",
		Subject:           &subj,
	}
	if err := repo.SetLocale(ctx, locale); err != nil {
		t.Fatalf("SetLocale() error: %v", err)
	}
	if locale.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestTemplateRepo_SetLocale_Upsert(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	ver := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "Subject",
		BodyMJML:      "<mjml></mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}

	subj1 := "Bienvenido"
	locale1 := &domain.TemplateVersionLocale{
		ID:                uuid.New(),
		TemplateVersionID: ver.ID,
		Locale:            "es",
		Subject:           &subj1,
	}
	if err := repo.SetLocale(ctx, locale1); err != nil {
		t.Fatalf("SetLocale(first) error: %v", err)
	}

	// Upsert with different subject
	subj2 := "Hola"
	locale2 := &domain.TemplateVersionLocale{
		ID:                uuid.New(),
		TemplateVersionID: ver.ID,
		Locale:            "es",
		Subject:           &subj2,
	}
	if err := repo.SetLocale(ctx, locale2); err != nil {
		t.Fatalf("SetLocale(upsert) error: %v", err)
	}

	got, err := repo.GetLocale(ctx, ver.ID, "es")
	if err != nil {
		t.Fatalf("GetLocale() error: %v", err)
	}
	if got.Subject == nil || *got.Subject != "Hola" {
		t.Errorf("expected subject 'Hola', got %v", got.Subject)
	}
}

func TestTemplateRepo_GetLocale(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, tpl := createTestTemplateWithType(ctx, t, repo)

	ver := &domain.TemplateVersion{
		ID:            uuid.New(),
		TemplateID:    tpl.ID,
		Status:        domain.VersionStatusDraft,
		Subject:       "Subject",
		BodyMJML:      "<mjml></mjml>",
		DefaultLocale: "en",
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}

	subj := "Bem-vindo"
	body := "<mjml>pt</mjml>"
	locale := &domain.TemplateVersionLocale{
		ID:                uuid.New(),
		TemplateVersionID: ver.ID,
		Locale:            "pt-BR",
		Subject:           &subj,
		BodyMJML:          &body,
	}
	if err := repo.SetLocale(ctx, locale); err != nil {
		t.Fatalf("SetLocale() error: %v", err)
	}

	got, err := repo.GetLocale(ctx, ver.ID, "pt-BR")
	if err != nil {
		t.Fatalf("GetLocale() error: %v", err)
	}
	if got.Locale != "pt-BR" {
		t.Errorf("expected locale pt-BR, got %q", got.Locale)
	}
	if got.Subject == nil || *got.Subject != "Bem-vindo" {
		t.Errorf("expected subject Bem-vindo, got %v", got.Subject)
	}
	if got.BodyMJML == nil || *got.BodyMJML != body {
		t.Errorf("expected body_mjml, got %v", got.BodyMJML)
	}
}

func TestTemplateRepo_GetLocale_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	_, err := repo.GetLocale(ctx, uuid.New(), "xx")
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}

func TestTemplateRepo_DeleteLocale_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewTemplateRepo(pool)

	err := repo.DeleteLocale(ctx, uuid.New(), "xx")
	if err == nil {
		t.Fatal("expected not found error")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("expected 404, got: %v", err)
	}
}
