package resolution

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// MergedInjectors maps injector_name -> field_name -> resolved value.
type MergedInjectors map[string]map[string]any

// InjectorMerger resolves all injector values for a workspace,
// merging values across the resolution chain with priority ordering.
type InjectorMerger struct {
	store         port.InjectorStore
	chainResolver *ChainResolver
}

// NewInjectorMerger creates an InjectorMerger with the given dependencies.
func NewInjectorMerger(store port.InjectorStore, cr *ChainResolver) *InjectorMerger {
	return &InjectorMerger{
		store:         store,
		chainResolver: cr,
	}
}

// Resolve returns all merged injector values for the given workspace,
// applying the resolution chain priority (workspace > _system > global).
func (m *InjectorMerger) Resolve(ctx context.Context, workspaceID uuid.UUID) (MergedInjectors, error) {
	chain, err := m.chainResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	allDefs, err := m.store.ListDefinitionsInChain(ctx, chain.Scopes)
	if err != nil {
		return nil, err
	}

	// Deduplicate definitions by name: highest-priority scope wins.
	type dedupEntry struct {
		defID uuid.UUID
		index int // position in scopes (lower = higher priority)
	}
	bestByName := make(map[string]dedupEntry)

	for _, def := range allDefs {
		scopeIdx := scopeIndex(def.WorkspaceID, chain.Scopes)
		existing, found := bestByName[def.Name]
		if !found || scopeIdx < existing.index {
			bestByName[def.Name] = dedupEntry{defID: def.ID, index: scopeIdx}
		}
	}

	result := make(MergedInjectors, len(bestByName))

	for name, entry := range bestByName {
		fields, err := m.store.GetFieldsByDefinition(ctx, entry.defID)
		if err != nil {
			return nil, err
		}

		values, err := m.store.GetValues(ctx, entry.defID, chain.Scopes)
		if err != nil {
			return nil, err
		}

		fieldMap := make(map[string]any, len(fields))
		for _, f := range fields {
			fieldMap[f.FieldName] = nil // default: no value
		}

		// Index values by field+scope for priority resolution
		for _, f := range fields {
			resolved := resolveFieldValue(f.FieldName, values, chain.Scopes)
			fieldMap[f.FieldName] = resolved
		}

		result[name] = fieldMap
	}

	return result, nil
}

// resolveFieldValue finds the highest-priority value for a field
// by iterating through scopes in order.
func resolveFieldValue(fieldName string, values []*domain.InjectorValue, scopes []uuid.NullUUID) any {
	for _, scope := range scopes {
		for _, v := range values {
			if v.FieldName != fieldName {
				continue
			}
			if matchScope(v.WorkspaceID, scope) {
				var parsed any
				if err := json.Unmarshal([]byte(v.Value), &parsed); err != nil {
					return v.Value // fallback to raw string
				}
				return parsed
			}
		}
	}
	return nil
}

// matchScope checks if a *uuid.UUID matches a uuid.NullUUID scope.
func matchScope(wsID *uuid.UUID, scope uuid.NullUUID) bool {
	if wsID == nil && !scope.Valid {
		return true // both global
	}
	if wsID != nil && scope.Valid && *wsID == scope.UUID {
		return true
	}
	return false
}

// scopeIndex returns the position of a *uuid.UUID in the scopes list.
// Lower index = higher priority.
func scopeIndex(wsID *uuid.UUID, scopes []uuid.NullUUID) int {
	for i, s := range scopes {
		if matchScope(wsID, s) {
			return i
		}
	}
	return len(scopes) // not found = lowest priority
}
