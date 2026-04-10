package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// MergedInjectors maps injector_name -> field_name -> resolved value.
type MergedInjectors map[string]map[string]any

// defaultCodeInjectorTimeout is the fallback timeout for code injectors.
const defaultCodeInjectorTimeout = 30 * time.Second

type injectorFieldRule struct {
	DefaultValue   any
	AllowOverwrite bool
}

// InjectorMerger resolves DB injectors through the documented scope chain and
// then applies runtime request/code overrides on top of the resolved base value.
type InjectorMerger struct {
	store         port.InjectorStore
	chainResolver *ChainResolver
	cache         port.Cache
	codeInjectors []port.CodeInjector
	codeInitFunc  port.CodeInitFunc
}

// NewInjectorMerger creates an InjectorMerger with the given dependencies.
func NewInjectorMerger(
	store port.InjectorStore,
	cr *ChainResolver,
	cache port.Cache,
	codeInjectors []port.CodeInjector,
	codeInitFunc port.CodeInitFunc,
) *InjectorMerger {
	return &InjectorMerger{
		store:         store,
		chainResolver: cr,
		cache:         cache,
		codeInjectors: codeInjectors,
		codeInitFunc:  codeInitFunc,
	}
}

// HasCodeInjectors returns true if user-provided code injectors are registered.
func (m *InjectorMerger) HasCodeInjectors() bool {
	return len(m.codeInjectors) > 0 || m.codeInitFunc != nil
}

// StaticCatalog synthesizes UI/catalog injector definitions for static
// code injectors, optionally resolving their effective values for a workspace.
func (m *InjectorMerger) StaticCatalog(ctx context.Context, workspace *domain.Workspace, templateType string) ([]*domain.InjectorDefinition, map[uuid.UUID][]*domain.InjectorField, error) {
	staticInjectors := m.staticCatalogInjectors()
	if len(staticInjectors) == 0 {
		return nil, map[uuid.UUID][]*domain.InjectorField{}, nil
	}

	var (
		workspaceID  uuid.UUID
		tenantID     uuid.UUID
		environment  domain.Environment
		hasWorkspace bool
	)
	if workspace != nil {
		hasWorkspace = true
		workspaceID = workspace.ID
		tenantID = workspace.TenantID
		environment = workspace.Environment
	}

	injCtx := port.NewInjectorContext(nil, staticCatalogInjectorRef(workspace, templateType), nil, tenantID, workspaceID, environment, templateType)
	resolved, err := m.resolveCodeInjectors(ctx, injCtx, func(inj port.CodeInjector) bool {
		meta, ok := injectorCatalog(inj)
		return ok && meta.Static
	})
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	defs := make([]*domain.InjectorDefinition, 0, len(staticInjectors))
	fieldsByDefinition := make(map[uuid.UUID][]*domain.InjectorField, len(staticInjectors))
	for _, inj := range staticInjectors {
		meta, _ := injectorCatalog(inj)
		defID := syntheticCodeInjectorDefinitionID(meta.Code)
		description := optionalString(meta.Description)
		def := &domain.InjectorDefinition{
			ID:          defID,
			Name:        meta.Code,
			Description: description,
			Source:      "code",
			Static:      true,
			OwnerScope:  "global",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		defs = append(defs, def)

		fieldDefs := make([]*domain.InjectorField, 0, len(meta.Fields))
		for position, field := range meta.Fields {
			var value any
			if hasWorkspace {
				value = resolved[meta.Code][field.Name]
			}
			fieldDefs = append(fieldDefs, &domain.InjectorField{
				ID:                   syntheticCodeInjectorFieldID(meta.Code, field.Name),
				InjectorDefinitionID: defID,
				FieldName:            field.Name,
				FieldType:            field.Type,
				Description:          optionalString(field.Description),
				Position:             position,
				DefaultValue:         value,
				AllowOverwrite:       false,
			})
		}
		fieldsByDefinition[defID] = fieldDefs
	}

	return defs, fieldsByDefinition, nil
}

// ResolveStaticPreview resolves only fields that should be materialized in the
// builder preview: DB locked fields and static code injectors.
func (m *InjectorMerger) ResolveStaticPreview(ctx context.Context, workspaceID *uuid.UUID, injCtx *port.InjectorContext) (MergedInjectors, error) {
	result := make(MergedInjectors)

	var (
		defaults   MergedInjectors
		fieldRules map[string]map[string]injectorFieldRule
		err        error
	)
	if workspaceID == nil {
		defaults, fieldRules, err = m.resolveDBForScopes(ctx, []uuid.NullUUID{{}})
	} else {
		defaults, fieldRules, err = m.resolveDB(ctx, *workspaceID)
	}
	if err != nil {
		return nil, err
	}

	for injectorName, rules := range fieldRules {
		for fieldName, rule := range rules {
			if rule.AllowOverwrite {
				continue
			}
			if result[injectorName] == nil {
				result[injectorName] = make(map[string]any)
			}
			result[injectorName][fieldName] = defaults[injectorName][fieldName]
		}
	}

	staticValues, err := m.resolveCodeInjectors(ctx, injCtx, func(inj port.CodeInjector) bool {
		meta, ok := injectorCatalog(inj)
		return ok && meta.Static
	})
	if err != nil {
		return nil, err
	}
	for name, fields := range staticValues {
		if result[name] == nil {
			result[name] = make(map[string]any, len(fields))
		}
		for fieldName, value := range fields {
			result[name][fieldName] = value
		}
	}

	return result, nil
}

// Resolve returns only UI-defined workspace injectors with their default values.
func (m *InjectorMerger) Resolve(ctx context.Context, workspaceID uuid.UUID) (MergedInjectors, error) {
	defaults, _, err := m.resolveDB(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return defaults, nil
}

// ResolveGlobalWithContext resolves only global DB injectors and applies
// request-time overrides for overwriteable fields.
func (m *InjectorMerger) ResolveGlobalWithContext(ctx context.Context, injCtx *port.InjectorContext) (MergedInjectors, error) {
	defaults, fieldRules, err := m.resolveDBForScopes(ctx, []uuid.NullUUID{{}})
	if err != nil {
		return nil, err
	}

	injCtx.MergeDBInjectors(defaults)

	if m.codeInitFunc != nil {
		initData, initErr := m.codeInitFunc(ctx, injCtx)
		if initErr != nil {
			return nil, fmt.Errorf("code init func: %w", initErr)
		}
		injCtx.SetInitData(initData)
	}

	codeValues, err := m.resolveCodeInjectors(ctx, injCtx, nil)
	if err != nil {
		return nil, err
	}

	result := cloneMergedInjectors(defaults)
	requestValues := injCtx.RequestInjectors()
	applyGlobalFieldRules(result, fieldRules, requestValues, codeValues)
	mergeCodeOnlyFields(result, fieldRules, codeValues)

	injCtx.MergeDBInjectors(result)
	return result, nil
}

// ResolveWithContext resolves workspace defaults, optional code injectors, and
// request-body injector overrides into a single runtime map.
//
//nolint:gocognit // precedence orchestration across defaults, request data, and code injectors
func (m *InjectorMerger) ResolveWithContext(ctx context.Context, workspaceID uuid.UUID, injCtx *port.InjectorContext) (MergedInjectors, error) {
	defaults, fieldRules, err := m.resolveDB(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	injCtx.MergeDBInjectors(defaults)

	if m.codeInitFunc != nil {
		initData, initErr := m.codeInitFunc(ctx, injCtx)
		if initErr != nil {
			return nil, fmt.Errorf("code init func: %w", initErr)
		}
		injCtx.SetInitData(initData)
	}

	codeValues, err := m.resolveCodeInjectors(ctx, injCtx, nil)
	if err != nil {
		return nil, err
	}

	result := cloneMergedInjectors(defaults)
	requestValues := injCtx.RequestInjectors()

	for injectorName, rules := range fieldRules {
		if result[injectorName] == nil {
			result[injectorName] = make(map[string]any, len(rules))
		}

		for fieldName, rule := range rules {
			if !rule.AllowOverwrite {
				continue
			}

			if value, ok := getFieldValue(requestValues, injectorName, fieldName); ok {
				result[injectorName][fieldName] = value
				continue
			}
			if value, ok := getFieldValue(codeValues, injectorName, fieldName); ok {
				result[injectorName][fieldName] = value
				continue
			}

			result[injectorName][fieldName] = rule.DefaultValue
		}
	}

	for name, fields := range codeValues {
		if result[name] == nil {
			result[name] = cloneFields(fields)
			continue
		}

		if _, exists := fieldRules[name]; exists {
			for fieldName, value := range fields {
				if _, defined := fieldRules[name][fieldName]; !defined {
					result[name][fieldName] = value
				}
			}
		}
	}

	injCtx.MergeDBInjectors(result)
	return result, nil
}

func (m *InjectorMerger) resolveDB(ctx context.Context, workspaceID uuid.UUID) (MergedInjectors, map[string]map[string]injectorFieldRule, error) {
	chain, err := m.chainResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	return m.resolveDBForScopes(ctx, chain.Scopes)
}

func (m *InjectorMerger) resolveDBForScopes(ctx context.Context, scopes []uuid.NullUUID) (MergedInjectors, map[string]map[string]injectorFieldRule, error) {
	defs, err := m.store.ListDefinitionsInChain(ctx, scopes)
	if err != nil {
		return nil, nil, err
	}

	type dedupEntry struct {
		def   *domain.InjectorDefinition
		index int
	}

	bestByName := make(map[string]dedupEntry, len(defs))
	for _, def := range defs {
		scopeIdx := scopeIndex(def.WorkspaceID, scopes)
		existing, found := bestByName[def.Name]
		if !found || scopeIdx < existing.index {
			bestByName[def.Name] = dedupEntry{def: def, index: scopeIdx}
		}
	}

	defIDs := make([]uuid.UUID, 0, len(bestByName))
	for _, entry := range bestByName {
		defIDs = append(defIDs, entry.def.ID)
	}

	allFields, err := m.store.GetAllFieldsByDefinitions(ctx, defIDs)
	if err != nil {
		return nil, nil, err
	}
	allValues, err := m.store.GetAllValuesByDefinitions(ctx, defIDs, scopes)
	if err != nil {
		return nil, nil, err
	}

	valIdx := buildValueIndex(allValues)
	result := make(MergedInjectors, len(bestByName))
	fieldRules := make(map[string]map[string]injectorFieldRule, len(bestByName))

	for name, entry := range bestByName {
		def := entry.def
		fields := allFields[def.ID]
		result[def.Name] = make(map[string]any, len(fields))
		fieldRules[def.Name] = make(map[string]injectorFieldRule, len(fields))

		for _, field := range fields {
			resolved := resolveFieldValueIndexed(field.FieldName, scopes, valIdx[def.ID])
			if resolved == nil {
				resolved = field.DefaultValue
			}
			result[name][field.FieldName] = resolved
			fieldRules[def.Name][field.FieldName] = injectorFieldRule{
				DefaultValue:   field.DefaultValue,
				AllowOverwrite: field.AllowOverwrite,
			}
		}
	}

	return result, fieldRules, nil
}

// resolveCodeInjectors runs code injectors respecting dependency order.
func (m *InjectorMerger) resolveCodeInjectors(ctx context.Context, injCtx *port.InjectorContext, filter func(port.CodeInjector) bool) (MergedInjectors, error) {
	if len(m.codeInjectors) == 0 {
		return nil, nil
	}

	byCode := make(map[string]port.CodeInjector, len(m.codeInjectors))
	for _, inj := range m.codeInjectors {
		if filter != nil && !filter(inj) {
			continue
		}
		byCode[inj.Code()] = inj
	}
	if len(byCode) == 0 {
		return nil, nil
	}

	resolved := make(MergedInjectors, len(byCode))
	visited := make(map[string]bool)

	var resolveOne func(code string) error
	resolveOne = func(code string) error {
		if visited[code] {
			return nil
		}
		visited[code] = true

		inj, ok := byCode[code]
		if !ok {
			return nil
		}

		_, deps := inj.Resolve()
		for _, dep := range deps {
			if err := resolveOne(dep); err != nil {
				return err
			}
		}

		timeout := inj.Timeout()
		if timeout == 0 {
			timeout = defaultCodeInjectorTimeout
		}

		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		fields, err := m.resolveInjectorFields(execCtx, injCtx, inj)
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

	for code := range byCode {
		if err := resolveOne(code); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

func (m *InjectorMerger) resolveInjectorFields(ctx context.Context, injCtx *port.InjectorContext, inj port.CodeInjector) (map[string]any, error) {
	meta, ok := injectorCatalog(inj)
	if ok && meta.Static && meta.TTL > 0 && m.cache != nil {
		cacheKey := staticCodeInjectorCacheKey(injCtx.WorkspaceID(), injCtx.TemplateType(), inj.Code())
		if data, err := m.cache.Get(ctx, cacheKey); err == nil {
			var cached map[string]any
			if err := json.Unmarshal(data, &cached); err == nil {
				return cached, nil
			}
		}

		resolveFn, _ := inj.Resolve()
		fields, err := resolveFn(ctx, injCtx)
		if err != nil {
			return nil, err
		}
		if data, err := json.Marshal(fields); err == nil {
			_ = m.cache.Set(ctx, cacheKey, data, meta.TTL)
		}
		return fields, nil
	}

	resolveFn, _ := inj.Resolve()
	return resolveFn(ctx, injCtx)
}

func (m *InjectorMerger) staticCatalogInjectors() []port.CodeInjector {
	result := make([]port.CodeInjector, 0, len(m.codeInjectors))
	for _, inj := range m.codeInjectors {
		meta, ok := injectorCatalog(inj)
		if !ok || !meta.Static {
			continue
		}
		result = append(result, inj)
	}
	return result
}

func injectorCatalog(inj port.CodeInjector) (port.InjectorCatalog, bool) {
	catalogInj, ok := inj.(port.CatalogCodeInjector)
	if !ok {
		return port.InjectorCatalog{}, false
	}
	return catalogInj.Catalog(), true
}

func staticCodeInjectorCacheKey(workspaceID uuid.UUID, templateType string, code string) string {
	if templateType == "" {
		templateType = "_"
	}
	return fmt.Sprintf("code_injector_static:%s:%s:%s", workspaceID.String(), templateType, code)
}

func staticCatalogInjectorRef(workspace *domain.Workspace, templateType string) string {
	if workspace == nil || strings.TrimSpace(workspace.Code) == "" {
		if templateType == "" {
			return ""
		}
		return "global::" + templateType
	}

	if templateType == "" {
		templateType = "catalog"
	}
	return "catalog:" + workspace.Code + ":" + templateType
}

func syntheticCodeInjectorDefinitionID(code string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("code-injector:def:"+code))
}

func syntheticCodeInjectorFieldID(code, field string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("code-injector:field:"+code+":"+field))
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func getFieldValue(values map[string]map[string]any, injectorName, fieldName string) (any, bool) {
	fields, ok := values[injectorName]
	if !ok {
		return nil, false
	}
	value, ok := fields[fieldName]
	return value, ok
}

func cloneMergedInjectors(injectors MergedInjectors) MergedInjectors {
	cloned := make(MergedInjectors, len(injectors))
	for name, fields := range injectors {
		cloned[name] = cloneFields(fields)
	}
	return cloned
}

func cloneFields(fields map[string]any) map[string]any {
	cloned := make(map[string]any, len(fields))
	for fieldName, value := range fields {
		cloned[fieldName] = value
	}
	return cloned
}

func applyGlobalFieldRules(
	result MergedInjectors,
	fieldRules map[string]map[string]injectorFieldRule,
	requestValues MergedInjectors,
	codeValues MergedInjectors,
) {
	for injectorName, rules := range fieldRules {
		if result[injectorName] == nil {
			result[injectorName] = make(map[string]any, len(rules))
		}
		for fieldName, rule := range rules {
			result[injectorName][fieldName] = resolveGlobalFieldValue(rule, injectorName, fieldName, requestValues, codeValues)
		}
	}
}

func resolveGlobalFieldValue(
	rule injectorFieldRule,
	injectorName string,
	fieldName string,
	requestValues MergedInjectors,
	codeValues MergedInjectors,
) any {
	if !rule.AllowOverwrite {
		return rule.DefaultValue
	}
	if value, ok := getFieldValue(requestValues, injectorName, fieldName); ok {
		return value
	}
	if value, ok := getFieldValue(codeValues, injectorName, fieldName); ok {
		return value
	}
	return rule.DefaultValue
}

func mergeCodeOnlyFields(result MergedInjectors, fieldRules map[string]map[string]injectorFieldRule, codeValues MergedInjectors) {
	for name, fields := range codeValues {
		if result[name] == nil {
			result[name] = cloneFields(fields)
			continue
		}
		rules, exists := fieldRules[name]
		if !exists {
			continue
		}
		for fieldName, value := range fields {
			if _, defined := rules[fieldName]; !defined {
				result[name][fieldName] = value
			}
		}
	}
}

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
			idx[defID][v.FieldName][scopeKey(v.WorkspaceID)] = v
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
		if v, found := scopeValues[scopeKeyFromNullUUID(scope)]; found {
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
	for i, scope := range scopes {
		if matchScope(wsID, scope) {
			return i
		}
	}
	return len(scopes)
}
