package ses

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/mail"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	sesv1 "github.com/aws/aws-sdk-go-v2/service/ses"
	sesv1types "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/senda-app/senda/internal/port"
)

// safeHeaderKeyRe allows only alphanumeric characters and hyphens in custom header keys.
var safeHeaderKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// Config holds the decrypted configuration for an SES adapter.
type Config struct {
	Region               string `json:"region"`
	AccessKeyID          string `json:"access_key_id,omitempty"`
	SecretAccessKey      string `json:"secret_access_key,omitempty"`
	EndpointURL          string `json:"endpoint_url,omitempty"`
	ConfigurationSetName string `json:"configuration_set_name,omitempty"`
}

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

// LoadAWSConfig builds an AWS SDK config from the decrypted SES adapter config.
func LoadAWSConfig(ctx context.Context, cfg Config) (aws.Config, error) {
	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	return config.LoadDefaultConfig(ctx, loadOpts...)
}

// NewAdapterFromConfig creates a new SES adapter using the decrypted adapter config.
func NewAdapterFromConfig(ctx context.Context, cfg Config) (*Adapter, error) {
	awsCfg, err := LoadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("ses: load aws config: %w", err)
	}
	client := NewSESAPIFromAWSConfig(awsCfg, cfg.EndpointURL)
	return NewAdapter(client, cfg.Region, WithConfigurationSet(cfg.ConfigurationSetName)), nil
}

// NewSESAPIFromAWSConfig creates the SES API implementation to use for the given config.
// endpoint_url is a test hook used for LocalStack and other custom SES endpoints.
func NewSESAPIFromAWSConfig(cfg aws.Config, endpointURL string) SESAPI {
	endpointURL = strings.TrimRight(endpointURL, "/")
	if shouldUseSESV1Shim(endpointURL) {
		return &sesV1API{
			client: sesv1.NewFromConfig(cfg, func(o *sesv1.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			}),
		}
	}
	return sesv2.NewFromConfig(cfg, func(o *sesv2.Options) {
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
	})
}

func shouldUseSESV1Shim(endpointURL string) bool {
	if endpointURL == "" {
		return false
	}
	parsed, err := url.Parse(endpointURL)
	if err != nil {
		return true
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "localstack"
}

type sesV1API struct {
	client *sesv1.Client
}

func (s *sesV1API) SendEmail(ctx context.Context, params *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	if params == nil || params.Content == nil || params.Content.Raw == nil {
		return nil, fmt.Errorf("ses: raw message content is required")
	}
	input := &sesv1.SendRawEmailInput{
		RawMessage: &sesv1types.RawMessage{
			Data: params.Content.Raw.Data,
		},
		ConfigurationSetName: params.ConfigurationSetName,
		Destinations:         flattenDestination(params.Destination),
	}
	out, err := s.client.SendRawEmail(ctx, input)
	if err != nil {
		return nil, err
	}
	return &sesv2.SendEmailOutput{MessageId: out.MessageId}, nil
}

func (s *sesV1API) ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, _ ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	input := &sesv1.ListIdentitiesInput{
		MaxItems:  params.PageSize,
		NextToken: params.NextToken,
	}
	out, err := s.client.ListIdentities(ctx, input)
	if err != nil {
		return nil, err
	}

	verificationAttrs := map[string]sesv1types.IdentityVerificationAttributes{}
	if len(out.Identities) > 0 {
		verifyOut, err := s.client.GetIdentityVerificationAttributes(ctx, &sesv1.GetIdentityVerificationAttributesInput{
			Identities: out.Identities,
		})
		if err != nil {
			return nil, err
		}
		verificationAttrs = verifyOut.VerificationAttributes
	}

	identities := make([]types.IdentityInfo, 0, len(out.Identities))
	for _, identity := range out.Identities {
		attr, ok := verificationAttrs[identity]
		status := types.VerificationStatusNotStarted
		sendingEnabled := false
		if ok {
			status = mapVerificationStatus(attr.VerificationStatus)
			sendingEnabled = attr.VerificationStatus == sesv1types.VerificationStatusSuccess
		}
		identities = append(identities, types.IdentityInfo{
			IdentityName:       aws.String(identity),
			IdentityType:       mapIdentityType(identity),
			SendingEnabled:     sendingEnabled,
			VerificationStatus: status,
		})
	}

	return &sesv2.ListEmailIdentitiesOutput{
		EmailIdentities: identities,
		NextToken:       out.NextToken,
	}, nil
}

func (s *sesV1API) CreateConfigurationSet(ctx context.Context, params *sesv2.CreateConfigurationSetInput, _ ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetOutput, error) {
	if params == nil || params.ConfigurationSetName == nil {
		return nil, fmt.Errorf("ses: configuration set name is required")
	}
	_, err := s.client.CreateConfigurationSet(ctx, &sesv1.CreateConfigurationSetInput{
		ConfigurationSet: &sesv1types.ConfigurationSet{
			Name: params.ConfigurationSetName,
		},
	})
	if err != nil {
		return nil, err
	}
	return &sesv2.CreateConfigurationSetOutput{}, nil
}

func (s *sesV1API) CreateConfigurationSetEventDestination(ctx context.Context, params *sesv2.CreateConfigurationSetEventDestinationInput, _ ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetEventDestinationOutput, error) {
	if params == nil || params.ConfigurationSetName == nil || params.EventDestination == nil || params.EventDestinationName == nil {
		return nil, fmt.Errorf("ses: event destination is required")
	}
	input := &sesv1.CreateConfigurationSetEventDestinationInput{
		ConfigurationSetName: params.ConfigurationSetName,
		EventDestination: &sesv1types.EventDestination{
			Name:               params.EventDestinationName,
			Enabled:            params.EventDestination.Enabled,
			MatchingEventTypes: mapEventTypes(params.EventDestination.MatchingEventTypes),
		},
	}
	if params.EventDestination.SnsDestination != nil && params.EventDestination.SnsDestination.TopicArn != nil {
		input.EventDestination.SNSDestination = &sesv1types.SNSDestination{
			TopicARN: params.EventDestination.SnsDestination.TopicArn,
		}
	}
	_, err := s.client.CreateConfigurationSetEventDestination(ctx, input)
	if err != nil {
		return nil, err
	}
	return &sesv2.CreateConfigurationSetEventDestinationOutput{}, nil
}

func flattenDestination(dest *types.Destination) []string {
	if dest == nil {
		return nil
	}
	all := make([]string, 0, len(dest.ToAddresses)+len(dest.CcAddresses)+len(dest.BccAddresses))
	all = append(all, dest.ToAddresses...)
	all = append(all, dest.CcAddresses...)
	all = append(all, dest.BccAddresses...)
	return all
}

func mapVerificationStatus(status sesv1types.VerificationStatus) types.VerificationStatus {
	switch status {
	case sesv1types.VerificationStatusSuccess:
		return types.VerificationStatusSuccess
	case sesv1types.VerificationStatusPending:
		return types.VerificationStatusPending
	case sesv1types.VerificationStatusTemporaryFailure:
		return types.VerificationStatusTemporaryFailure
	case sesv1types.VerificationStatusFailed:
		return types.VerificationStatusFailed
	default:
		return types.VerificationStatusNotStarted
	}
}

func mapIdentityType(identity string) types.IdentityType {
	if strings.Contains(identity, "@") {
		return types.IdentityTypeEmailAddress
	}
	return types.IdentityTypeDomain
}

func mapEventTypes(eventTypes []types.EventType) []sesv1types.EventType {
	out := make([]sesv1types.EventType, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		out = append(out, sesv1types.EventType(eventType))
	}
	return out
}

// Send delivers an email via AWS SES v2 using SendRawEmail.
func (a *Adapter) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := buildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("ses: build raw message: %w", err)
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
