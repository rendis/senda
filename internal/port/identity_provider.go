package port

import "context"

// ProviderIdentity represents a sender identity as reported by the email provider.
type ProviderIdentity struct {
	Identity           string // email address or domain
	IdentityType       string // "email" or "domain"
	VerificationStatus string // "verified", "pending", "failed"
	SendingEnabled     bool
}

// IdentityProvider abstracts provider-specific identity listing.
// Implemented by SES adapter, Gmail adapter. NOT by SMTP (manual only).
type IdentityProvider interface {
	ListIdentities(ctx context.Context) ([]ProviderIdentity, error)
	ProviderName() string
}
