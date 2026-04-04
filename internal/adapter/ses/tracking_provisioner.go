package ses

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

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
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

// Step response status constants (distinct from domain.ProvisioningStepStatus which tracks DB state).
const (
	StepStatusCreated              = "created"
	StepStatusAlreadyExists        = "already_exists"
	StepStatusAlreadyCompleted     = "already_completed"
	StepStatusFailed               = "failed"
	StepStatusPendingConfirmation  = "pending_confirmation"
)

// ProvisionStep describes the outcome of a single provisioning step.
type ProvisionStep struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"` // StepStatus* constants
	Detail       string  `json:"detail,omitempty"`
	ResourceName *string `json:"resource_name,omitempty"`
	ResourceARN  *string `json:"resource_arn,omitempty"`
}

// AWSClientFactory creates SES and SNS clients from an AWS config.
// Abstracted for testability.
type AWSClientFactory func(cfg aws.Config, endpointURL string) (SESAPI, SNSAPI)

// DefaultAWSClientFactory creates real AWS SES v2 and SNS clients.
func DefaultAWSClientFactory(cfg aws.Config, endpointURL string) (SESAPI, SNSAPI) {
	return NewSESAPIFromAWSConfig(cfg, endpointURL),
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
	stepStore      port.ProvisioningStepStore // nil = stateless fallback
	logger         *slog.Logger

}

// NewTrackingProvisioner creates a new TrackingProvisioner.
// stepStore is optional — pass nil for stateless (legacy) behavior.
func NewTrackingProvisioner(
	adapterStore adapterReadWriter,
	crypto configCrypto,
	webhookBaseURL string,
	logger *slog.Logger,
	stepStore port.ProvisioningStepStore,
) *TrackingProvisioner {
	if logger == nil {
		logger = slog.Default()
	}
	return &TrackingProvisioner{
		adapterStore:   adapterStore,
		crypto:         crypto,
		webhookBaseURL: webhookBaseURL,
		clientFactory:  DefaultAWSClientFactory,
		stepStore:      stepStore,
		logger:         logger,
	}
}

// loadAdapterClients loads an SES adapter, decrypts its config, and builds AWS clients.
func (p *TrackingProvisioner) loadAdapterClients(ctx context.Context, adapterID uuid.UUID) (*domain.Adapter, Config, SESAPI, SNSAPI, error) {
	adapter, err := p.adapterStore.GetByID(ctx, adapterID)
	if err != nil {
		return nil, Config{}, nil, nil, err
	}
	if adapter.AdapterType != domain.AdapterTypeSES {
		return nil, Config{}, nil, nil, fmt.Errorf("%w: operation only supported for SES adapters", domain.ErrValidation)
	}
	decrypted, err := p.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return nil, Config{}, nil, nil, fmt.Errorf("decrypt adapter config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(decrypted, &cfg); err != nil {
		return nil, Config{}, nil, nil, fmt.Errorf("unmarshal adapter config: %w", err)
	}
	awsCfg, err := LoadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, Config{}, nil, nil, fmt.Errorf("load aws config: %w", err)
	}
	sesClient, snsClient := p.clientFactory(awsCfg, cfg.EndpointURL)
	return adapter, cfg, sesClient, snsClient, nil
}

// Provision auto-provisions all SES tracking resources for the given adapter.
// When a stepStore is configured, completed steps are skipped on retry.
func (p *TrackingProvisioner) Provision(ctx context.Context, adapterID uuid.UUID) (*ProvisionResult, error) {
	adapter, _, sesClient, snsClient, err := p.loadAdapterClients(ctx, adapterID)
	if err != nil {
		return nil, err
	}

	// 5. Derive resource names.
	shortID := adapter.ID.String()[:8]
	configSetName := fmt.Sprintf("senda-%s", shortID)
	topicName := fmt.Sprintf("senda-ses-events-%s", shortID)
	webhookURL := p.webhookBaseURL + "/api/v1/webhooks/ses/inbound"

	// 6. Load persisted step state (if store is configured).
	stepMap := map[string]*domain.AdapterProvisioningStep{}
	if p.stepStore != nil {
		if err := p.stepStore.InitSteps(ctx, adapterID); err != nil {
			return nil, fmt.Errorf("init provisioning steps: %w", err)
		}
		if err := p.stepStore.ResetFailed(ctx, adapterID); err != nil {
			return nil, fmt.Errorf("reset failed steps: %w", err)
		}
		steps, err := p.stepStore.ListByAdapter(ctx, adapterID)
		if err != nil {
			return nil, fmt.Errorf("load provisioning steps: %w", err)
		}
		for _, s := range steps {
			stepMap[s.StepName] = s
		}
	}

	result := &ProvisionResult{
		ConfigSetName: configSetName,
		WebhookURL:    webhookURL,
	}

	// Step 1: Create Configuration Set.
	if ps, ok := stepMap[domain.StepCreateConfigurationSet]; ok && ps.Status == domain.ProvisionStepCompleted {
		result.Steps = append(result.Steps, ProvisionStep{
			Name: domain.StepCreateConfigurationSet, Status: StepStatusAlreadyCompleted,
			Detail: configSetName, ResourceName: ps.ResourceName, ResourceARN: ps.ResourceARN,
		})
	} else {
		step1 := p.createConfigurationSet(ctx, sesClient, configSetName)
		result.Steps = append(result.Steps, step1)
		if step1.Status == StepStatusFailed {
			p.persistStepFailure(ctx, stepMap, domain.StepCreateConfigurationSet, step1.Detail)
			return result, fmt.Errorf("create configuration set: %s", step1.Detail)
		}
		p.persistStepSuccess(ctx, stepMap, domain.StepCreateConfigurationSet, &configSetName, nil)
	}

	// Step 2: Create SNS Topic.
	var topicARN string
	if ps, ok := stepMap[domain.StepCreateSNSTopic]; ok && ps.Status == domain.ProvisionStepCompleted {
		if ps.ResourceARN != nil {
			topicARN = *ps.ResourceARN
		}
		result.TopicARN = topicARN
		result.Steps = append(result.Steps, ProvisionStep{
			Name: domain.StepCreateSNSTopic, Status: StepStatusAlreadyCompleted,
			Detail: topicARN, ResourceName: ps.ResourceName, ResourceARN: ps.ResourceARN,
		})
	} else {
		step2, arn := p.createSNSTopic(ctx, snsClient, topicName)
		topicARN = arn
		result.Steps = append(result.Steps, step2)
		result.TopicARN = topicARN
		if step2.Status == StepStatusFailed {
			p.persistStepFailure(ctx, stepMap, domain.StepCreateSNSTopic, step2.Detail)
			return result, fmt.Errorf("create sns topic: %s", step2.Detail)
		}
		p.persistStepSuccess(ctx, stepMap, domain.StepCreateSNSTopic, &topicName, &topicARN)
	}

	// Step 3: Create Event Destination.
	if ps, ok := stepMap[domain.StepCreateEventDestination]; ok && ps.Status == domain.ProvisionStepCompleted {
		result.Steps = append(result.Steps, ProvisionStep{
			Name: domain.StepCreateEventDestination, Status: StepStatusAlreadyCompleted,
			ResourceName: ps.ResourceName, ResourceARN: ps.ResourceARN,
		})
	} else {
		step3 := p.createEventDestination(ctx, sesClient, configSetName, topicARN)
		result.Steps = append(result.Steps, step3)
		if step3.Status == StepStatusFailed {
			p.persistStepFailure(ctx, stepMap, domain.StepCreateEventDestination, step3.Detail)
			return result, fmt.Errorf("create event destination: %s", step3.Detail)
		}
		destName := "senda-events"
		p.persistStepSuccess(ctx, stepMap, domain.StepCreateEventDestination, &destName, nil)
	}

	// Step 4: Subscribe webhook endpoint to SNS Topic.
	var subscriptionARN string
	if ps, ok := stepMap[domain.StepSubscribeWebhook]; ok && ps.Status == domain.ProvisionStepCompleted {
		if ps.ResourceARN != nil {
			subscriptionARN = *ps.ResourceARN
		}
		result.SubscriptionARN = subscriptionARN
		result.Steps = append(result.Steps, ProvisionStep{
			Name: domain.StepSubscribeWebhook, Status: StepStatusAlreadyCompleted,
			Detail: subscriptionARN, ResourceName: ps.ResourceName, ResourceARN: ps.ResourceARN,
		})
	} else {
		step4, arn := p.subscribeTopic(ctx, snsClient, topicARN, webhookURL)
		subscriptionARN = arn
		result.Steps = append(result.Steps, step4)
		result.SubscriptionARN = subscriptionARN
		if step4.Status == StepStatusFailed {
			p.persistStepFailure(ctx, stepMap, domain.StepSubscribeWebhook, step4.Detail)
			return result, fmt.Errorf("subscribe topic: %s", step4.Detail)
		}
		p.persistStepSuccess(ctx, stepMap, domain.StepSubscribeWebhook, &webhookURL, &subscriptionARN)
	}

	// Step 5: Persist configuration_set_name into adapter config.
	if ps, ok := stepMap[domain.StepSaveConfiguration]; ok && ps.Status == domain.ProvisionStepCompleted {
		result.Steps = append(result.Steps, ProvisionStep{
			Name: domain.StepSaveConfiguration, Status: StepStatusAlreadyCompleted,
			ResourceName: ps.ResourceName,
		})
	} else {
		step5 := p.saveConfigSetName(ctx, adapter, configSetName)
		result.Steps = append(result.Steps, step5)
		if step5.Status == StepStatusFailed {
			p.persistStepFailure(ctx, stepMap, domain.StepSaveConfiguration, step5.Detail)
			return result, fmt.Errorf("save config: %s", step5.Detail)
		}
		p.persistStepSuccess(ctx, stepMap, domain.StepSaveConfiguration, &configSetName, nil)
	}

	// Step 6: Verify SNS subscription confirmed.
	if ps, ok := stepMap[domain.StepVerifySubscription]; ok && ps.Status == domain.ProvisionStepCompleted {
		result.Steps = append(result.Steps, ProvisionStep{
			Name: domain.StepVerifySubscription, Status: StepStatusAlreadyCompleted,
			Detail: subscriptionARN, ResourceARN: ps.ResourceARN,
		})
	} else {
		step6 := p.verifySubscription(ctx, snsClient, topicARN, webhookURL)
		result.Steps = append(result.Steps, step6)
		switch step6.Status {
		case StepStatusCreated, StepStatusPendingConfirmation:
			// Pending confirmation is a transient state — the webhook is live
			// and will auto-confirm. Persist as completed either way.
			p.persistStepSuccess(ctx, stepMap, domain.StepVerifySubscription, nil, &subscriptionARN)
			if step6.Status == StepStatusPendingConfirmation {
				p.logger.InfoContext(ctx, "subscription pending confirmation — will be auto-confirmed by webhook",
					"adapter_id", adapterID)
			}
		default: // StepStatusFailed
			p.persistStepFailure(ctx, stepMap, domain.StepVerifySubscription, step6.Detail)
			p.logger.WarnContext(ctx, "subscription verification failed (non-blocking)",
				"adapter_id", adapterID, "detail", step6.Detail)
		}
	}

	p.logger.InfoContext(ctx, "tracking auto-provisioned",
		"adapter_id", adapterID,
		"config_set", configSetName,
		"topic_arn", topicARN,
	)

	return result, nil
}

// persistStepSuccess marks a step as completed in the store (if configured).
func (p *TrackingProvisioner) persistStepSuccess(ctx context.Context, stepMap map[string]*domain.AdapterProvisioningStep, stepName string, resourceName, resourceARN *string) {
	if p.stepStore == nil {
		return
	}
	ps, ok := stepMap[stepName]
	if !ok {
		return
	}
	if err := p.stepStore.MarkCompleted(ctx, ps.ID, resourceName, resourceARN); err != nil {
		p.logger.WarnContext(ctx, "failed to persist step success", "step", stepName, "error", err)
	}
}

// persistStepFailure marks a step as failed in the store (if configured).
func (p *TrackingProvisioner) persistStepFailure(ctx context.Context, stepMap map[string]*domain.AdapterProvisioningStep, stepName, errMsg string) {
	if p.stepStore == nil {
		return
	}
	ps, ok := stepMap[stepName]
	if !ok {
		return
	}
	if err := p.stepStore.MarkFailed(ctx, ps.ID, errMsg); err != nil {
		p.logger.WarnContext(ctx, "failed to persist step failure", "step", stepName, "error", err)
	}
}

func (p *TrackingProvisioner) createConfigurationSet(ctx context.Context, client SESAPI, name string) ProvisionStep {
	_, err := client.CreateConfigurationSet(ctx, &sesv2.CreateConfigurationSetInput{
		ConfigurationSetName: aws.String(name),
	})
	if err != nil {
		if isAlreadyExists(err) {
			return ProvisionStep{Name: domain.StepCreateConfigurationSet, Status: StepStatusAlreadyExists, Detail: name}
		}
		return ProvisionStep{Name: domain.StepCreateConfigurationSet, Status: StepStatusFailed, Detail: err.Error()}
	}
	return ProvisionStep{Name: domain.StepCreateConfigurationSet, Status: StepStatusCreated, Detail: name}
}

func (p *TrackingProvisioner) createSNSTopic(ctx context.Context, client SNSAPI, name string) (ProvisionStep, string) {
	output, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(name),
	})
	if err != nil {
		return ProvisionStep{Name: domain.StepCreateSNSTopic, Status: StepStatusFailed, Detail: err.Error()}, ""
	}
	arn := aws.ToString(output.TopicArn)
	// SNS CreateTopic is idempotent — it returns the existing topic ARN if it already exists.
	return ProvisionStep{Name: domain.StepCreateSNSTopic, Status: StepStatusCreated, Detail: arn}, arn
}

func (p *TrackingProvisioner) createEventDestination(ctx context.Context, client SESAPI, configSetName, topicARN string) ProvisionStep {
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
			return ProvisionStep{Name: domain.StepCreateEventDestination, Status: StepStatusAlreadyExists}
		}
		return ProvisionStep{Name: domain.StepCreateEventDestination, Status: StepStatusFailed, Detail: err.Error()}
	}
	return ProvisionStep{Name: domain.StepCreateEventDestination, Status: StepStatusCreated}
}

func (p *TrackingProvisioner) subscribeTopic(ctx context.Context, client SNSAPI, topicARN, endpoint string) (ProvisionStep, string) {
	output, err := client.Subscribe(ctx, &sns.SubscribeInput{
		Protocol:              aws.String("https"),
		TopicArn:              aws.String(topicARN),
		Endpoint:              aws.String(endpoint),
		ReturnSubscriptionArn: true,
	})
	if err != nil {
		return ProvisionStep{Name: domain.StepSubscribeWebhook, Status: StepStatusFailed, Detail: err.Error()}, ""
	}
	arn := aws.ToString(output.SubscriptionArn)
	return ProvisionStep{Name: domain.StepSubscribeWebhook, Status: StepStatusCreated, Detail: arn}, arn
}

func (p *TrackingProvisioner) saveConfigSetName(ctx context.Context, adapter *domain.Adapter, configSetName string) ProvisionStep {
	// Decrypt -> modify -> encrypt -> store.
	decrypted, err := p.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return ProvisionStep{Name: domain.StepSaveConfiguration, Status: StepStatusFailed, Detail: err.Error()}
	}
	var cfgMap map[string]any
	if err := json.Unmarshal(decrypted, &cfgMap); err != nil {
		return ProvisionStep{Name: domain.StepSaveConfiguration, Status: StepStatusFailed, Detail: err.Error()}
	}
	cfgMap["configuration_set_name"] = configSetName
	updated, err := json.Marshal(cfgMap)
	if err != nil {
		return ProvisionStep{Name: domain.StepSaveConfiguration, Status: StepStatusFailed, Detail: err.Error()}
	}
	encrypted, err := p.crypto.Encrypt(updated)
	if err != nil {
		return ProvisionStep{Name: domain.StepSaveConfiguration, Status: StepStatusFailed, Detail: err.Error()}
	}
	adapter.ConfigEncrypted = encrypted
	if err := p.adapterStore.Update(ctx, adapter); err != nil {
		return ProvisionStep{Name: domain.StepSaveConfiguration, Status: StepStatusFailed, Detail: err.Error()}
	}
	return ProvisionStep{Name: domain.StepSaveConfiguration, Status: StepStatusCreated}
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
		case "AccessDeniedException", "AccessDenied", "UnauthorizedAccess", "AuthorizationError":
			return true
		}
	}
	return false
}

// isNotFound checks if the AWS error indicates the resource does not exist.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFoundException", "ResourceNotFoundException", "InvalidParameterException":
			return true
		}
	}
	return false
}

// verifySubscription performs a single check to see if the SNS subscription is confirmed.
// No polling loop — the webhook endpoint is live and will auto-confirm the subscription
// when SNS delivers the SubscriptionConfirmation message. If not yet confirmed, returns
// StepStatusPendingConfirmation so the caller and frontend can handle it gracefully.
func (p *TrackingProvisioner) verifySubscription(ctx context.Context, client SNSAPI, topicARN, endpoint string) ProvisionStep {
	out, err := client.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
		TopicArn: aws.String(topicARN),
	})
	if err != nil {
		// If the adapter credentials lack sns:ListSubscriptionsByTopic permission,
		// we can't verify — but the subscription itself still works (subscribe +
		// webhook confirmation don't require this permission). Treat as pending.
		if IsAccessDenied(err) {
			return ProvisionStep{
				Name:   domain.StepVerifySubscription,
				Status: StepStatusPendingConfirmation,
				Detail: "cannot verify (insufficient permissions) — subscription will be auto-confirmed by webhook",
			}
		}
		return ProvisionStep{
			Name:   domain.StepVerifySubscription,
			Status: StepStatusFailed,
			Detail: fmt.Sprintf("list subscriptions: %s", err),
		}
	}

	for _, sub := range out.Subscriptions {
		if aws.ToString(sub.Endpoint) != endpoint {
			continue
		}
		arn := aws.ToString(sub.SubscriptionArn)
		if arn != "" && arn != "PendingConfirmation" {
			return ProvisionStep{Name: domain.StepVerifySubscription, Status: StepStatusCreated, Detail: arn}
		}
		// Subscription exists but awaiting confirmation — normal transient state.
		// The webhook endpoint will auto-confirm it when SNS delivers the request.
		return ProvisionStep{
			Name:   domain.StepVerifySubscription,
			Status: StepStatusPendingConfirmation,
			Detail: "awaiting SNS confirmation — will be auto-confirmed by webhook",
		}
	}

	return ProvisionStep{
		Name:   domain.StepVerifySubscription,
		Status: StepStatusFailed,
		Detail: "subscription not found in topic",
	}
}

// Deprovision cleans up AWS resources (Configuration Set, SNS Topic, Event Destination,
// SNS Subscription) for the given SES adapter. Each step is idempotent — NotFoundException
// is treated as success.
func (p *TrackingProvisioner) Deprovision(ctx context.Context, adapterID uuid.UUID) error {
	adapter, _, sesClient, snsClient, err := p.loadAdapterClients(ctx, adapterID)
	if err != nil {
		return err
	}

	shortID := adapter.ID.String()[:8]
	configSetName := fmt.Sprintf("senda-%s", shortID)
	topicARN := ""
	subscriptionARN := ""

	// Init deprovision steps + load ALL steps (provision + deprovision) in one query.
	stepMap := map[string]*domain.AdapterProvisioningStep{}
	if p.stepStore != nil {
		if err := p.stepStore.InitDeprovisionSteps(ctx, adapterID); err != nil {
			return fmt.Errorf("init deprovision steps: %w", err)
		}
		steps, err := p.stepStore.ListByAdapter(ctx, adapterID)
		if err != nil {
			return fmt.Errorf("load steps: %w", err)
		}
		for _, s := range steps {
			stepMap[s.StepName] = s
			// Extract resource identifiers from completed provision steps.
			switch s.StepName {
			case domain.StepCreateSNSTopic:
				if s.ResourceARN != nil {
					topicARN = *s.ResourceARN
				}
			case domain.StepSubscribeWebhook:
				if s.ResourceARN != nil {
					subscriptionARN = *s.ResourceARN
				}
			case domain.StepCreateConfigurationSet:
				if s.ResourceName != nil {
					configSetName = *s.ResourceName
				}
			}
		}
	}

	// Table-driven deprovision: each entry defines a step and its AWS call.
	type deprovStep struct {
		name string
		fn   func() error
	}
	deprovSteps := []deprovStep{
		{domain.StepDeprovUnsubscribeWebhook, func() error {
			if subscriptionARN == "" || subscriptionARN == "PendingConfirmation" {
				return nil
			}
			_, err := snsClient.Unsubscribe(ctx, &sns.UnsubscribeInput{SubscriptionArn: aws.String(subscriptionARN)})
			return err
		}},
		{domain.StepDeprovDeleteEventDestination, func() error {
			_, err := sesClient.DeleteConfigurationSetEventDestination(ctx, &sesv2.DeleteConfigurationSetEventDestinationInput{
				ConfigurationSetName: aws.String(configSetName), EventDestinationName: aws.String("senda-events"),
			})
			return err
		}},
		{domain.StepDeprovDeleteSNSTopic, func() error {
			if topicARN == "" {
				return nil
			}
			_, err := snsClient.DeleteTopic(ctx, &sns.DeleteTopicInput{TopicArn: aws.String(topicARN)})
			return err
		}},
		{domain.StepDeprovDeleteConfigurationSet, func() error {
			_, err := sesClient.DeleteConfigurationSet(ctx, &sesv2.DeleteConfigurationSetInput{
				ConfigurationSetName: aws.String(configSetName),
			})
			return err
		}},
	}

	var deprovErr error
	for _, step := range deprovSteps {
		if err := step.fn(); err != nil && !isNotFound(err) {
			p.persistStepFailure(ctx, stepMap, step.name, err.Error())
			deprovErr = errors.Join(deprovErr, fmt.Errorf("%s: %w", step.name, err))
		} else {
			p.persistStepSuccess(ctx, stepMap, step.name, nil, nil)
		}
	}

	if deprovErr != nil {
		return deprovErr
	}

	if p.stepStore != nil {
		if err := p.stepStore.DeleteByAdapter(ctx, adapterID); err != nil {
			p.logger.WarnContext(ctx, "failed to delete provisioning steps", "adapter_id", adapterID, "error", err)
		}
	}

	p.logger.InfoContext(ctx, "tracking resources deprovisioned",
		"adapter_id", adapterID,
		"config_set", configSetName,
		"topic_arn", topicARN,
	)

	return nil
}

// Compile-time interface check.
var _ port.Deprovisioner = (*TrackingProvisioner)(nil)
