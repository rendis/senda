
# SDK Extension Points

This reference explains how to embed Senda as a Go library and how each extension point behaves.

## Engine lifecycle

Typical bootstrap:

```go
engine := sdk.NewWithConfig("settings/config.yaml")

engine.RegisterInjector(&MyInjector{})
engine.SetInitFunc(myInit)
engine.RegisterExternalAuthMethod(&PortalAuthMethod{})
engine.RegisterExternalWorkspaceResolver(&PortalWorkspaceResolver{})
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
5. start River workers
6. start HTTP server
7. on shutdown, run `OnShutdown` hooks in reverse order

## Registration points

### `RegisterInjector(...)`

Registers a code injector implementing `sdk.Injector`.

```go
type Injector interface {
    Code() string
    Resolve() (sdk.ResolveFunc, []string)
    IsCritical() bool
    Timeout() time.Duration
}
```

Use it when your repo needs business-specific template fields from code.

Behavior:

- many allowed
- duplicate codes: last registered code injector wins over previous code injectors
- if a code injector and DB injector share the same code, the code injector wins for that injector namespace
- injector values merge into the same `injector.<code>.<field>` namespace used by DB injectors

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
type ExternalWorkspaceResolver interface {
    Name() string
    Description() string
    ResolveWorkspace(ctx context.Context, req *sdk.ExternalIntegrationRequest, auth *sdk.ExternalAuthResult) (*sdk.ExternalWorkspaceResolution, error)
}
```

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

    engine.RegisterInjector(&StudentInjector{})
    engine.RegisterInjector(&SummaryInjector{})

    engine.RegisterExternalAuthMethod(&PortalAuthMethod{})
    engine.RegisterExternalWorkspaceResolver(&PortalWorkspaceResolver{})

    engine.OnStart(func(ctx context.Context) error {
        return connectExternalClients(ctx)
    })

    engine.OnShutdown(func(ctx context.Context) error {
        return closeExternalClients(ctx)
    })
}
```

## Design guidance for embedders

- Put registration in a single `extensions.Register(engine)` function.
- Keep auth methods and workspace resolvers separate: auth authenticates, resolver maps.
- Use `InitFunc` for shared request-scoped loads instead of repeating the same call in multiple injectors.
- Use `Environment()` rather than ad-hoc flags or workspace naming conventions.
- Keep provider adapters, queues, crypto, auth middleware, and internal stores inside Senda config; do not try to re-register those through the SDK.
