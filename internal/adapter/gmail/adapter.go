package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gm "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/senda-app/senda/internal/adapter/dkim"
	"github.com/senda-app/senda/internal/port"
)

var safeHeaderKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// GmailConfig holds the decrypted configuration for a Gmail adapter.
type GmailConfig struct {
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	RefreshToken      string `json:"refresh_token"`
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

// NewAdapterFromConfig creates a Gmail adapter using OAuth2 credentials.
func NewAdapterFromConfig(ctx context.Context, cfg GmailConfig) (*Adapter, error) {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.OAuthClientID,
		ClientSecret: cfg.OAuthClientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			gm.GmailSendScope,
			gm.GmailSettingsBasicScope,
		},
	}

	token := &oauth2.Token{RefreshToken: cfg.RefreshToken}
	httpClient := oauthCfg.Client(ctx, token)

	svc, err := gm.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("gmail: create service: %w", err)
	}

	return &Adapter{client: &liveGmailAPI{svc: svc}}, nil
}

// Send delivers an email via Gmail API.
func (a *Adapter) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := buildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("gmail: build message: %w", err)
	}

	if msg.DKIMConfig != nil {
		signed, signErr := dkim.Sign(rawMsg, msg.DKIMConfig.Domain, msg.DKIMConfig.Selector, msg.DKIMConfig.PrivateKey)
		if signErr != nil {
			return "", fmt.Errorf("gmail: dkim sign: %w", signErr)
		}
		rawMsg = signed
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

func buildRawMessage(msg *port.OutgoingEmail) ([]byte, error) {
	var buf bytes.Buffer

	headers := textproto.MIMEHeader{}
	headers.Set("From", formatAddress(msg.From))
	headers.Set("To", formatAddress(msg.To))
	headers.Set("Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	headers.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	headers.Set("MIME-Version", "1.0")

	if len(msg.CC) > 0 {
		addrs := make([]string, len(msg.CC))
		for i, cc := range msg.CC {
			addrs[i] = formatAddress(cc)
		}
		headers.Set("Cc", strings.Join(addrs, ", "))
	}
	if msg.ReplyTo != nil {
		headers.Set("Reply-To", formatAddress(*msg.ReplyTo))
	}

	for k, v := range msg.Headers {
		if !safeHeaderKeyRe.MatchString(k) {
			continue
		}
		headers.Set(k, sanitizeHeaderValue(v))
	}

	hasHTML := msg.BodyHTML != ""
	hasText := msg.BodyText != ""

	if hasHTML && hasText {
		boundary := "senda-boundary-" + msg.TrackingID
		headers.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
		writeHeaders(&buf, headers)

		fmt.Fprintf(&buf, "\r\n--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.BodyText)

		fmt.Fprintf(&buf, "\r\n--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.BodyHTML)

		fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)
	} else if hasHTML {
		headers.Set("Content-Type", "text/html; charset=UTF-8")
		writeHeaders(&buf, headers)
		buf.WriteString("\r\n")
		buf.WriteString(msg.BodyHTML)
	} else {
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		writeHeaders(&buf, headers)
		buf.WriteString("\r\n")
		buf.WriteString(msg.BodyText)
	}

	return buf.Bytes(), nil
}

func formatAddress(addr port.EmailAddress) string {
	if addr.Name == "" {
		return addr.Address
	}
	return (&mail.Address{Name: addr.Name, Address: addr.Address}).String()
}

func writeHeaders(buf *bytes.Buffer, headers textproto.MIMEHeader) {
	order := []string{"From", "To", "Cc", "Reply-To", "Subject", "Date", "Mime-Version", "Content-Type"}
	written := make(map[string]bool)
	for _, key := range order {
		if vals, ok := headers[key]; ok {
			for _, v := range vals {
				fmt.Fprintf(buf, "%s: %s\r\n", key, v)
			}
			written[key] = true
		}
	}
	for key, vals := range headers {
		if written[key] {
			continue
		}
		for _, v := range vals {
			fmt.Fprintf(buf, "%s: %s\r\n", key, v)
		}
	}
}

func sanitizeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}
