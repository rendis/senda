package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/service"
)

type mockIdentityCrypto struct {
	decryptFn func(ciphertext []byte) ([]byte, error)
}

func (m *mockIdentityCrypto) Encrypt(_ []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (m *mockIdentityCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	if m.decryptFn != nil {
		return m.decryptFn(ciphertext)
	}
	return ciphertext, nil
}

func TestDefaultIdentityProviderFactory_SESMissingRegion(t *testing.T) {
	adapter := &domain.Adapter{ID: uuid.Must(uuid.NewV7()), AdapterType: domain.AdapterTypeSES}

	_, err := service.DefaultIdentityProviderFactory(adapter, []byte(`{}`))
	if err == nil {
		t.Fatal("expected validation error for missing SES region")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestDefaultIdentityProviderFactory_GmailMissingFields(t *testing.T) {
	adapter := &domain.Adapter{ID: uuid.Must(uuid.NewV7()), AdapterType: domain.AdapterTypeGmail}

	_, err := service.DefaultIdentityProviderFactory(adapter, []byte(`{"oauth_client_id":"id-only"}`))
	if err == nil {
		t.Fatal("expected validation error for missing Gmail OAuth fields")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestIdentityService_SyncIdentities_UnsupportedAdapter(t *testing.T) {
	adapterID := uuid.Must(uuid.NewV7())
	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				return nil, domain.ErrNotFound
			}
			return &domain.Adapter{
				ID:              adapterID,
				AdapterType:     domain.AdapterType("smtp"),
				ConfigEncrypted: []byte(`{"host":"smtp.example.com"}`),
			}, nil
		},
	}

	svc := service.NewIdentityService(
		&mockAdapterIdentityStoreSend{},
		adapterStore,
		&mockIdentityCrypto{},
		nil, // should default to DefaultIdentityProviderFactory
	)

	_, err := svc.SyncIdentities(context.Background(), adapterID)
	if err == nil {
		t.Fatal("expected unsupported adapter error")
	}
	if !strings.Contains(err.Error(), "does not support identity listing") {
		t.Fatalf("expected unsupported-listing error, got %v", err)
	}
}
