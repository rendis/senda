package resolution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

const domainCacheTTL = 10 * time.Minute

// DomainResolver validates that a from-address domain is verified
// within the resolution chain for a workspace.
type DomainResolver struct {
	store         port.DomainStore
	chainResolver *ChainResolver
	cache         port.Cache
}

// NewDomainResolver creates a DomainResolver with the given dependencies.
func NewDomainResolver(store port.DomainStore, cr *ChainResolver, cache port.Cache) *DomainResolver {
	return &DomainResolver{
		store:         store,
		chainResolver: cr,
		cache:         cache,
	}
}

// ValidateFromAddress checks that the domain of fromEmail is verified
// in the scope chain for the given workspace.
func (r *DomainResolver) ValidateFromAddress(ctx context.Context, workspaceID uuid.UUID, fromEmail string) error {
	emailDomain := extractDomain(fromEmail)
	if emailDomain == "" {
		return fmt.Errorf("%w: domain %s not verified in scope chain", domain.ErrDomainNotVerified, emailDomain)
	}

	// Check cache
	cacheKey := fmt.Sprintf("domain_valid:%s:%s", workspaceID.String(), emailDomain)
	if data, err := r.cache.Get(ctx, cacheKey); err == nil && string(data) == "1" {
		return nil
	}

	// Resolve chain
	chain, err := r.chainResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return err
	}

	// List domains in chain
	domains, err := r.store.ListInChain(ctx, chain.Scopes)
	if err != nil {
		return err
	}

	// Find matching verified domain
	for _, d := range domains {
		if d.DomainName == emailDomain && d.Status == domain.DomainStatusVerified && d.DeletedAt == nil {
			_ = r.cache.Set(ctx, cacheKey, []byte("1"), domainCacheTTL)
			return nil
		}
	}

	return fmt.Errorf("%w: domain %s not verified in scope chain", domain.ErrDomainNotVerified, emailDomain)
}

// extractDomain returns the lowercase domain part of an email address.
// Returns "" if the email has no @ sign.
func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	return strings.ToLower(parts[1])
}
