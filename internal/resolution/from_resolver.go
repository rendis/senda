package resolution

import (
	"context"
	"encoding/json"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// ResolveFromAddress determines the sender email address for an adapter by trying:
// 1. The default identity from the identity store.
// 2. The delegate_email from adapter config_meta.
// 3. The delegate_email from the decrypted config JSON (backfill fallback).
func ResolveFromAddress(ctx context.Context, identityStore port.AdapterIdentityStore, adapter *domain.Adapter, decryptedConfig []byte) port.EmailAddress {
	var from port.EmailAddress

	// 1. Try default identity from DB.
	identity, err := identityStore.GetDefault(ctx, adapter.ID)
	if err == nil {
		from.Address = identity.Identity
		if identity.DisplayName != nil {
			from.Name = *identity.DisplayName
		}
		return from
	}

	// 2. Try config_meta (populated on create/update).
	if de := adapter.ConfigMeta["delegate_email"]; de != "" {
		from.Address = de
		return from
	}

	// 3. Fallback: read delegate_email from decrypted config (backfill not yet run).
	var cfgMap map[string]any
	if json.Unmarshal(decryptedConfig, &cfgMap) == nil {
		if de, ok := cfgMap["delegate_email"].(string); ok && de != "" {
			from.Address = de
		}
	}
	return from
}
