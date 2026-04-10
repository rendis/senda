package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// APIKeyService handles API key generation, validation, and management.
type APIKeyService struct {
	store  port.APIKeyStore
	pepper string
}

// NewAPIKeyService creates a new APIKeyService.
// The pepper parameter is derived from the master key and used for HMAC-SHA256 hashing of API keys.
// app.go must pass a stable pepper derived from the master key (e.g. HKDF-derived subkey).
func NewAPIKeyService(store port.APIKeyStore, pepper string) *APIKeyService {
	return &APIKeyService{store: store, pepper: pepper}
}

// Generate creates a new API key for a workspace.
// Returns the full key (only shown once) and the persisted domain.APIKey.
// Format: "senda_<environment>_" + 32 random hex chars.
// Storage: HMAC-SHA256(fullKey) as KeyHash, persisted prefix metadata as "senda_<environment>", last 8 chars as KeyHint.
func (s *APIKeyService) Generate(ctx context.Context, workspaceID uuid.UUID, environment domain.Environment, name string, createdBy uuid.UUID) (string, *domain.APIKey, error) {
	fullKey, err := generateKey(environment)
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	key := &domain.APIKey{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: workspaceID,
		Name:        name,
		KeyHash:     hashKeyHMAC(fullKey, s.pepper),
		KeyPrefix:   environment.APIKeyPrefix(),
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
// Computes HMAC-SHA256(rawKey, pepper), looks up by hash, checks revocation, and touches last_used_at.
func (s *APIKeyService) Validate(ctx context.Context, rawKey string) (*domain.APIKey, error) {
	hash := hashKeyHMAC(rawKey, s.pepper)

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
	key, err := s.store.GetByID(ctx, keyID)
	if err != nil {
		return err
	}
	if key.WorkspaceID != workspaceID {
		return domain.ErrNotFound
	}

	return s.store.Revoke(ctx, keyID)
}

// ListByWorkspace returns paginated API keys (hash never exposed).
func (s *APIKeyService) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.APIKey], error) {
	return s.store.ListByWorkspace(ctx, workspaceID, opts)
}

func generateKey(environment domain.Environment) (string, error) {
	if !environment.Valid() {
		environment = domain.EnvironmentProd
	}
	b := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return environment.APIKeyTokenPrefix() + hex.EncodeToString(b), nil
}

// hashKeyHMAC computes HMAC-SHA256 of the key using the given pepper.
// This prevents offline brute-force attacks even if the database is compromised,
// because the attacker also needs the pepper (derived from the master key).
func hashKeyHMAC(key, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

// HashKeyPlain computes a plain SHA-256 hash of the key.
// Exported for use by the auth middleware as a backward-compatible fallback
// for keys created before the HMAC migration.
func HashKeyPlain(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// HashKeyHMAC is the exported version of hashKeyHMAC for use by the auth middleware.
func HashKeyHMAC(key, pepper string) string {
	return hashKeyHMAC(key, pepper)
}
