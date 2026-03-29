package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
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

// --- Tests ---

func TestAPIKeyService_Generate_Success(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	memberID := uuid.Must(uuid.NewV7())

	var created *domain.APIKey
	store := &mockAPIKeyStore{
		createFn: func(_ context.Context, key *domain.APIKey) error {
			created = key
			return nil
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	fullKey, key, err := svc.Generate(context.Background(), wsID, "My API Key", memberID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Full key format: "senda_live_" + 32 hex chars = 43 chars total
	if !strings.HasPrefix(fullKey, "senda_live_") {
		t.Fatalf("expected prefix 'senda_live_', got %q", fullKey)
	}
	if len(fullKey) != 43 {
		t.Fatalf("expected key length 43, got %d", len(fullKey))
	}

	// Key object should have correct fields.
	if key.WorkspaceID != wsID {
		t.Fatalf("expected workspace ID %s, got %s", wsID, key.WorkspaceID)
	}
	if key.Name != "My API Key" {
		t.Fatalf("expected name 'My API Key', got %q", key.Name)
	}
	if key.CreatedBy != memberID {
		t.Fatalf("expected created_by %s, got %s", memberID, key.CreatedBy)
	}

	// KeyPrefix = literal "senda_live" per TECH_SPEC.
	if key.KeyPrefix != "senda_live" {
		t.Fatalf("expected prefix %q, got %q", "senda_live", key.KeyPrefix)
	}

	// KeyHint = last 8 chars of full key.
	if key.KeyHint != fullKey[len(fullKey)-8:] {
		t.Fatalf("expected hint %q, got %q", fullKey[len(fullKey)-8:], key.KeyHint)
	}

	// KeyHash should be a 64-char hex string (SHA-256).
	if len(key.KeyHash) != 64 {
		t.Fatalf("expected hash length 64, got %d", len(key.KeyHash))
	}

	// Must have been persisted.
	if created == nil {
		t.Fatal("expected key to be persisted")
	}
	if created.KeyHash != key.KeyHash {
		t.Fatalf("expected persisted hash to match")
	}
}

func TestAPIKeyService_Generate_StoreError(t *testing.T) {
	store := &mockAPIKeyStore{
		createFn: func(_ context.Context, _ *domain.APIKey) error {
			return domain.ErrConflict
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	_, _, err := svc.Generate(context.Background(), uuid.Must(uuid.NewV7()), "key", uuid.Must(uuid.NewV7()))
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestAPIKeyService_Generate_UniqueKeys(t *testing.T) {
	store := &mockAPIKeyStore{}
	svc := service.NewAPIKeyService(store, "test-pepper")

	key1, _, err := svc.Generate(context.Background(), uuid.Must(uuid.NewV7()), "k1", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key2, _, err := svc.Generate(context.Background(), uuid.Must(uuid.NewV7()), "k2", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key1 == key2 {
		t.Fatal("expected unique keys")
	}
}

func TestAPIKeyService_Validate_Success(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	keyID := uuid.Must(uuid.NewV7())

	// First generate a key to get its hash.
	var storedKey *domain.APIKey
	store := &mockAPIKeyStore{
		createFn: func(_ context.Context, key *domain.APIKey) error {
			storedKey = key
			return nil
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")
	fullKey, _, err := svc.Generate(context.Background(), wsID, "test", uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now set up the store to return the key when looked up by hash.
	storedKey.ID = keyID
	store.getByHashFn = func(_ context.Context, hash string) (*domain.APIKey, error) {
		if hash == storedKey.KeyHash {
			return storedKey, nil
		}
		return nil, domain.ErrNotFound
	}

	var touchedID uuid.UUID
	store.touchLastUsedFn = func(_ context.Context, id uuid.UUID) error {
		touchedID = id
		return nil
	}

	// Validate using the full key.
	result, err := svc.Validate(context.Background(), fullKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != keyID {
		t.Fatalf("expected key ID %s, got %s", keyID, result.ID)
	}
	if touchedID != keyID {
		t.Fatalf("expected TouchLastUsed called with %s, got %s", keyID, touchedID)
	}
}

func TestAPIKeyService_Validate_NotFound(t *testing.T) {
	store := &mockAPIKeyStore{
		getByHashFn: func(_ context.Context, _ string) (*domain.APIKey, error) {
			return nil, domain.ErrNotFound
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	_, err := svc.Validate(context.Background(), "senda_live_deadbeefdeadbeefdeadbeefdeadbeef")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAPIKeyService_Validate_Revoked(t *testing.T) {
	now := time.Now().UTC()
	store := &mockAPIKeyStore{
		getByHashFn: func(_ context.Context, _ string) (*domain.APIKey, error) {
			return &domain.APIKey{
				ID:        uuid.Must(uuid.NewV7()),
				RevokedAt: &now,
			}, nil
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	_, err := svc.Validate(context.Background(), "senda_live_deadbeefdeadbeefdeadbeefdeadbeef")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for revoked key, got %v", err)
	}
}

func TestAPIKeyService_Revoke_Success(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	keyID := uuid.Must(uuid.NewV7())
	var revokedID uuid.UUID
	store := &mockAPIKeyStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.APIKey, error) {
			if id != keyID {
				return nil, domain.ErrNotFound
			}
			return &domain.APIKey{ID: keyID, WorkspaceID: wsID}, nil
		},
		revokeFn: func(_ context.Context, id uuid.UUID) error {
			revokedID = id
			return nil
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	err := svc.Revoke(context.Background(), wsID, keyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revokedID != keyID {
		t.Fatalf("expected revoked ID %s, got %s", keyID, revokedID)
	}
}

func TestAPIKeyService_Revoke_CrossWorkspace(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	otherWsID := uuid.Must(uuid.NewV7())
	keyID := uuid.Must(uuid.NewV7())
	store := &mockAPIKeyStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.APIKey, error) {
			// Key exists but belongs to a different workspace.
			return &domain.APIKey{ID: id, WorkspaceID: otherWsID}, nil
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	err := svc.Revoke(context.Background(), wsID, keyID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-workspace revoke, got %v", err)
	}
}

func TestAPIKeyService_Revoke_StoreError(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	keyID := uuid.Must(uuid.NewV7())
	store := &mockAPIKeyStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.APIKey, error) {
			return &domain.APIKey{ID: id, WorkspaceID: wsID}, nil
		},
		revokeFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrNotFound
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	err := svc.Revoke(context.Background(), wsID, keyID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAPIKeyService_ListByWorkspace_Success(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	store := &mockAPIKeyStore{
		listByWorkspaceFn: func(_ context.Context, wID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.APIKey], error) {
			if wID != wsID {
				t.Fatalf("expected workspace ID %s, got %s", wsID, wID)
			}
			return &port.PageResult[domain.APIKey]{
				Items: []*domain.APIKey{
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: wsID, Name: "key1", KeyHint: "abcd1234", CreatedAt: now},
					{ID: uuid.Must(uuid.NewV7()), WorkspaceID: wsID, Name: "key2", KeyHint: "efgh5678", CreatedAt: now},
				},
				HasMore: false,
			}, nil
		},
	}

	svc := service.NewAPIKeyService(store, "test-pepper")

	page, err := svc.ListByWorkspace(context.Background(), wsID, port.ListOptions{Limit: 25})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
}
