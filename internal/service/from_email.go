package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

func resolveFromEmail(identityStore port.AdapterIdentityStore, ctx context.Context, adapter *domain.Adapter, senderIdentityID *uuid.UUID) (string, error) {
	if senderIdentityID != nil {
		identity, err := identityStore.GetByID(ctx, *senderIdentityID)
		if err != nil {
			return "", fmt.Errorf("sender identity %s not found: %w", *senderIdentityID, err)
		}
		if identity.IdentityType != domain.IdentityTypeEmail {
			return "", fmt.Errorf("%w: sender identity %s is not an email", domain.ErrSenderIdentityAccessDenied, *senderIdentityID)
		}
		return identity.Identity, nil
	}

	identity, err := identityStore.GetDefault(ctx, adapter.ID)
	if err != nil {
		return "", fmt.Errorf("%w: adapter %s", domain.ErrNoDefaultIdentity, adapter.ID)
	}
	if identity.IdentityType != domain.IdentityTypeEmail {
		return "", fmt.Errorf("%w: adapter %s default is a domain, not an email", domain.ErrNoDefaultIdentity, adapter.ID)
	}
	return identity.Identity, nil
}

func resolveFromEmailForTemplateTest(identityStore port.AdapterIdentityStore, ctx context.Context, adapter *domain.Adapter, decryptedConfig []byte, senderIdentityID *uuid.UUID) (string, error) {
	if senderIdentityID != nil {
		return resolveFromEmail(identityStore, ctx, adapter, senderIdentityID)
	}

	from := resolution.ResolveFromAddress(ctx, identityStore, adapter, decryptedConfig)
	if from.Address == "" {
		return "", fmt.Errorf("%w: no sender identity for adapter %s", domain.ErrNoDefaultIdentity, adapter.Name)
	}
	return from.Address, nil
}
