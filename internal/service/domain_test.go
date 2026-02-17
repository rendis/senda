package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// --- Mocks ---

type mockDomainStore struct {
	createFn              func(ctx context.Context, d *domain.Domain) error
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Domain, error)
	updateFn              func(ctx context.Context, d *domain.Domain) error
	softDeleteFn          func(ctx context.Context, id uuid.UUID) error
	listInChainFn         func(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error)
	listByWorkspaceFn     func(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Domain], error)
	getPendingFn          func(ctx context.Context, limit int) ([]*domain.Domain, error)
}

func (m *mockDomainStore) Create(ctx context.Context, d *domain.Domain) error {
	if m.createFn != nil {
		return m.createFn(ctx, d)
	}
	return nil
}
func (m *mockDomainStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockDomainStore) Update(ctx context.Context, d *domain.Domain) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, d)
	}
	return nil
}
func (m *mockDomainStore) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockDomainStore) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error) {
	if m.listInChainFn != nil {
		return m.listInChainFn(ctx, scopes)
	}
	return nil, nil
}
func (m *mockDomainStore) ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Domain], error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, opts)
	}
	return nil, nil
}
func (m *mockDomainStore) GetPendingVerifications(ctx context.Context, limit int) ([]*domain.Domain, error) {
	if m.getPendingFn != nil {
		return m.getPendingFn(ctx, limit)
	}
	return nil, nil
}

type mockCrypto struct {
	encryptFn func(plaintext []byte) ([]byte, error)
	decryptFn func(ciphertext []byte) ([]byte, error)
}

func (m *mockCrypto) Encrypt(plaintext []byte) ([]byte, error) {
	if m.encryptFn != nil {
		return m.encryptFn(plaintext)
	}
	return append([]byte("enc:"), plaintext...), nil
}
func (m *mockCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	if m.decryptFn != nil {
		return m.decryptFn(ciphertext)
	}
	return ciphertext[4:], nil
}

type mockJobQueue struct {
	enqueueSendFn        func(ctx context.Context, job *port.SendJob) error
	enqueueDomainCheckFn func(ctx context.Context, domainID uuid.UUID) error
	enqueueWebhookFn     func(ctx context.Context, job *port.WebhookJob) error
}

func (m *mockJobQueue) EnqueueSend(ctx context.Context, job *port.SendJob) error {
	if m.enqueueSendFn != nil {
		return m.enqueueSendFn(ctx, job)
	}
	return nil
}
func (m *mockJobQueue) EnqueueDomainCheck(ctx context.Context, domainID uuid.UUID) error {
	if m.enqueueDomainCheckFn != nil {
		return m.enqueueDomainCheckFn(ctx, domainID)
	}
	return nil
}
func (m *mockJobQueue) EnqueueWebhook(ctx context.Context, job *port.WebhookJob) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, job)
	}
	return nil
}

// --- Tests ---

func TestDomainService_Register_Success(t *testing.T) {
	var created *domain.Domain
	store := &mockDomainStore{
		createFn: func(_ context.Context, d *domain.Domain) error {
			created = d
			return nil
		},
	}
	crypto := &mockCrypto{}
	jq := &mockJobQueue{}

	svc := service.NewDomainService(store, crypto, jq)

	wsID := uuid.Must(uuid.NewV7())
	d, err := svc.Register(context.Background(), "example.com", &wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.DomainName != "example.com" {
		t.Fatalf("expected domain_name 'example.com', got %q", d.DomainName)
	}
	if d.DKIMSelector != "senda" {
		t.Fatalf("expected selector 'senda', got %q", d.DKIMSelector)
	}
	if d.DKIMPublicKey == "" {
		t.Fatal("expected non-empty public key")
	}
	if len(d.DKIMPrivateKeyEncrypted) == 0 {
		t.Fatal("expected non-empty encrypted private key")
	}
	if d.Status != domain.DomainStatusPending {
		t.Fatalf("expected status 'pending', got %q", d.Status)
	}
	if len(d.DNSRecords) != 1 {
		t.Fatalf("expected 1 DNS record, got %d", len(d.DNSRecords))
	}
	if d.WorkspaceID == nil || *d.WorkspaceID != wsID {
		t.Fatalf("expected workspace ID %s", wsID)
	}
	if created == nil {
		t.Fatal("expected domain to be persisted")
	}
}

func TestDomainService_Register_CryptoError(t *testing.T) {
	store := &mockDomainStore{}
	crypto := &mockCrypto{
		encryptFn: func(_ []byte) ([]byte, error) {
			return nil, errors.New("crypto failure")
		},
	}
	jq := &mockJobQueue{}

	svc := service.NewDomainService(store, crypto, jq)

	_, err := svc.Register(context.Background(), "example.com", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "crypto failure" {
		t.Fatalf("expected 'crypto failure', got %q", err.Error())
	}
}

func TestDomainService_Register_StoreError(t *testing.T) {
	store := &mockDomainStore{
		createFn: func(_ context.Context, _ *domain.Domain) error {
			return domain.ErrConflict
		},
	}
	crypto := &mockCrypto{}
	jq := &mockJobQueue{}

	svc := service.NewDomainService(store, crypto, jq)

	_, err := svc.Register(context.Background(), "example.com", nil)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestDomainService_RequestVerification_Success(t *testing.T) {
	var enqueuedID uuid.UUID
	jq := &mockJobQueue{
		enqueueDomainCheckFn: func(_ context.Context, id uuid.UUID) error {
			enqueuedID = id
			return nil
		},
	}
	svc := service.NewDomainService(&mockDomainStore{}, &mockCrypto{}, jq)

	domainID := uuid.Must(uuid.NewV7())
	err := svc.RequestVerification(context.Background(), domainID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueuedID != domainID {
		t.Fatalf("expected enqueued domain ID %s, got %s", domainID, enqueuedID)
	}
}

func TestDomainService_RequestVerification_Error(t *testing.T) {
	jq := &mockJobQueue{
		enqueueDomainCheckFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("queue error")
		},
	}
	svc := service.NewDomainService(&mockDomainStore{}, &mockCrypto{}, jq)

	err := svc.RequestVerification(context.Background(), uuid.Must(uuid.NewV7()))
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "queue error" {
		t.Fatalf("expected 'queue error', got %q", err.Error())
	}
}
