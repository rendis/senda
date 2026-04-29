
# SDK Extension Points

This reference explains how to embed Senda as a Go library and how each extension point behaves.

## Engine lifecycle

Typical bootstrap:

```go
engine := sdk.NewWithConfig("settings/config.yaml")

engine.RegisterInjector(sdk.InjectorRegistration{
    Code:    "student",
    Name:    "Student",
    Resolve: resolveStudent,            // func(ctx, *sdk.InjectorContext) (map[string]any, error)
    Fields: []sdk.InjectorFieldSpec{
        {Name: "first_name", Type: sdk.FieldTypeText},
        {Name: "grade",      Type: sdk.FieldTypeNumber},
    },
    Critical: true,
    Timeout:  2 * time.Second,
})
engine.SetInitFunc(myInit)
engine.RegisterExternalAuthMethod(&PortalAuthMethod{})           // interface impl
engine.RegisterExternalWorkspaceResolver(&PortalWorkspaceResolver{}) // interface impl
engine.OnStart(func(ctx context.Context) error { return nil })
engine.OnShutdown(func(ctx context.Context) error { return nil })

if err := engine.Run(); err != nil {
    panic(err)
}
```

`Run()` performs, in order:

1. load config (`SENDA_CONFIG` overrides constructor path)
2. setup logger + metrics
3. bootstrap app internals
4. run `OnStart` hooks in registration order
5. start River workers in a **background goroutine**
6. start HTTP server (runs concurrently with River; both are alive once `Run()` returns its blocking loop)
7. on shutdown, run `OnShutdown` hooks in reverse order

Steps 5 and 6 are not strictly sequential: River is dispatched with `go ...`
and the HTTP server starts immediately afterwards. Don't assume River is
fully drained or fully connected before the HTTP listener accepts requests.

## Registration points

### `RegisterInjector(...)`

Registers a code injector. The public contract is a struct value, not an
interface to implement.

```go
type ResolveFunc func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error)

type InjectorFieldSpec struct {
    Name        string
    Type        sdk.InjectorFieldType // text | number | bool | img | url | html
    Description string
}

type InjectorRegistration struct {
    Code         string                  // matches DB `injector_definitions.name`
    Name         string
    Description  string
    Static       bool                    // catalogable; preview can substitute static values
    TTL          time.Duration           // cache TTL for resolved values, 0 = no cache
    Fields       []InjectorFieldSpec     // declared fields (used by static preview, catalog UI)
    Resolve      ResolveFunc             // runtime resolver — required when Static is false
    Dependencies []string                // other injector codes this one depends on (topo order)
    Critical     bool                    // if true, send fails when this injector errors
    Timeout      time.Duration           // per-resolve timeout
}

func (e *Engine) RegisterInjector(reg InjectorRegistration) *Engine
```

Use it when your repo needs business-specific template fields from code.

Behavior:

- many allowed; `engine.RegisterInjector(InjectorRegistration{...})`
- duplicate `Code` values: last registration wins over earlier code injectors
- code and DB injectors share the same `injector.<code>.<field>` namespace.
  Precedence is per-field and gated by `allow_overwrite` on the DB field:
  when `false` the DB chain wins (workspace `injector_values` > `_system` >
  default); when `true` it is request body > code injector > field default
  and the DB `injector_values` rows are skipped. See `injectors.md`.
- a code injector can contribute fields the DB does not declare; those
  flow through unchanged
- `Code` is the injector name. There is no separate "code" column in the
  DB; matching is purely by name
- `Static = true` requires a non-empty `Fields` list (panics otherwise) and
  registers a catalog entry visible in the UI's static-preview substitution
- `Resolve` is always required — `RegisterInjector` panics if `Resolve == nil`
  even for static registrations. For purely declarative catalogs, supply a
  trivial resolver that returns each field's static value (or an empty map)
- `Code` is also required (panics if empty)

### `SetInitFunc(...)`

Registers a per-request init function.

```go
type InitFunc func(ctx context.Context, injCtx *sdk.InjectorContext) (any, error)
```

Use it when multiple injectors need the same request-scoped data.

Behavior:

- only one active init func; last call wins
- runs once per send request
- runs after workspace/template resolution and before code injectors
- return value is stored in `injCtx.InitData()`

### `RegisterExternalAuthMethod(...)`

Registers a custom auth method for external integrations.

```go
type ExternalAuthMethod interface {
    Name() string
    Description() string
    Authenticate(ctx context.Context, req *sdk.ExternalIntegrationRequest) (*sdk.ExternalAuthResult, error)
}
```

Use it when an external portal, iframe, or embedded builder needs its own auth scheme.

Behavior:

- many allowed
- the active one is selected by profile config (`auth_method_name`)
- should validate the incoming request and return normalized permissions/context
- should not resolve the workspace; that belongs to the workspace resolver

### `RegisterExternalWorkspaceResolver(...)`

Registers a workspace resolver for external integrations.

```go
type WorkspaceFilter interface {
    // Returns a dense map: every requested code is a key; value is true only
    // when the workspace exists and is active for the tenant in the current
    // environment.
    Exists(ctx context.Context, codes []string) (map[string]bool, error)
}

type ExternalWorkspaceResolver interface {
    Name() string
    Description() string
    ResolveWorkspace(
        ctx context.Context,
        req *sdk.ExternalIntegrationRequest,
        auth *sdk.ExternalAuthResult,
        filter sdk.WorkspaceFilter,
    ) (*sdk.ExternalWorkspaceResolution, error)
}
```

The `filter` argument is constructed per-request by the middleware; use it
to validate that a candidate workspace code is actually active for the
tenant + environment before returning it. Implementations that omit the
filter parameter will not satisfy the interface.

Use it when external access needs tenant/workspace mapping rules that differ from the normal management/data-plane model.

Behavior:

- many allowed
- the active one is selected by profile config (`resolver_name`)
- runs after auth succeeds
- may return a direct workspace or a read-only fallback

### `OnStart(...)` and `OnShutdown(...)`

Lifecycle hooks for external resources.

Use them for:

- opening database/API clients external to Senda
- loading caches or clients needed by your injectors/auth/resolvers
- closing those resources cleanly

Do not use them for request-scoped behavior.

## InjectorContext

`InjectorContext` is the read-only runtime context for init/injectors.

Important methods:

| Method | Purpose |
|---|---|
| `Header(key)` / `Headers()` | request headers |
| `Ref()` | `tenant:workspace:templateType` |
| `Variables()` | caller-provided send variables |
| `RequestInjectors()` | request-body injector overrides |
| `InitData()` | output from `InitFunc` |
| `TenantID()` | resolved tenant UUID |
| `WorkspaceID()` | resolved workspace UUID |
| `Environment()` | `prod` or `test` |
| `TemplateType()` | resolved template type slug |
| `GetResolved(code)` | already-resolved injector values |
| `AllResolved()` | merged injector values |

Important behavior:

- `Environment()` is the runtime source of truth for init/injectors.
- `RequestInjectors()` exposes request-level runtime overrides.
- DB injectors are seeded before code injectors run.

## External request contracts

### `ExternalIntegrationRequest`

Carries the request context for external auth and workspace resolution.

Fields you should rely on:

- `ProfileSlug`
- `Environment`
- `TenantCode`
- `WorkspaceCodes`
- `Token`
- `Headers`
- `QueryParams`
- `Path`
- `Method`

The environment is already normalized from `X-Senda-Environment`.

### `ExternalAuthResult`

Returned by the auth method.

Use it to return:

- capability flags (`ListTemplates`, `ViewVersions`, `EditVersions`, etc.)
- normalized auth context via `Context map[string]any`

### `ExternalWorkspaceResolution`

Returned by the workspace resolver.

Fields:

- `WorkspaceCode`
- `ReadOnly`

Rules:

- when `ReadOnly=true`, the external middleware forces `_system` as the effective workspace
- external callers must not mutate in read-only fallback mode

## Example registration from a consumer repo

```go
func Register(engine *sdk.Engine) {
    engine.SetInitFunc(MyInit())

    engine.RegisterInjector(sdk.InjectorRegistration{
        Code:    "student",
        Name:    "Student",
        Resolve: ResolveStudent,
        Fields: []sdk.InjectorFieldSpec{
            {Name: "first_name", Type: sdk.FieldTypeText},
            {Name: "grade",      Type: sdk.FieldTypeNumber},
        },
        Dependencies: []string{"institution"},
        Critical:     true,
        Timeout:      2 * time.Second,
    })

    engine.RegisterInjector(sdk.InjectorRegistration{
        Code:    "summary",
        Name:    "Summary",
        Resolve: ResolveSummary,
    })

    engine.RegisterExternalAuthMethod(&PortalAuthMethod{})           // interface impl
    engine.RegisterExternalWorkspaceResolver(&PortalWorkspaceResolver{}) // interface impl

    engine.OnStart(func(ctx context.Context) error {
        return connectExternalClients(ctx)
    })

    engine.OnShutdown(func(ctx context.Context) error {
        return closeExternalClients(ctx)
    })
}
```

`ExternalAuthMethod` and `ExternalWorkspaceResolver` are still **interfaces**;
implement them on a struct and pass a pointer. Only `RegisterInjector`
switched to a struct value contract.

## Design guidance for embedders

- Put registration in a single `extensions.Register(engine)` function.
- Keep auth methods and workspace resolvers separate: auth authenticates, resolver maps.
- Use `InitFunc` for shared request-scoped loads instead of repeating the same call in multiple injectors.
- Use `Environment()` rather than ad-hoc flags or workspace naming conventions.
- Keep provider adapters, queues, crypto, auth middleware, and internal stores inside Senda config; do not try to re-register those through the SDK.

## Cross-references

- For the per-field precedence between request body, code injector, DB scope,
  and `default_value`: `injectors.md`.
- For the external-surface capabilities and profile config:
  `external-integration.md`.
- For `Environment()` semantics and where it comes from at each surface:
  `platform-overview.md`.
