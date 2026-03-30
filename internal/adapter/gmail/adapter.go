package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/oauth2/google"
	gm "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	sendamime "github.com/rendis/senda/internal/mime"
	"github.com/rendis/senda/internal/port"
)

// GmailConfig holds the decrypted configuration for a Gmail adapter.
// Uses a Google Cloud Service Account with domain-wide delegation.
type GmailConfig struct {
	ServiceAccountJSON string `json:"service_account_json"`
	DelegateEmail      string `json:"delegate_email"`
}

// Validate checks that required fields are present.
func (c GmailConfig) Validate() error {
	if c.ServiceAccountJSON == "" || c.DelegateEmail == "" {
		return fmt.Errorf("missing service_account_json or delegate_email")
	}
	return nil
}

// GmailAPI abstracts the Gmail API for testability.
type GmailAPI interface {
	SendMessage(ctx context.Context, msg *gm.Message) (*gm.Message, error)
	ListSendAs(ctx context.Context) ([]*gm.SendAs, error)
}

// liveGmailAPI implements GmailAPI using the real Gmail service.
type liveGmailAPI struct {
	svc *gm.Service
}

func (g *liveGmailAPI) SendMessage(ctx context.Context, msg *gm.Message) (*gm.Message, error) {
	return g.svc.Users.Messages.Send("me", msg).Context(ctx).Do()
}

func (g *liveGmailAPI) ListSendAs(ctx context.Context) ([]*gm.SendAs, error) {
	resp, err := g.svc.Users.Settings.SendAs.List("me").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return resp.SendAs, nil
}

// Adapter implements port.EmailSender and port.IdentityProvider using Gmail API.
type Adapter struct {
	client GmailAPI
}

// NewAdapter creates a Gmail adapter from a pre-configured client.
func NewAdapter(client GmailAPI) *Adapter {
	return &Adapter{client: client}
}

// NewAdapterFromConfig creates a Gmail adapter using a Service Account with domain-wide delegation.
func NewAdapterFromConfig(ctx context.Context, cfg GmailConfig) (*Adapter, error) {
	jwtCfg, err := google.JWTConfigFromJSON(
		[]byte(cfg.ServiceAccountJSON),
		gm.GmailSendScope,
		gm.GmailSettingsBasicScope,
	)
	if err != nil {
		return nil, fmt.Errorf("gmail: parse service account key: %w", err)
	}
	jwtCfg.Subject = cfg.DelegateEmail

	svc, err := gm.NewService(ctx, option.WithHTTPClient(jwtCfg.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("gmail: create service: %w", err)
	}

	return &Adapter{client: &liveGmailAPI{svc: svc}}, nil
}

// IsPermanentSendError classifies a Gmail send error as permanent or transient.
// 4xx errors (except 429 Too Many Requests) are permanent; 5xx and 429 are transient.
func (a *Adapter) IsPermanentSendError(err error) bool {
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code >= 400 && gErr.Code < 500 && gErr.Code != 429
	}
	return false
}

// Send delivers an email via Gmail API.
func (a *Adapter) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("gmail: build message: %w", err)
	}

	encoded := base64.URLEncoding.EncodeToString(rawMsg)
	gmMsg := &gm.Message{Raw: encoded}

	sent, err := a.client.SendMessage(ctx, gmMsg)
	if err != nil {
		return "", fmt.Errorf("gmail: send: %w", err)
	}

	return sent.Id, nil
}

// Name returns the adapter identifier.
func (a *Adapter) Name() string { return "gmail" }

// HealthCheck verifies the Gmail API is reachable.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	_, err := a.client.ListSendAs(ctx)
	if err != nil {
		return fmt.Errorf("gmail: health check: %w", err)
	}
	return nil
}

// ListIdentities fetches all send-as aliases from Gmail.
func (a *Adapter) ListIdentities(ctx context.Context) ([]port.ProviderIdentity, error) {
	aliases, err := a.client.ListSendAs(ctx)
	if err != nil {
		return nil, fmt.Errorf("gmail: list identities: %w", err)
	}

	identities := make([]port.ProviderIdentity, 0, len(aliases))
	for _, alias := range aliases {
		pi := port.ProviderIdentity{
			Identity:     alias.SendAsEmail,
			IdentityType: "email",
		}

		switch alias.VerificationStatus {
		case "accepted":
			pi.VerificationStatus = "verified"
			pi.SendingEnabled = true
		case "pending":
			pi.VerificationStatus = "pending"
			pi.SendingEnabled = false
		default:
			pi.VerificationStatus = "failed"
			pi.SendingEnabled = false
		}

		identities = append(identities, pi)
	}
	return identities, nil
}

// ProviderName returns the provider identifier.
func (a *Adapter) ProviderName() string { return "gmail" }

// Compile-time interface checks.
var _ port.EmailSender = (*Adapter)(nil)
var _ port.IdentityProvider = (*Adapter)(nil)

