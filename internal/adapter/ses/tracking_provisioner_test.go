package ses

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"

	"github.com/rendis/senda/internal/domain"
)

// --- Mock SES client ---

type mockProvisionSES struct {
	createConfigSetErr  error
	createEventDestErr  error
	deleteConfigSetErr  error
	deleteEventDestErr  error
}

func (m *mockProvisionSES) SendEmail(_ context.Context, _ *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	return nil, nil
}

func (m *mockProvisionSES) ListEmailIdentities(_ context.Context, _ *sesv2.ListEmailIdentitiesInput, _ ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	return nil, nil
}

func (m *mockProvisionSES) CreateConfigurationSet(_ context.Context, _ *sesv2.CreateConfigurationSetInput, _ ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetOutput, error) {
	if m.createConfigSetErr != nil {
		return nil, m.createConfigSetErr
	}
	return &sesv2.CreateConfigurationSetOutput{}, nil
}

func (m *mockProvisionSES) CreateConfigurationSetEventDestination(_ context.Context, _ *sesv2.CreateConfigurationSetEventDestinationInput, _ ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetEventDestinationOutput, error) {
	if m.createEventDestErr != nil {
		return nil, m.createEventDestErr
	}
	return &sesv2.CreateConfigurationSetEventDestinationOutput{}, nil
}

func (m *mockProvisionSES) DeleteConfigurationSet(_ context.Context, _ *sesv2.DeleteConfigurationSetInput, _ ...func(*sesv2.Options)) (*sesv2.DeleteConfigurationSetOutput, error) {
	if m.deleteConfigSetErr != nil {
		return nil, m.deleteConfigSetErr
	}
	return &sesv2.DeleteConfigurationSetOutput{}, nil
}

func (m *mockProvisionSES) DeleteConfigurationSetEventDestination(_ context.Context, _ *sesv2.DeleteConfigurationSetEventDestinationInput, _ ...func(*sesv2.Options)) (*sesv2.DeleteConfigurationSetEventDestinationOutput, error) {
	if m.deleteEventDestErr != nil {
		return nil, m.deleteEventDestErr
	}
	return &sesv2.DeleteConfigurationSetEventDestinationOutput{}, nil
}

// --- Mock SNS client ---

type mockProvisionSNS struct {
	createTopicARN string
	createTopicErr error
	subscribeARN   string
	subscribeErr   error
	unsubscribeErr error
	deleteTopicErr error

	// GetSubscriptionAttributes fields.
	getSubAttrsCalls  int
	getSubAttrsOutput func(call int) (*sns.GetSubscriptionAttributesOutput, error)

	// ListSubscriptionsByTopic fields.
	listSubsCalls    int
	listSubsEndpoint string
	listSubsOutput   func(call int) (*sns.ListSubscriptionsByTopicOutput, error)
}

func (m *mockProvisionSNS) CreateTopic(_ context.Context, _ *sns.CreateTopicInput, _ ...func(*sns.Options)) (*sns.CreateTopicOutput, error) {
	if m.createTopicErr != nil {
		return nil, m.createTopicErr
	}
	return &sns.CreateTopicOutput{
		TopicArn: aws.String(m.createTopicARN),
	}, nil
}

func (m *mockProvisionSNS) Subscribe(_ context.Context, _ *sns.SubscribeInput, _ ...func(*sns.Options)) (*sns.SubscribeOutput, error) {
	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}
	return &sns.SubscribeOutput{
		SubscriptionArn: aws.String(m.subscribeARN),
	}, nil
}

func (m *mockProvisionSNS) GetSubscriptionAttributes(_ context.Context, _ *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	m.getSubAttrsCalls++
	if m.getSubAttrsOutput != nil {
		return m.getSubAttrsOutput(m.getSubAttrsCalls)
	}
	return &sns.GetSubscriptionAttributesOutput{
		Attributes: map[string]string{"SubscriptionArn": m.subscribeARN},
	}, nil
}

func (m *mockProvisionSNS) Unsubscribe(_ context.Context, _ *sns.UnsubscribeInput, _ ...func(*sns.Options)) (*sns.UnsubscribeOutput, error) {
	if m.unsubscribeErr != nil {
		return nil, m.unsubscribeErr
	}
	return &sns.UnsubscribeOutput{}, nil
}

func (m *mockProvisionSNS) ListSubscriptionsByTopic(_ context.Context, input *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	m.listSubsCalls++
	if m.listSubsOutput != nil {
		return m.listSubsOutput(m.listSubsCalls)
	}
	endpoint := m.listSubsEndpoint
	if endpoint == "" {
		endpoint = "https://senda.example.com/api/v1/webhooks/ses/inbound"
	}
	return &sns.ListSubscriptionsByTopicOutput{
		Subscriptions: []snstypes.Subscription{
			{
				SubscriptionArn: aws.String(m.subscribeARN),
				Endpoint:        aws.String(endpoint),
				Protocol:        aws.String("https"),
				TopicArn:        input.TopicArn,
			},
		},
	}, nil
}

func (m *mockProvisionSNS) DeleteTopic(_ context.Context, _ *sns.DeleteTopicInput, _ ...func(*sns.Options)) (*sns.DeleteTopicOutput, error) {
	if m.deleteTopicErr != nil {
		return nil, m.deleteTopicErr
	}
	return &sns.DeleteTopicOutput{}, nil
}

// --- Mock stores (implements local adapterReadWriter + configCrypto interfaces) ---

type mockProvisionAdapterStore struct {
	adapter   *domain.Adapter
	getErr    error
	updateErr error
	updated   *domain.Adapter
}

func (m *mockProvisionAdapterStore) GetByID(_ context.Context, _ uuid.UUID) (*domain.Adapter, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.adapter, nil
}

func (m *mockProvisionAdapterStore) Update(_ context.Context, adapter *domain.Adapter) error {
	m.updated = adapter
	return m.updateErr
}

type mockProvisionCrypto struct {
	decrypted []byte
	encrypted []byte
}

func (m *mockProvisionCrypto) Encrypt(_ []byte) ([]byte, error) {
	return m.encrypted, nil
}

func (m *mockProvisionCrypto) Decrypt(_ []byte) ([]byte, error) {
	return m.decrypted, nil
}

// --- Mock step store ---

type mockStepStore struct {
	steps map[uuid.UUID]map[string]*domain.AdapterProvisioningStep // adapterID -> stepName -> step
}

func newMockStepStore() *mockStepStore {
	return &mockStepStore{steps: make(map[uuid.UUID]map[string]*domain.AdapterProvisioningStep)}
}

func (m *mockStepStore) InitSteps(_ context.Context, adapterID uuid.UUID) error {
	if _, ok := m.steps[adapterID]; !ok {
		m.steps[adapterID] = make(map[string]*domain.AdapterProvisioningStep)
	}
	for _, def := range domain.ProvisionStepDefs {
		if _, exists := m.steps[adapterID][def.Name]; !exists {
			m.steps[adapterID][def.Name] = &domain.AdapterProvisioningStep{
				ID: uuid.Must(uuid.NewV7()), AdapterID: adapterID,
				StepName: def.Name, StepOrder: def.Order, Status: domain.ProvisionStepPending,
			}
		}
	}
	return nil
}

func (m *mockStepStore) InitDeprovisionSteps(_ context.Context, adapterID uuid.UUID) error {
	if _, ok := m.steps[adapterID]; !ok {
		m.steps[adapterID] = make(map[string]*domain.AdapterProvisioningStep)
	}
	for _, def := range domain.DeprovisionStepDefs {
		if _, exists := m.steps[adapterID][def.Name]; !exists {
			m.steps[adapterID][def.Name] = &domain.AdapterProvisioningStep{
				ID: uuid.Must(uuid.NewV7()), AdapterID: adapterID,
				StepName: def.Name, StepOrder: def.Order, Status: domain.ProvisionStepPending,
			}
		}
	}
	return nil
}

func (m *mockStepStore) ListByAdapter(_ context.Context, adapterID uuid.UUID) ([]*domain.AdapterProvisioningStep, error) {
	adapterSteps, ok := m.steps[adapterID]
	if !ok {
		return nil, nil
	}
	out := make([]*domain.AdapterProvisioningStep, 0, len(adapterSteps))
	for _, s := range adapterSteps {
		out = append(out, s)
	}
	return out, nil
}

func (m *mockStepStore) MarkCompleted(_ context.Context, stepID uuid.UUID, resourceName, resourceARN *string) error {
	for _, adapterSteps := range m.steps {
		for _, s := range adapterSteps {
			if s.ID == stepID {
				s.Status = domain.ProvisionStepCompleted
				s.ResourceName = resourceName
				s.ResourceARN = resourceARN
				return nil
			}
		}
	}
	return nil
}

func (m *mockStepStore) MarkFailed(_ context.Context, stepID uuid.UUID, errMsg string) error {
	for _, adapterSteps := range m.steps {
		for _, s := range adapterSteps {
			if s.ID == stepID {
				s.Status = domain.ProvisionStepFailed
				s.ErrorMessage = &errMsg
				return nil
			}
		}
	}
	return nil
}

func (m *mockStepStore) ResetFailed(_ context.Context, adapterID uuid.UUID) error {
	for _, s := range m.steps[adapterID] {
		if s.Status == domain.ProvisionStepFailed {
			s.Status = domain.ProvisionStepPending
			s.ErrorMessage = nil
		}
	}
	return nil
}

func (m *mockStepStore) DeleteByAdapter(_ context.Context, adapterID uuid.UUID) error {
	delete(m.steps, adapterID)
	return nil
}

// setCompletedProvisionSteps pre-populates completed provision steps (for deprovision tests).
func (m *mockStepStore) setCompletedProvisionSteps(adapterID uuid.UUID, configSetName, topicARN, subARN string) {
	if _, ok := m.steps[adapterID]; !ok {
		m.steps[adapterID] = make(map[string]*domain.AdapterProvisioningStep)
	}
	destName := "senda-events"
	webhookURL := "https://senda.example.com/api/v1/webhooks/ses/inbound"
	steps := []struct {
		name     string
		order    int
		resName  *string
		resARN   *string
	}{
		{domain.StepCreateConfigurationSet, 1, &configSetName, nil},
		{domain.StepCreateSNSTopic, 2, nil, &topicARN},
		{domain.StepCreateEventDestination, 3, &destName, nil},
		{domain.StepSubscribeWebhook, 4, &webhookURL, &subARN},
		{domain.StepSaveConfiguration, 5, &configSetName, nil},
		{domain.StepVerifySubscription, 6, nil, &subARN},
	}
	for _, s := range steps {
		m.steps[adapterID][s.name] = &domain.AdapterProvisioningStep{
			ID: uuid.Must(uuid.NewV7()), AdapterID: adapterID,
			StepName: s.name, StepOrder: s.order, Status: domain.ProvisionStepCompleted,
			ResourceName: s.resName, ResourceARN: s.resARN,
		}
	}
}

// --- Tests ---

func newAlreadyExistsErr(code string) error {
	return &smithy.GenericAPIError{
		Code:    code,
		Message: "already exists",
		Fault:   smithy.FaultClient,
	}
}

func newAccessDeniedErr() error {
	return &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User is not authorized",
		Fault:   smithy.FaultClient,
	}
}

func setupProvisioner(sesMock *mockProvisionSES, snsMock *mockProvisionSNS, adapterStore *mockProvisionAdapterStore, crypto *mockProvisionCrypto) *TrackingProvisioner {
	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil, nil)
	p.clientFactory = func(_ aws.Config, _ string) (SESAPI, SNSAPI) {
		return sesMock, snsMock
	}
	return p
}

func TestProvision_Success(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID:              adapterID,
			AdapterType:     domain.AdapterTypeSES,
			ConfigEncrypted: cfgJSON, // mock crypto returns this as-is
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON, encrypted: []byte("encrypted")}
	sesMock := &mockProvisionSES{}
	topicARN := "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + adapterID.String()[:8]
	subARN := "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + adapterID.String()[:8] + ":sub-1"
	snsMock := &mockProvisionSNS{
		createTopicARN: topicARN,
		subscribeARN:   subARN,
	}

	p := setupProvisioner(sesMock, snsMock, adapterStore, crypto)
	result, err := p.Provision(context.Background(), adapterID)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	if len(result.Steps) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(result.Steps))
	}
	for _, step := range result.Steps {
		if step.Status != "created" {
			t.Errorf("step %q status = %q, want created", step.Name, step.Status)
		}
	}
	if result.TopicARN != topicARN {
		t.Errorf("TopicARN = %q, want %q", result.TopicARN, topicARN)
	}
	if result.SubscriptionARN != subARN {
		t.Errorf("SubscriptionARN = %q, want %q", result.SubscriptionARN, subARN)
	}
	if result.WebhookURL != "https://senda.example.com/api/v1/webhooks/ses/inbound" {
		t.Errorf("WebhookURL = %q", result.WebhookURL)
	}

	// Verify adapter was updated.
	if adapterStore.updated == nil {
		t.Fatal("expected adapter to be updated with configuration_set_name")
	}
}

func TestProvision_AlreadyExists_Idempotent(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID:              adapterID,
			AdapterType:     domain.AdapterTypeSES,
			ConfigEncrypted: cfgJSON,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON, encrypted: []byte("encrypted")}
	sesMock := &mockProvisionSES{
		createConfigSetErr: newAlreadyExistsErr("AlreadyExistsException"),
		createEventDestErr: newAlreadyExistsErr("EventDestinationAlreadyExistsException"),
	}
	topicARN := "arn:aws:sns:us-east-1:123456789:senda-ses-events-existing"
	snsMock := &mockProvisionSNS{
		createTopicARN: topicARN,
		subscribeARN:   "arn:aws:sns:sub-existing",
	}

	p := setupProvisioner(sesMock, snsMock, adapterStore, crypto)
	result, err := p.Provision(context.Background(), adapterID)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	// Config set and event destination should be "already_exists", rest "created".
	if result.Steps[0].Status != "already_exists" {
		t.Errorf("step 0 status = %q, want already_exists", result.Steps[0].Status)
	}
	if result.Steps[2].Status != "already_exists" {
		t.Errorf("step 2 status = %q, want already_exists", result.Steps[2].Status)
	}
}

func TestProvision_NonSESAdapter_Fails(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID:          adapterID,
			AdapterType: domain.AdapterTypeGmail,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: []byte("{}")}

	p := setupProvisioner(&mockProvisionSES{}, &mockProvisionSNS{}, adapterStore, crypto)
	_, err := p.Provision(context.Background(), adapterID)
	if err == nil {
		t.Fatal("expected error for non-SES adapter")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("error = %v, want ErrValidation", err)
	}
}

func TestProvision_AccessDenied_Fails(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID:              adapterID,
			AdapterType:     domain.AdapterTypeSES,
			ConfigEncrypted: cfgJSON,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON}
	sesMock := &mockProvisionSES{
		createConfigSetErr: newAccessDeniedErr(),
	}

	p := setupProvisioner(sesMock, &mockProvisionSNS{}, adapterStore, crypto)
	result, err := p.Provision(context.Background(), adapterID)
	if err == nil {
		t.Fatal("expected error for access denied")
	}
	if result == nil || len(result.Steps) == 0 {
		t.Fatal("expected partial result with failed step")
	}
	if result.Steps[0].Status != "failed" {
		t.Errorf("step 0 status = %q, want failed", result.Steps[0].Status)
	}
}

func TestProvision_PassesEndpointURLToClientFactory(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
		EndpointURL:     "http://localstack:4566",
	})

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID:              adapterID,
			AdapterType:     domain.AdapterTypeSES,
			ConfigEncrypted: cfgJSON,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON, encrypted: []byte("encrypted")}
	var gotEndpoint string

	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil, nil)
	p.clientFactory = func(_ aws.Config, endpointURL string) (SESAPI, SNSAPI) {
		gotEndpoint = endpointURL
		return &mockProvisionSES{}, &mockProvisionSNS{
			createTopicARN: "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + adapterID.String()[:8],
			subscribeARN:   "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + adapterID.String()[:8] + ":sub-1",
		}
	}

	if _, err := p.Provision(context.Background(), adapterID); err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if gotEndpoint != "http://localstack:4566" {
		t.Fatalf("clientFactory endpoint = %q, want %q", gotEndpoint, "http://localstack:4566")
	}
}

func TestIsAccessDenied(t *testing.T) {
	if !IsAccessDenied(newAccessDeniedErr()) {
		t.Error("expected IsAccessDenied to return true for AccessDeniedException")
	}
	if IsAccessDenied(errors.New("random error")) {
		t.Error("expected IsAccessDenied to return false for non-AWS error")
	}
}

// --- Verify Subscription Tests ---

func TestVerifySubscription_AlreadyConfirmed(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789:my-topic"
	endpoint := "https://senda.example.com/api/v1/webhooks/ses/inbound"
	confirmedARN := topicARN + ":sub-confirmed"

	snsMock := &mockProvisionSNS{
		subscribeARN: confirmedARN,
	}

	p := &TrackingProvisioner{verifyTimeout: 100 * time.Millisecond, verifyInterval: 10 * time.Millisecond}
	step := p.verifySubscription(context.Background(), snsMock, topicARN, endpoint)

	if step.Status != StepStatusCreated {
		t.Errorf("status = %q, want %q", step.Status, StepStatusCreated)
	}
	if snsMock.listSubsCalls != 1 {
		t.Errorf("ListSubscriptionsByTopic called %d times, want 1", snsMock.listSubsCalls)
	}
}

func TestVerifySubscription_PendingThenConfirmed(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789:my-topic"
	endpoint := "https://senda.example.com/api/v1/webhooks/ses/inbound"
	confirmedARN := topicARN + ":sub-confirmed"

	snsMock := &mockProvisionSNS{
		listSubsOutput: func(call int) (*sns.ListSubscriptionsByTopicOutput, error) {
			arn := "PendingConfirmation"
			if call > 2 {
				arn = confirmedARN
			}
			return &sns.ListSubscriptionsByTopicOutput{
				Subscriptions: []snstypes.Subscription{
					{SubscriptionArn: aws.String(arn), Endpoint: aws.String(endpoint), Protocol: aws.String("https"), TopicArn: aws.String(topicARN)},
				},
			}, nil
		},
	}

	p := &TrackingProvisioner{verifyTimeout: 1 * time.Second, verifyInterval: 10 * time.Millisecond}
	step := p.verifySubscription(context.Background(), snsMock, topicARN, endpoint)

	if step.Status != StepStatusCreated {
		t.Errorf("status = %q, want %q", step.Status, StepStatusCreated)
	}
	if snsMock.listSubsCalls < 3 {
		t.Errorf("ListSubscriptionsByTopic called %d times, want >= 3", snsMock.listSubsCalls)
	}
}

func TestVerifySubscription_Timeout(t *testing.T) {
	topicARN := "arn:aws:sns:us-east-1:123456789:my-topic"
	endpoint := "https://senda.example.com/api/v1/webhooks/ses/inbound"

	snsMock := &mockProvisionSNS{
		listSubsOutput: func(_ int) (*sns.ListSubscriptionsByTopicOutput, error) {
			return &sns.ListSubscriptionsByTopicOutput{
				Subscriptions: []snstypes.Subscription{
					{SubscriptionArn: aws.String("PendingConfirmation"), Endpoint: aws.String(endpoint), Protocol: aws.String("https"), TopicArn: aws.String(topicARN)},
				},
			}, nil
		},
	}

	p := &TrackingProvisioner{verifyTimeout: 50 * time.Millisecond, verifyInterval: 10 * time.Millisecond}
	step := p.verifySubscription(context.Background(), snsMock, topicARN, endpoint)

	if step.Status != StepStatusFailed {
		t.Errorf("status = %q, want %q", step.Status, StepStatusFailed)
	}
	if step.Detail == "" {
		t.Error("expected non-empty error detail on timeout")
	}
}

// --- Deprovision Tests ---

func newNotFoundErr() error {
	return &smithy.GenericAPIError{
		Code:    "NotFoundException",
		Message: "resource not found",
		Fault:   smithy.FaultClient,
	}
}

func TestDeprovision_Success(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	shortID := adapterID.String()[:8]
	configSetName := "senda-" + shortID
	topicARN := "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + shortID
	subARN := topicARN + ":sub-1"

	stepStore := newMockStepStore()
	stepStore.setCompletedProvisionSteps(adapterID, configSetName, topicARN, subARN)

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID: adapterID, AdapterType: domain.AdapterTypeSES, ConfigEncrypted: cfgJSON,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON}
	sesMock := &mockProvisionSES{}
	snsMock := &mockProvisionSNS{}

	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil, stepStore)
	p.clientFactory = func(_ aws.Config, _ string) (SESAPI, SNSAPI) { return sesMock, snsMock }

	err := p.Deprovision(context.Background(), adapterID)
	if err != nil {
		t.Fatalf("Deprovision() error = %v", err)
	}

	// All steps should have been cleaned up.
	if len(stepStore.steps[adapterID]) != 0 {
		t.Errorf("expected all steps deleted, got %d remaining", len(stepStore.steps[adapterID]))
	}
}

func TestDeprovision_ResourceNotFound_Idempotent(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	shortID := adapterID.String()[:8]
	configSetName := "senda-" + shortID
	topicARN := "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + shortID
	subARN := topicARN + ":sub-1"

	stepStore := newMockStepStore()
	stepStore.setCompletedProvisionSteps(adapterID, configSetName, topicARN, subARN)

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID: adapterID, AdapterType: domain.AdapterTypeSES, ConfigEncrypted: cfgJSON,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON}

	// All AWS resources already deleted — should be treated as success.
	sesMock := &mockProvisionSES{
		deleteConfigSetErr: newNotFoundErr(),
		deleteEventDestErr: newNotFoundErr(),
	}
	snsMock := &mockProvisionSNS{
		unsubscribeErr: newNotFoundErr(),
		deleteTopicErr: newNotFoundErr(),
	}

	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil, stepStore)
	p.clientFactory = func(_ aws.Config, _ string) (SESAPI, SNSAPI) { return sesMock, snsMock }

	err := p.Deprovision(context.Background(), adapterID)
	if err != nil {
		t.Fatalf("Deprovision() with not-found resources should succeed, got error = %v", err)
	}
}

func TestDeprovision_PartialFailure(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	shortID := adapterID.String()[:8]
	configSetName := "senda-" + shortID
	topicARN := "arn:aws:sns:us-east-1:123456789:senda-ses-events-" + shortID
	subARN := topicARN + ":sub-1"

	stepStore := newMockStepStore()
	stepStore.setCompletedProvisionSteps(adapterID, configSetName, topicARN, subARN)

	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID: adapterID, AdapterType: domain.AdapterTypeSES, ConfigEncrypted: cfgJSON,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: cfgJSON}

	// DeleteTopic fails with a real error (not NotFound).
	sesMock := &mockProvisionSES{}
	snsMock := &mockProvisionSNS{
		deleteTopicErr: newAccessDeniedErr(),
	}

	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil, stepStore)
	p.clientFactory = func(_ aws.Config, _ string) (SESAPI, SNSAPI) { return sesMock, snsMock }

	err := p.Deprovision(context.Background(), adapterID)
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	// Steps should NOT be fully cleaned up (deprovision not complete).
	if _, exists := stepStore.steps[adapterID]; !exists {
		t.Error("expected steps to still exist after partial deprovision failure")
	}
}

func TestDeprovision_NonSESAdapter_Fails(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	adapterStore := &mockProvisionAdapterStore{
		adapter: &domain.Adapter{
			ID: adapterID, AdapterType: domain.AdapterTypeGmail,
		},
	}
	crypto := &mockProvisionCrypto{decrypted: []byte("{}")}

	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil, nil)
	err := p.Deprovision(context.Background(), adapterID)
	if err == nil {
		t.Fatal("expected error for non-SES adapter")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("error = %v, want ErrValidation", err)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"NotFoundException", newNotFoundErr(), true},
		{"ResourceNotFoundException", &smithy.GenericAPIError{Code: "ResourceNotFoundException"}, true},
		{"InvalidParameterException", &smithy.GenericAPIError{Code: "InvalidParameterException"}, true},
		{"AccessDeniedException", newAccessDeniedErr(), false},
		{"plain error", errors.New("random"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFound(tt.err); got != tt.want {
				t.Errorf("isNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}
