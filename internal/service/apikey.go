package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// APIKeyService handles API key generation, validation, and management.
type APIKeyService struct {
	store port.APIKeyStore
}

// NewAPIKeyService creates a new APIKeyService.
func NewAPIKeyService(store port.APIKeyStore) *APIKeyService {
	return &APIKeyService{store: store}
}

// Generate creates a new API key for a workspace.
// Returns the full key (only shown once) and the persisted domain.APIKey.
// Format: "senda_live_" + 32 random hex chars = 43 chars total.
// Storage: SHA-256(fullKey) as KeyHash, first 8 hex chars as KeyPrefix, last 8 chars as KeyHint.
func (s *APIKeyService) Generate(ctx context.Context, workspaceID uuid.UUID, name string, createdBy uuid.UUID) (string, *domain.APIKey, error) {
	fullKey, err := generateKey()
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	key := &domain.APIKey{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID,
		Name:        name,
		KeyHash:     hashKey(fullKey),
		KeyPrefix:   "senda_live",
		KeyHint:     fullKey[len(fullKey)-8:],
		CreatedBy:   createdBy,
		CreatedAt:   now,
	}

	if err := s.store.Create(ctx, key); err != nil {
		return "", nil, err
	}

	return fullKey, key, nil
}

// Validate checks if a raw key is valid and not revoked.
// Computes SHA-256(rawKey), looks up by hash, checks revocation, and touches last_used_at.
func (s *APIKeyService) Validate(ctx context.Context, rawKey string) (*domain.APIKey, error) {
	hash := hashKey(rawKey)

	key, err := s.store.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if key.RevokedAt != nil {
		// Revoked keys behave as if they don't exist from the caller's perspective.
		return nil, domain.ErrNotFound
	}

	// Best-effort update of last_used_at; do not fail validation on touch error.
	_ = s.store.TouchLastUsed(ctx, key.ID)

	return key, nil
}

// Revoke marks an API key as revoked.
// It verifies the key belongs to the given workspace before revoking.
// If the key is not found in the workspace, returns domain.ErrNotFound.
func (s *APIKeyService) Revoke(ctx context.Context, workspaceID uuid.UUID, keyID uuid.UUID) error {
	// Verify the key belongs to this workspace by scanning workspace keys.
	// APIKeyStore has no GetByID, so we use ListByWorkspace.
	// Use a large limit to capture all keys; API keys per workspace are bounded in practice.
	page, err := s.store.ListByWorkspace(ctx, workspaceID, port.ListOptions{Limit: 100})
	if err != nil {
		return err
	}

	found := false
	for _, k := range page.Items {
		if k.ID == keyID {
			found = true
			break
		}
	}
	if !found {
		return domain.ErrNotFound
	}

	return s.store.Revoke(ctx, keyID)
}

// ListByWorkspace returns paginated API keys (hash never exposed).
func (s *APIKeyService) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.APIKey], error) {
	return s.store.ListByWorkspace(ctx, workspaceID, opts)
}

func generateKey() (string, error) {
	b := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "senda_live_" + hex.EncodeToString(b), nil
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
