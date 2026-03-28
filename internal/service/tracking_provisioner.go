package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"

	sesadapter "github.com/senda-app/senda/internal/adapter/ses"
	"github.com/senda-app/senda/internal/domain"
)

// adapterReadWriter is the minimal interface needed by TrackingProvisioner.
type adapterReadWriter interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
	Update(ctx context.Context, adapter *domain.Adapter) error
}

// configCrypto is the minimal interface for encrypt/decrypt operations.
type configCrypto interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// ProvisionResult contains the outcome of auto-provisioning tracking resources.
type ProvisionResult struct {
	ConfigSetName   string          `json:"configuration_set_name"`
	TopicARN        string          `json:"topic_arn"`
	SubscriptionARN string          `json:"subscription_arn"`
	WebhookURL      string          `json:"webhook_url"`
	Steps           []ProvisionStep `json:"steps"`
}

// ProvisionStep describes the outcome of a single provisioning step.
type ProvisionStep struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "created", "already_exists", "failed"
	Detail string `json:"detail,omitempty"`
}

// AWSClientFactory creates SES and SNS clients from an AWS config.
// Abstracted for testability.
type AWSClientFactory func(cfg aws.Config, endpointURL string) (sesadapter.SESAPI, sesadapter.SNSAPI)

// DefaultAWSClientFactory creates real AWS SES v2 and SNS clients.
func DefaultAWSClientFactory(cfg aws.Config, endpointURL string) (sesadapter.SESAPI, sesadapter.SNSAPI) {
	return sesadapter.NewSESAPIFromAWSConfig(cfg, endpointURL),
		sns.NewFromConfig(cfg, func(o *sns.Options) {
			if endpointURL != "" {
				o.BaseEndpoint = aws.String(endpointURL)
			}
		})
}

// TrackingProvisioner auto-provisions SES tracking resources (Configuration Set, SNS Topic,
// Event Destination, HTTPS Subscription) using the adapter's own AWS credentials.
type TrackingProvisioner struct {
	adapterStore   adapterReadWriter
	crypto         configCrypto
	webhookBaseURL string
	clientFactory  AWSClientFactory
	logger         *slog.Logger
}

// NewTrackingProvisioner creates a new TrackingProvisioner.
func NewTrackingProvisioner(
	adapterStore adapterReadWriter,
	crypto configCrypto,
	webhookBaseURL string,
	logger *slog.Logger,
) *TrackingProvisioner {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrackingProvisioner{
		adapterStore:   adapterStore,
		crypto:         crypto,
		webhookBaseURL: webhookBaseURL,
		clientFactory:  DefaultAWSClientFactory,
		logger:         logger,
	}
}

// sesAdapterConfig mirrors the encrypted config stored for SES adapters.
type sesAdapterConfig = sesadapter.Config

// Provision auto-provisions all SES tracking resources for the given adapter.
func (p *TrackingProvisioner) Provision(ctx context.Context, adapterID uuid.UUID) (*ProvisionResult, error) {
	// 1. Load adapter and verify type.
	adapter, err := p.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, err
	}
	if adapter.AdapterType != domain.AdapterTypeSES {
		return nil, fmt.Errorf("%w: auto-provisioning is only supported for SES adapters", domain.ErrValidation)
	}

	// 2. Decrypt config.
	decrypted, err := p.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt adapter config: %w", err)
	}
	var adapterCfg sesAdapterConfig
	if err := json.Unmarshal(decrypted, &adapterCfg); err != nil {
		return nil, fmt.Errorf("unmarshal adapter config: %w", err)
	}

	// 3. Build AWS config from adapter credentials.
	awsCfg, err := sesadapter.LoadAWSConfig(ctx, adapterCfg)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	// 4. Create clients.
	sesClient, snsClient := p.clientFactory(awsCfg, adapterCfg.EndpointURL)

	// 5. Derive resource names.
	shortID := adapter.ID.String()[:8]
	configSetName := fmt.Sprintf("senda-%s", shortID)
	topicName := fmt.Sprintf("senda-ses-events-%s", shortID)
	webhookURL := p.webhookBaseURL + "/api/v1/webhooks/ses/inbound"

	result := &ProvisionResult{
		ConfigSetName: configSetName,
		WebhookURL:    webhookURL,
	}

	// Step 1: Create Configuration Set.
	step1 := p.createConfigurationSet(ctx, sesClient, configSetName)
	result.Steps = append(result.Steps, step1)
	if step1.Status == "failed" {
		return result, fmt.Errorf("create configuration set: %s", step1.Detail)
	}

	// Step 2: Create SNS Topic.
	step2, topicARN := p.createSNSTopic(ctx, snsClient, topicName)
	result.Steps = append(result.Steps, step2)
	result.TopicARN = topicARN
	if step2.Status == "failed" {
		return result, fmt.Errorf("create sns topic: %s", step2.Detail)
	}

	// Step 3: Create Event Destination.
	step3 := p.createEventDestination(ctx, sesClient, configSetName, topicARN)
	result.Steps = append(result.Steps, step3)
	if step3.Status == "failed" {
		return result, fmt.Errorf("create event destination: %s", step3.Detail)
	}

	// Step 4: Subscribe webhook endpoint to SNS Topic.
	step4, subscriptionARN := p.subscribeTopic(ctx, snsClient, topicARN, webhookURL)
	result.Steps = append(result.Steps, step4)
	result.SubscriptionARN = subscriptionARN
	if step4.Status == "failed" {
		return result, fmt.Errorf("subscribe topic: %s", step4.Detail)
	}

	// Step 5: Persist configuration_set_name into adapter config.
	step5 := p.saveConfigSetName(ctx, adapter, configSetName)
	result.Steps = append(result.Steps, step5)
	if step5.Status == "failed" {
		return result, fmt.Errorf("save config: %s", step5.Detail)
	}

	p.logger.InfoContext(ctx, "tracking auto-provisioned",
		"adapter_id", adapterID,
		"config_set", configSetName,
		"topic_arn", topicARN,
	)

	return result, nil
}

func (p *TrackingProvisioner) createConfigurationSet(ctx context.Context, client sesadapter.SESAPI, name string) ProvisionStep {
	_, err := client.CreateConfigurationSet(ctx, &sesv2.CreateConfigurationSetInput{
		ConfigurationSetName: aws.String(name),
	})
	if err != nil {
		if isAlreadyExists(err) {
			return ProvisionStep{Name: "create_configuration_set", Status: "already_exists", Detail: name}
		}
		return ProvisionStep{Name: "create_configuration_set", Status: "failed", Detail: err.Error()}
	}
	return ProvisionStep{Name: "create_configuration_set", Status: "created", Detail: name}
}

func (p *TrackingProvisioner) createSNSTopic(ctx context.Context, client sesadapter.SNSAPI, name string) (ProvisionStep, string) {
	output, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(name),
	})
	if err != nil {
		return ProvisionStep{Name: "create_sns_topic", Status: "failed", Detail: err.Error()}, ""
	}
	arn := aws.ToString(output.TopicArn)
	// SNS CreateTopic is idempotent — it returns the existing topic ARN if it already exists.
	return ProvisionStep{Name: "create_sns_topic", Status: "created", Detail: arn}, arn
}

func (p *TrackingProvisioner) createEventDestination(ctx context.Context, client sesadapter.SESAPI, configSetName, topicARN string) ProvisionStep {
	_, err := client.CreateConfigurationSetEventDestination(ctx, &sesv2.CreateConfigurationSetEventDestinationInput{
		ConfigurationSetName: aws.String(configSetName),
		EventDestinationName: aws.String("senda-events"),
		EventDestination: &sestypes.EventDestinationDefinition{
			Enabled: true,
			MatchingEventTypes: []sestypes.EventType{
				sestypes.EventTypeSend,
				sestypes.EventTypeDelivery,
				sestypes.EventTypeBounce,
				sestypes.EventTypeComplaint,
			},
			SnsDestination: &sestypes.SnsDestination{
				TopicArn: aws.String(topicARN),
			},
		},
	})
	if err != nil {
		if isAlreadyExists(err) {
			return ProvisionStep{Name: "create_event_destination", Status: "already_exists"}
		}
		return ProvisionStep{Name: "create_event_destination", Status: "failed", Detail: err.Error()}
	}
	return ProvisionStep{Name: "create_event_destination", Status: "created"}
}

func (p *TrackingProvisioner) subscribeTopic(ctx context.Context, client sesadapter.SNSAPI, topicARN, endpoint string) (ProvisionStep, string) {
	output, err := client.Subscribe(ctx, &sns.SubscribeInput{
		Protocol:              aws.String("https"),
		TopicArn:              aws.String(topicARN),
		Endpoint:              aws.String(endpoint),
		ReturnSubscriptionArn: true,
	})
	if err != nil {
		return ProvisionStep{Name: "subscribe_webhook", Status: "failed", Detail: err.Error()}, ""
	}
	arn := aws.ToString(output.SubscriptionArn)
	return ProvisionStep{Name: "subscribe_webhook", Status: "created", Detail: arn}, arn
}

func (p *TrackingProvisioner) saveConfigSetName(ctx context.Context, adapter *domain.Adapter, configSetName string) ProvisionStep {
	// Decrypt → modify → encrypt → store.
	decrypted, err := p.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return ProvisionStep{Name: "save_configuration", Status: "failed", Detail: err.Error()}
	}
	var cfgMap map[string]any
	if err := json.Unmarshal(decrypted, &cfgMap); err != nil {
		return ProvisionStep{Name: "save_configuration", Status: "failed", Detail: err.Error()}
	}
	cfgMap["configuration_set_name"] = configSetName
	updated, err := json.Marshal(cfgMap)
	if err != nil {
		return ProvisionStep{Name: "save_configuration", Status: "failed", Detail: err.Error()}
	}
	encrypted, err := p.crypto.Encrypt(updated)
	if err != nil {
		return ProvisionStep{Name: "save_configuration", Status: "failed", Detail: err.Error()}
	}
	adapter.ConfigEncrypted = encrypted
	if err := p.adapterStore.Update(ctx, adapter); err != nil {
		return ProvisionStep{Name: "save_configuration", Status: "failed", Detail: err.Error()}
	}
	return ProvisionStep{Name: "save_configuration", Status: "created"}
}

// isAlreadyExists checks if the AWS error indicates the resource already exists.
func isAlreadyExists(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AlreadyExistsException", "ConfigurationSetAlreadyExistsException", "EventDestinationAlreadyExistsException":
			return true
		}
	}
	return false
}

// IsAccessDenied checks if the AWS error is an access denied / insufficient permissions error.
func IsAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDeniedException", "AccessDenied", "UnauthorizedAccess":
			return true
		}
	}
	return false
}
