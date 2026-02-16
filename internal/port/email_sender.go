package port

import "context"

// EmailSender is the port that email provider adapters must implement.
// Each adapter (SES, Gmail, etc.) implements this interface.
type EmailSender interface {
	// Send delivers a single email. Returns the provider's message ID.
	Send(ctx context.Context, msg *OutgoingEmail) (providerMessageID string, err error)

	// Name returns the adapter identifier (e.g., "ses", "gmail").
	Name() string

	// HealthCheck verifies the adapter can reach the provider.
	HealthCheck(ctx context.Context) error
}

// OutgoingEmail contains everything needed to send an email.
type OutgoingEmail struct {
	From       EmailAddress
	To         EmailAddress
	CC         []EmailAddress
	BCC        []EmailAddress
	ReplyTo    *EmailAddress
	Subject    string            // Already rendered (variables resolved)
	BodyHTML   string            // Compiled from MJML (final HTML)
	BodyText   string            // Plain text fallback
	Headers    map[string]string
	DKIMConfig *DKIMConfig       // For signing
	TrackingID string
}

// EmailAddress represents a named email address.
type EmailAddress struct {
	Name    string // Display name ("Acme Support")
	Address string // Email ("support@acme.com")
}

// DKIMConfig holds the DKIM signing configuration for an outgoing email.
type DKIMConfig struct {
	Selector   string
	Domain     string
	PrivateKey []byte // Decrypted
}
