package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"

	sesadapter "github.com/senda-app/senda/internal/adapter/ses"
	"github.com/senda-app/senda/internal/domain"
)

// --- Mock SES client ---

type mockProvisionSES struct {
	createConfigSetErr error
	createEventDestErr error
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

// --- Mock SNS client ---

type mockProvisionSNS struct {
	createTopicARN string
	createTopicErr error
	subscribeARN   string
	subscribeErr   error
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
	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil)
	p.clientFactory = func(_ aws.Config, _ string) (sesadapter.SESAPI, sesadapter.SNSAPI) {
		return sesMock, snsMock
	}
	return p
}

func TestProvision_Success(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	cfgJSON, _ := json.Marshal(sesAdapterConfig{
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

	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
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
	cfgJSON, _ := json.Marshal(sesAdapterConfig{
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
	cfgJSON, _ := json.Marshal(sesAdapterConfig{
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
	cfgJSON, _ := json.Marshal(sesAdapterConfig{
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

	p := NewTrackingProvisioner(adapterStore, crypto, "https://senda.example.com", nil)
	p.clientFactory = func(_ aws.Config, endpointURL string) (sesadapter.SESAPI, sesadapter.SNSAPI) {
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
