package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
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

	// Factory that returns nil for unsupported adapters (like SMTP).
	nilFactory := func(_ context.Context, _ *domain.Adapter, _ []byte) (port.IdentityProvider, error) {
		return nil, nil
	}

	svc := service.NewIdentityService(
		&mockAdapterIdentityStoreSend{},
		adapterStore,
		&mockIdentityCrypto{},
		nilFactory,
	)

	_, err := svc.SyncIdentities(context.Background(), adapterID)
	if err == nil {
		t.Fatal("expected unsupported adapter error")
	}
	if !strings.Contains(err.Error(), "does not support identity listing") {
		t.Fatalf("expected unsupported-listing error, got %v", err)
	}
}
