package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// MergedInjectors maps injector_name -> field_name -> resolved value.
type MergedInjectors map[string]map[string]any

// defaultCodeInjectorTimeout is the fallback timeout for code injectors.
const defaultCodeInjectorTimeout = 30 * time.Second

// InjectorMerger resolves all injector values for a workspace,
// merging values across the resolution chain with priority ordering.
// It also resolves user-provided code injectors and merges them in.
type InjectorMerger struct {
	store          port.InjectorStore
	chainResolver  *ChainResolver
	codeInjectors  []port.CodeInjector
	codeInitFunc   port.CodeInitFunc
}

// NewInjectorMerger creates an InjectorMerger with the given dependencies.
func NewInjectorMerger(
	store port.InjectorStore,
	cr *ChainResolver,
	codeInjectors []port.CodeInjector,
	codeInitFunc port.CodeInitFunc,
) *InjectorMerger {
	return &InjectorMerger{
		store:         store,
		chainResolver: cr,
		codeInjectors: codeInjectors,
		codeInitFunc:  codeInitFunc,
	}
}

// HasCodeInjectors returns true if user-provided code injectors are registered.
func (m *InjectorMerger) HasCodeInjectors() bool {
	return len(m.codeInjectors) > 0 || m.codeInitFunc != nil
}

// Resolve returns all merged injector values for the given workspace,
// applying the resolution chain priority (workspace > _system > global).
// This is the DB-only path, preserved for backward compatibility.
func (m *InjectorMerger) Resolve(ctx context.Context, workspaceID uuid.UUID) (MergedInjectors, error) {
	return m.resolveDB(ctx, workspaceID)
}

// ResolveWithContext resolves both DB injectors and code injectors,
// merging them into a single MergedInjectors map. Code injectors take
// precedence over DB injectors with the same name (with a warning).
func (m *InjectorMerger) ResolveWithContext(ctx context.Context, workspaceID uuid.UUID, injCtx *port.InjectorContext) (MergedInjectors, error) {
	// 1. Resolve DB injectors.
	dbValues, err := m.resolveDB(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// If no code injectors, return DB-only.
	if len(m.codeInjectors) == 0 {
		return dbValues, nil
	}

	// 2. Seed context with DB values so code injectors can reference them.
	injCtx.MergeDBInjectors(dbValues)

	// 3. Run init function if set.
	if m.codeInitFunc != nil {
		initData, initErr := m.codeInitFunc(ctx, injCtx)
		if initErr != nil {
			return nil, fmt.Errorf("code init func: %w", initErr)
		}
		injCtx.SetInitData(initData)
	}

	// 4. Resolve code injectors (respecting dependency order).
	codeValues, err := m.resolveCodeInjectors(ctx, injCtx)
	if err != nil {
		return nil, err
	}

	// 5. Merge: code injectors override DB on name collision.
	for name, fields := range codeValues {
		if _, exists := dbValues[name]; exists {
			slog.Warn("code injector overrides DB injector", "code", name)
		}
		dbValues[name] = fields
	}

	return dbValues, nil
}

// resolveDB resolves DB injectors only (existing logic).
func (m *InjectorMerger) resolveDB(ctx context.Context, workspaceID uuid.UUID) (MergedInjectors, error) {
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
		index int
	}
	bestByName := make(map[string]dedupEntry)

	for _, def := range allDefs {
		scopeIdx := scopeIndex(def.WorkspaceID, chain.Scopes)
		existing, found := bestByName[def.Name]
		if !found || scopeIdx < existing.index {
			bestByName[def.Name] = dedupEntry{defID: def.ID, index: scopeIdx}
		}
	}

	defIDs := make([]uuid.UUID, 0, len(bestByName))
	for _, entry := range bestByName {
		defIDs = append(defIDs, entry.defID)
	}

	allFields, err := m.store.GetAllFieldsByDefinitions(ctx, defIDs)
	if err != nil {
		return nil, err
	}

	allValues, err := m.store.GetAllValuesByDefinitions(ctx, defIDs, chain.Scopes)
	if err != nil {
		return nil, err
	}

	valIdx := buildValueIndex(allValues)

	result := make(MergedInjectors, len(bestByName))

	for name, entry := range bestByName {
		fields := allFields[entry.defID]

		fieldMap := make(map[string]any, len(fields))
		for _, f := range fields {
			fieldMap[f.FieldName] = nil
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

// resolveCodeInjectors runs all code injectors respecting dependency order.
func (m *InjectorMerger) resolveCodeInjectors(ctx context.Context, injCtx *port.InjectorContext) (MergedInjectors, error) {
	if len(m.codeInjectors) == 0 {
		return nil, nil
	}

	// Build dependency graph and resolve in topological order.
	byCode := make(map[string]port.CodeInjector, len(m.codeInjectors))
	for _, inj := range m.codeInjectors {
		byCode[inj.Code()] = inj
	}

	resolved := make(MergedInjectors, len(m.codeInjectors))
	visited := make(map[string]bool)

	var resolveOne func(code string) error
	resolveOne = func(code string) error {
		if visited[code] {
			return nil
		}
		visited[code] = true

		inj, ok := byCode[code]
		if !ok {
			return nil // dependency is a DB injector or unknown; skip
		}

		resolveFn, deps := inj.Resolve()

		// Resolve dependencies first.
		for _, dep := range deps {
			if err := resolveOne(dep); err != nil {
				return err
			}
		}

		// Execute with timeout.
		timeout := inj.Timeout()
		if timeout == 0 {
			timeout = defaultCodeInjectorTimeout
		}

		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		fields, err := resolveFn(execCtx, injCtx)
		if err != nil {
			if inj.IsCritical() {
				return fmt.Errorf("critical code injector %q failed: %w", code, err)
			}
			slog.Warn("non-critical code injector failed", "code", code, "error", err)
			return nil
		}

		resolved[code] = fields
		injCtx.SetResolved(code, fields)
		return nil
	}

	for _, inj := range m.codeInjectors {
		if err := resolveOne(inj.Code()); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

// --- existing helper functions below (unchanged) ---

// valueIndex maps defID -> fieldName -> scopeKey -> *InjectorValue for O(1) lookup.
type valueIndex map[uuid.UUID]map[string]map[string]*domain.InjectorValue

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

func scopeKey(wsID *uuid.UUID) string {
	if wsID == nil {
		return ""
	}
	return wsID.String()
}

func scopeKeyFromNullUUID(scope uuid.NullUUID) string {
	if !scope.Valid {
		return ""
	}
	return scope.UUID.String()
}

func parseJSONValue(raw string) any {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	return parsed
}

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

func matchScope(wsID *uuid.UUID, scope uuid.NullUUID) bool {
	if wsID == nil && !scope.Valid {
		return true
	}
	if wsID != nil && scope.Valid && *wsID == scope.UUID {
		return true
	}
	return false
}

func scopeIndex(wsID *uuid.UUID, scopes []uuid.NullUUID) int {
	for i, s := range scopes {
		if matchScope(wsID, s) {
			return i
		}
	}
	return len(scopes)
}
