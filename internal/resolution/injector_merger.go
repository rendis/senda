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

	// Collect all winning definition IDs for batch fetch.
	defIDs := make([]uuid.UUID, 0, len(bestByName))
	for _, entry := range bestByName {
		defIDs = append(defIDs, entry.defID)
	}

	// Batch fetch all fields and values in two queries instead of 2*N.
	allFields, err := m.store.GetAllFieldsByDefinitions(ctx, defIDs)
	if err != nil {
		return nil, err
	}

	allValues, err := m.store.GetAllValuesByDefinitions(ctx, defIDs, chain.Scopes)
	if err != nil {
		return nil, err
	}

	// Build pre-indexed lookup for O(1) field value resolution.
	valIdx := buildValueIndex(allValues)

	result := make(MergedInjectors, len(bestByName))

	for name, entry := range bestByName {
		fields := allFields[entry.defID]

		fieldMap := make(map[string]any, len(fields))
		for _, f := range fields {
			fieldMap[f.FieldName] = nil // default: no value
		}

		defIdx := valIdx[entry.defID]
		for _, f := range fields {
			resolved := resolveFieldValueIndexed(f.FieldName, chain.Scopes, defIdx)
			fieldMap[f.FieldName] = resolved
		}

		result[name] = fieldMap
	}

	return result, nil
}

// valueIndex maps defID -> fieldName -> scopeKey -> *InjectorValue for O(1) lookup.
type valueIndex map[uuid.UUID]map[string]map[string]*domain.InjectorValue

// buildValueIndex constructs a pre-indexed lookup from the batch-fetched values.
func buildValueIndex(allValues map[uuid.UUID][]*domain.InjectorValue) valueIndex {
	idx := make(valueIndex, len(allValues))
	for defID, values := range allValues {
		if idx[defID] == nil {
			idx[defID] = make(map[string]map[string]*domain.InjectorValue)
		}
		for _, v := range values {
			if idx[defID][v.FieldName] == nil {
				idx[defID][v.FieldName] = make(map[string]*domain.InjectorValue)
			}
			key := scopeKey(v.WorkspaceID)
			idx[defID][v.FieldName][key] = v
		}
	}
	return idx
}

// scopeKey returns a string key for a workspace scope pointer.
// nil (global) maps to the empty string.
func scopeKey(wsID *uuid.UUID) string {
	if wsID == nil {
		return ""
	}
	return wsID.String()
}

// scopeKeyFromNullUUID returns a string key from a uuid.NullUUID.
func scopeKeyFromNullUUID(scope uuid.NullUUID) string {
	if !scope.Valid {
		return ""
	}
	return scope.UUID.String()
}

// parseJSONValue attempts to parse a JSON string into a Go value.
// Falls back to the raw string on parse failure.
func parseJSONValue(raw string) any {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	return parsed
}

// resolveFieldValueIndexed uses the pre-built index for O(1) field value lookup
// per scope, iterating scopes in priority order.
func resolveFieldValueIndexed(fieldName string, scopes []uuid.NullUUID, fieldIdx map[string]map[string]*domain.InjectorValue) any {
	if fieldIdx == nil {
		return nil
	}
	scopeValues, ok := fieldIdx[fieldName]
	if !ok {
		return nil
	}
	for _, scope := range scopes {
		key := scopeKeyFromNullUUID(scope)
		if v, found := scopeValues[key]; found {
			return parseJSONValue(v.Value)
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
