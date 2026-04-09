package port

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CodeInjector is the interface for user-provided code injectors
// that resolve dynamic values at send time. Resolved fields merge
// with DB injectors into the template variable namespace:
//
//	{{ injector.<Code()>.<fieldName> }}
type CodeInjector interface {
	// Code returns the unique injector name.
	Code() string

	// Resolve returns the resolution function and optional dependency codes
	// (names of other injectors that must resolve first).
	Resolve() (CodeResolveFunc, []string)

	// IsCritical returns true if a failure should abort the send.
	IsCritical() bool

	// Timeout returns max duration for resolution. Zero = default (30s).
	Timeout() time.Duration
}

// CodeResolveFunc executes injector logic and returns field values.
// The map keys become field names: {{ injector.<code>.<key> }}.
type CodeResolveFunc func(ctx context.Context, injCtx *InjectorContext) (map[string]any, error)

// CodeInitFunc runs once per send request before code injectors.
// The returned value is stored in InjectorContext.InitData().
type CodeInitFunc func(ctx context.Context, injCtx *InjectorContext) (any, error)

// InjectorContext is the read-only context available to code injectors
// and the init function during send resolution.
type InjectorContext struct {
	mu sync.RWMutex

	// From the HTTP request.
	headers map[string]string

	// From the send request.
	ref              string
	variables        map[string]any
	requestInjectors map[string]map[string]any

	// From the user's CodeInitFunc.
	initData any

	// Resolved by Senda's resolution engine.
	tenantID     uuid.UUID
	workspaceID  uuid.UUID
	templateType string

	// Merged values from DB + code injectors already resolved.
	resolved map[string]map[string]any
}

// NewInjectorContext creates a new InjectorContext for a send request.
func NewInjectorContext(
	headers map[string]string,
	ref string,
	variables map[string]any,
	tenantID, workspaceID uuid.UUID,
	templateType string,
) *InjectorContext {
	return &InjectorContext{
		headers:      headers,
		ref:          ref,
		variables:    variables,
		tenantID:     tenantID,
		workspaceID:  workspaceID,
		templateType: templateType,
		resolved:     make(map[string]map[string]any),
	}
}

// Header returns a request header value by key.
func (c *InjectorContext) Header(key string) string {
	if c.headers == nil {
		return ""
	}
	return c.headers[key]
}

// Headers returns a copy of all request headers.
func (c *InjectorContext) Headers() map[string]string {
	cp := make(map[string]string, len(c.headers))
	for k, v := range c.headers {
		cp[k] = v
	}
	return cp
}

// Ref returns the send addressing ref (e.g., "tenant:workspace:templateType").
func (c *InjectorContext) Ref() string { return c.ref }

// Variables returns the caller-provided event variables.
func (c *InjectorContext) Variables() map[string]any { return c.variables }

// RequestInjectors returns a copy of request-body injector overrides.
func (c *InjectorContext) RequestInjectors() map[string]map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]map[string]any, len(c.requestInjectors))
	for code, fields := range c.requestInjectors {
		fieldCopy := make(map[string]any, len(fields))
		for field, value := range fields {
			fieldCopy[field] = value
		}
		cp[code] = fieldCopy
	}
	return cp
}

// InitData returns the value produced by CodeInitFunc.
func (c *InjectorContext) InitData() any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initData
}

// TenantID returns the resolved tenant UUID.
func (c *InjectorContext) TenantID() uuid.UUID { return c.tenantID }

// WorkspaceID returns the resolved workspace UUID.
func (c *InjectorContext) WorkspaceID() uuid.UUID { return c.workspaceID }

// TemplateType returns the resolved template type slug.
func (c *InjectorContext) TemplateType() string { return c.templateType }

// GetResolved returns the merged field values for a named injector.
func (c *InjectorContext) GetResolved(code string) (map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.resolved[code]
	return v, ok
}

// SetInitData stores the init function result. Internal use only.
func (c *InjectorContext) SetInitData(data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initData = data
}

// SetResolved stores resolved fields for a named injector. Internal use only.
func (c *InjectorContext) SetResolved(code string, fields map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resolved[code] = fields
}

// SetRequestInjectors stores request-body injector overrides. Internal use only.
func (c *InjectorContext) SetRequestInjectors(injectors map[string]map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestInjectors = make(map[string]map[string]any, len(injectors))
	for code, fields := range injectors {
		fieldCopy := make(map[string]any, len(fields))
		for field, value := range fields {
			fieldCopy[field] = value
		}
		c.requestInjectors[code] = fieldCopy
	}
}

// MergeDBInjectors seeds the context with all DB-resolved injector values.
func (c *InjectorContext) MergeDBInjectors(dbValues map[string]map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range dbValues {
		c.resolved[k] = v
	}
}

// AllResolved returns all resolved injector values (DB + code) as MergedInjectors.
func (c *InjectorContext) AllResolved() map[string]map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]map[string]any, len(c.resolved))
	for k, v := range c.resolved {
		cp[k] = v
	}
	return cp
}
