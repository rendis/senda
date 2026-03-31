package port

import (
	"context"

	"github.com/rendis/senda/internal/domain"
)

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
	TrackingID string
}

// SenderFactory creates an EmailSender from a resolved adapter and its decrypted config.
type SenderFactory func(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (EmailSender, error)

// EmailAddress represents a named email address.
type EmailAddress struct {
	Name    string // Display name ("Acme Support")
	Address string // Email ("support@acme.com")
}
