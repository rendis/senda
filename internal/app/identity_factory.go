package app

import (
	"context"
	"encoding/json"
	"fmt"

	gmailadapter "github.com/rendis/senda/internal/adapter/gmail"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// DefaultIdentityProviderFactory creates IdentityProvider instances based on adapter type.
// Lives in app/ because it imports concrete adapters (ses, gmail), which the service layer must not.
func DefaultIdentityProviderFactory(ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte) (port.IdentityProvider, error) {
	switch adapter.AdapterType {
	case domain.AdapterTypeSES:
		return newSESIdentityProvider(ctx, decryptedConfig)
	case domain.AdapterTypeGmail:
		return newGmailIdentityProvider(ctx, decryptedConfig)
	default:
		return nil, nil // SMTP and others: no auto-sync
	}
}

func newSESIdentityProvider(ctx context.Context, decryptedConfig []byte) (port.IdentityProvider, error) {
	var cfg sesadapter.Config
	if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal SES config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrValidation, err)
	}

	provider, err := sesadapter.NewAdapterFromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init SES identity provider: %w", err)
	}
	return provider, nil
}

func newGmailIdentityProvider(ctx context.Context, decryptedConfig []byte) (port.IdentityProvider, error) {
	var cfg gmailadapter.GmailConfig
	if err := json.Unmarshal(decryptedConfig, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal Gmail config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrValidation, err)
	}

	provider, err := gmailadapter.NewAdapterFromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init Gmail identity provider: %w", err)
	}
	return provider, nil
}
