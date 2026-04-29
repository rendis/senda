package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// IdentityProviderFactory creates an IdentityProvider from an adapter and its decrypted config.
// Returns nil for adapters that don't support identity listing (e.g., SMTP).
type IdentityProviderFactory func(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.IdentityProvider, error)

// IdentityService manages adapter sending identities.
type IdentityService struct {
	identityStore   port.AdapterIdentityStore
	adapterStore    port.AdapterStore
	crypto          port.Crypto
	providerFactory IdentityProviderFactory
}

// NewIdentityService creates a new IdentityService.
func NewIdentityService(
	identityStore port.AdapterIdentityStore,
	adapterStore port.AdapterStore,
	crypto port.Crypto,
	providerFactory IdentityProviderFactory,
) *IdentityService {
	return &IdentityService{
		identityStore:   identityStore,
		adapterStore:    adapterStore,
		crypto:          crypto,
		providerFactory: providerFactory,
	}
}

// SyncIdentities fetches identities from the provider and syncs them to the store.
func (s *IdentityService) SyncIdentities(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}

	decrypted, err := s.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt adapter config: %w", err)
	}

	provider, err := s.providerFactory(ctx, adapter, decrypted)
	if err != nil {
		return nil, fmt.Errorf("create identity provider: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("adapter type %s does not support identity listing", adapter.AdapterType)
	}

	providerIdentities, err := provider.ListIdentities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list provider identities: %w", err)
	}

	now := time.Now().UTC()
	identities := make([]*domain.AdapterIdentity, 0, len(providerIdentities))
	keepNames := make([]string, 0, len(providerIdentities))

	for _, pi := range providerIdentities {
		// Only import domain identities from the provider. Individual email identities
		// from SES are often sandbox testing artifacts (verified recipients, not senders).
		// Users add specific sender emails manually via the UI, validated against verified domains.
		if pi.IdentityType != "domain" {
			continue
		}

		identities = append(identities, &domain.AdapterIdentity{
			ID:             uuid.Must(uuid.NewV7()),
			AdapterID:      adapterID,
			Identity:       pi.Identity,
			IdentityType:   domain.IdentityType(pi.IdentityType),
			Status:         domain.IdentityStatus(pi.VerificationStatus),
			SendingEnabled: pi.SendingEnabled,
			Source:         domain.IdentitySourceProvider,
			LastSyncedAt:   &now,
		})
		keepNames = append(keepNames, pi.Identity)
	}

	if err := s.identityStore.UpsertBatch(ctx, adapterID, identities); err != nil {
		return nil, fmt.Errorf("upsert identities: %w", err)
	}

	if err := s.identityStore.DeleteStale(ctx, adapterID, keepNames); err != nil {
		return nil, fmt.Errorf("delete stale identities: %w", err)
	}

	return s.identityStore.ListByAdapter(ctx, adapterID)
}

// GetByID returns a single identity by ID.
func (s *IdentityService) GetByID(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
	return s.identityStore.GetByID(ctx, id)
}

// ListIdentities returns all identities for an adapter from the store.
func (s *IdentityService) ListIdentities(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error) {
	return s.identityStore.ListByAdapter(ctx, adapterID)
}

// GetDefault returns the default identity for an adapter.
func (s *IdentityService) GetDefault(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error) {
	return s.identityStore.GetDefault(ctx, adapterID)
}

// SetDefault marks an identity as the default for its adapter.
func (s *IdentityService) SetDefault(ctx context.Context, adapterID uuid.UUID, identityID uuid.UUID) error {
	return s.identityStore.SetDefault(ctx, adapterID, identityID)
}

// CreateManual creates a manually-configured identity for an adapter.
// Validates that the email domain exists in the adapter's verified domains.
func (s *IdentityService) CreateManual(ctx context.Context, adapterID uuid.UUID, email string, displayName *string) (*domain.AdapterIdentity, error) {
	emailDomain := extractDomain(email)
	if emailDomain == "" {
		return nil, fmt.Errorf("%w: invalid email address", domain.ErrValidation)
	}

	adapter, err := s.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}

	if adapter.AdapterType != domain.AdapterTypeSMTP {
		// Check that this email's domain exists as a verified domain identity.
		existing, err := s.identityStore.ListByAdapter(ctx, adapterID)
		if err != nil {
			return nil, err
		}

		domainVerified := false
		for _, ident := range existing {
			if ident.IdentityType == domain.IdentityTypeDomain &&
				ident.Identity == emailDomain &&
				ident.Status == domain.IdentityStatusVerified {
				domainVerified = true
				break
			}
		}
		if !domainVerified {
			return nil, fmt.Errorf("%w: domain %s is not verified in this adapter", domain.ErrIdentityNotInDomain, emailDomain)
		}
	}

	now := time.Now().UTC()
	identity := &domain.AdapterIdentity{
		ID:             uuid.Must(uuid.NewV7()),
		AdapterID:      adapterID,
		Identity:       email,
		IdentityType:   domain.IdentityTypeEmail,
		Status:         domain.IdentityStatusVerified,
		SendingEnabled: true,
		DisplayName:    displayName,
		Source:         domain.IdentitySourceManual,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.identityStore.Create(ctx, identity); err != nil {
		return nil, err
	}

	return identity, nil
}

// DeleteIdentity deletes a manually-configured identity.
func (s *IdentityService) DeleteIdentity(ctx context.Context, identityID uuid.UUID) error {
	identity, err := s.identityStore.GetByID(ctx, identityID)
	if err != nil {
		return err
	}
	if identity.Source != domain.IdentitySourceManual {
		return fmt.Errorf("%w: can only delete manual identities", domain.ErrValidation)
	}
	return s.identityStore.Delete(ctx, identityID)
}

func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	return parts[1]
}
