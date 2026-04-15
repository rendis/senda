package sdk

import (
	"github.com/google/uuid"
	"github.com/rendis/senda/internal/port"
)

// InjectorContext is the read-only context passed to code injectors and
// the init function during send resolution.
type InjectorContext struct {
	inner *port.InjectorContext
}

// NewInjectorContext creates a new InjectorContext. Re-exported for tests
// and advanced usage; normally the engine creates this internally.
func NewInjectorContext(
	headers map[string]string,
	ref string,
	variables map[string]any,
	tenantID, workspaceID uuid.UUID,
	environment Environment,
	templateType string,
) *InjectorContext {
	return wrapInjectorContext(port.NewInjectorContext(headers, ref, variables, tenantID, workspaceID, toDomainEnvironment(environment), templateType))
}

func wrapInjectorContext(inner *port.InjectorContext) *InjectorContext {
	if inner == nil {
		return nil
	}
	return &InjectorContext{inner: inner}
}

func (c *InjectorContext) Header(key string) string {
	if c == nil || c.inner == nil {
		return ""
	}
	return c.inner.Header(key)
}

func (c *InjectorContext) Headers() map[string]string {
	if c == nil || c.inner == nil {
		return map[string]string{}
	}
	return c.inner.Headers()
}

func (c *InjectorContext) Ref() string {
	if c == nil || c.inner == nil {
		return ""
	}
	return c.inner.Ref()
}

func (c *InjectorContext) Variables() map[string]any {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Variables()
}

func (c *InjectorContext) RequestInjectors() map[string]map[string]any {
	if c == nil || c.inner == nil {
		return map[string]map[string]any{}
	}
	return c.inner.RequestInjectors()
}

func (c *InjectorContext) InitData() any {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.InitData()
}

func (c *InjectorContext) TenantID() uuid.UUID {
	if c == nil || c.inner == nil {
		return uuid.Nil
	}
	return c.inner.TenantID()
}

func (c *InjectorContext) WorkspaceID() uuid.UUID {
	if c == nil || c.inner == nil {
		return uuid.Nil
	}
	return c.inner.WorkspaceID()
}

func (c *InjectorContext) Environment() Environment {
	if c == nil || c.inner == nil {
		return ""
	}
	return fromDomainEnvironment(c.inner.Environment())
}

func (c *InjectorContext) TemplateType() string {
	if c == nil || c.inner == nil {
		return ""
	}
	return c.inner.TemplateType()
}

func (c *InjectorContext) GetResolved(code string) (map[string]any, bool) {
	if c == nil || c.inner == nil {
		return nil, false
	}
	return c.inner.GetResolved(code)
}

func (c *InjectorContext) SetInitData(data any) {
	if c == nil || c.inner == nil {
		return
	}
	c.inner.SetInitData(data)
}

func (c *InjectorContext) SetResolved(code string, fields map[string]any) {
	if c == nil || c.inner == nil {
		return
	}
	c.inner.SetResolved(code, fields)
}

func (c *InjectorContext) SetRequestInjectors(injectors map[string]map[string]any) {
	if c == nil || c.inner == nil {
		return
	}
	c.inner.SetRequestInjectors(injectors)
}

func (c *InjectorContext) MergeDBInjectors(dbValues map[string]map[string]any) {
	if c == nil || c.inner == nil {
		return
	}
	c.inner.MergeDBInjectors(dbValues)
}

func (c *InjectorContext) AllResolved() map[string]map[string]any {
	if c == nil || c.inner == nil {
		return map[string]map[string]any{}
	}
	return c.inner.AllResolved()
}
