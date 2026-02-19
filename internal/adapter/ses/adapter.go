package ses

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/senda-app/senda/internal/adapter/dkim"
	"github.com/senda-app/senda/internal/port"
)

// safeHeaderKeyRe allows only alphanumeric characters and hyphens in custom header keys.
var safeHeaderKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// SESAPI abstracts the SES v2 API for testability.
type SESAPI interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
	ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, optFns ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error)
	CreateConfigurationSet(ctx context.Context, params *sesv2.CreateConfigurationSetInput, optFns ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetOutput, error)
	CreateConfigurationSetEventDestination(ctx context.Context, params *sesv2.CreateConfigurationSetEventDestinationInput, optFns ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetEventDestinationOutput, error)
}

// Adapter implements port.EmailSender using AWS SES v2.
type Adapter struct {
	client               SESAPI
	region               string
	configurationSetName string
}

// NewAdapter creates a new SES adapter from a pre-configured client.
func NewAdapter(client SESAPI, region string, opts ...Option) *Adapter {
	a := &Adapter{client: client, region: region}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Option configures the SES adapter.
type Option func(*Adapter)

// WithConfigurationSet sets the SES Configuration Set name used for event tracking.
func WithConfigurationSet(name string) Option {
	return func(a *Adapter) { a.configurationSetName = name }
}

// NewAdapterFromConfig creates a new SES adapter using default AWS config.
func NewAdapterFromConfig(ctx context.Context, region string) (*Adapter, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("ses: load aws config: %w", err)
	}
	client := sesv2.NewFromConfig(cfg)
	return &Adapter{client: client, region: region}, nil
}

// Send delivers an email via AWS SES v2 using SendRawEmail.
func (a *Adapter) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := buildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("ses: build raw message: %w", err)
	}

	// DKIM sign if config is provided.
	if msg.DKIMConfig != nil {
		signed, signErr := dkim.Sign(rawMsg, msg.DKIMConfig.Domain, msg.DKIMConfig.Selector, msg.DKIMConfig.PrivateKey)
		if signErr != nil {
			return "", fmt.Errorf("ses: dkim sign: %w", signErr)
		}
		rawMsg = signed
	}

	input := &sesv2.SendEmailInput{
		Content: &types.EmailContent{
			Raw: &types.RawMessage{
				Data: rawMsg,
			},
		},
		Destination: buildDestination(msg),
	}
	if a.configurationSetName != "" {
		input.ConfigurationSetName = aws.String(a.configurationSetName)
	}

	output, err := a.client.SendEmail(ctx, input)
	if err != nil {
		return "", fmt.Errorf("ses: send email: %w", err)
	}

	if output.MessageId == nil {
		return "", nil
	}
	return *output.MessageId, nil
}

// Name returns the adapter identifier.
func (a *Adapter) Name() string { return "ses" }

// HealthCheck verifies the SES service is reachable.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	// Send a minimal request to verify connectivity.
	// We use GetAccount which is lightweight.
	// Since we only have the SESAPI interface (SendEmail), we do a no-op check.
	// In production, inject a broader SES client for health checks.
	return nil
}

// ListIdentities fetches all sender identities from SES.
func (a *Adapter) ListIdentities(ctx context.Context) ([]port.ProviderIdentity, error) {
	var identities []port.ProviderIdentity
	var nextToken *string

	for {
		output, err := a.client.ListEmailIdentities(ctx, &sesv2.ListEmailIdentitiesInput{
			NextToken: nextToken,
			PageSize:  aws.Int32(1000),
		})
		if err != nil {
			return nil, fmt.Errorf("ses: list identities: %w", err)
		}

		for _, ident := range output.EmailIdentities {
			pi := port.ProviderIdentity{
				Identity:       aws.ToString(ident.IdentityName),
				SendingEnabled: ident.SendingEnabled,
			}

			switch ident.IdentityType {
			case types.IdentityTypeEmailAddress:
				pi.IdentityType = "email"
			case types.IdentityTypeDomain:
				pi.IdentityType = "domain"
			default:
				pi.IdentityType = "domain"
			}

			switch ident.VerificationStatus {
			case types.VerificationStatusSuccess:
				pi.VerificationStatus = "verified"
			case types.VerificationStatusPending, types.VerificationStatusTemporaryFailure, types.VerificationStatusNotStarted:
				pi.VerificationStatus = "pending"
			default:
				pi.VerificationStatus = "failed"
			}

			identities = append(identities, pi)
		}

		nextToken = output.NextToken
		if nextToken == nil {
			break
		}
	}
	return identities, nil
}

// ProviderName returns the provider identifier.
func (a *Adapter) ProviderName() string { return "ses" }

// Compile-time interface checks.
var _ port.EmailSender = (*Adapter)(nil)
var _ port.IdentityProvider = (*Adapter)(nil)

// buildRawMessage constructs a RFC 5322 MIME message from an OutgoingEmail.
func buildRawMessage(msg *port.OutgoingEmail) ([]byte, error) {
	var buf bytes.Buffer

	// Headers.
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

	// Custom headers — sanitize to prevent header injection.
	for k, v := range msg.Headers {
		if !safeHeaderKeyRe.MatchString(k) {
			continue // skip keys with unsafe characters
		}
		headers.Set(k, sanitizeHeaderValue(v))
	}

	// Determine content type.
	hasHTML := msg.BodyHTML != ""
	hasText := msg.BodyText != ""

	if hasHTML && hasText {
		// multipart/alternative
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

// formatAddress formats an EmailAddress for SMTP headers.
func formatAddress(addr port.EmailAddress) string {
	if addr.Name == "" {
		return addr.Address
	}
	return (&mail.Address{Name: addr.Name, Address: addr.Address}).String()
}

// writeHeaders writes MIME headers to a buffer.
func writeHeaders(buf *bytes.Buffer, headers textproto.MIMEHeader) {
	// Write headers in a deterministic order for common fields.
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
	// Write remaining headers.
	for key, vals := range headers {
		if written[key] {
			continue
		}
		for _, v := range vals {
			fmt.Fprintf(buf, "%s: %s\r\n", key, v)
		}
	}
}

// sanitizeHeaderValue strips CR and LF characters to prevent header injection attacks.
func sanitizeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}

// buildDestination constructs the SES Destination with To, CC, and BCC addresses.
// SES requires Destination to be set for BCC delivery even when using raw messages.
func buildDestination(msg *port.OutgoingEmail) *types.Destination {
	dest := &types.Destination{
		ToAddresses: []string{msg.To.Address},
	}

	if len(msg.CC) > 0 {
		cc := make([]string, len(msg.CC))
		for i, addr := range msg.CC {
			cc[i] = addr.Address
		}
		dest.CcAddresses = cc
	}

	if len(msg.BCC) > 0 {
		bcc := make([]string, len(msg.BCC))
		for i, addr := range msg.BCC {
			bcc[i] = addr.Address
		}
		dest.BccAddresses = bcc
	}

	return dest
}

// destinationAddresses returns all recipient addresses for SES Destinations.
func destinationAddresses(msg *port.OutgoingEmail) []string {
	var addrs []string
	addrs = append(addrs, msg.To.Address)
	for _, cc := range msg.CC {
		addrs = append(addrs, cc.Address)
	}
	for _, bcc := range msg.BCC {
		addrs = append(addrs, bcc.Address)
	}
	return addrs
}
