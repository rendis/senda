package app

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
)

func TestDefaultIdentityProviderFactory_SESMissingRegion(t *testing.T) {
	adapter := &domain.Adapter{ID: uuid.Must(uuid.NewV7()), AdapterType: domain.AdapterTypeSES}

	_, err := DefaultIdentityProviderFactory(context.Background(), adapter, []byte(`{}`))
	if err == nil {
		t.Fatal("expected validation error for missing SES region")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestDefaultIdentityProviderFactory_GmailMissingFields(t *testing.T) {
	adapter := &domain.Adapter{ID: uuid.Must(uuid.NewV7()), AdapterType: domain.AdapterTypeGmail}

	_, err := DefaultIdentityProviderFactory(context.Background(), adapter, []byte(`{"oauth_client_id":"id-only"}`))
	if err == nil {
		t.Fatal("expected validation error for missing Gmail OAuth fields")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestDefaultIdentityProviderFactory_SMTPReturnsNil(t *testing.T) {
	adapter := &domain.Adapter{ID: uuid.Must(uuid.NewV7()), AdapterType: domain.AdapterType("smtp")}

	provider, err := DefaultIdentityProviderFactory(context.Background(), adapter, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != nil {
		t.Fatal("expected nil provider for SMTP adapter")
	}
}
