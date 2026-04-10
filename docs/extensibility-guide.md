
# Extensibility Guide

Senda can be embedded as a Go library via `github.com/rendis/senda/sdk`.

This guide covers the public seams embedders can rely on without forking Senda.

```bash
go get github.com/rendis/senda
```

## Extension points summary

| Method | Purpose | Multiplicity | Runtime |
| --- | --- | ---: | --- |
| `RegisterInjector(...)` | Add a code injector | many | send flow |
| `SetInitFunc(...)` | Add per-request init | one | send flow |
| `RegisterExternalAuthMethod(...)` | Add external auth method | many | external surface |
| `RegisterExternalWorkspaceResolver(...)` | Add external workspace resolver | many | external surface |
| `OnStart(...)` | Startup hook | many | process lifecycle |
| `OnShutdown(...)` | Shutdown hook | many | process lifecycle |

## Basic engine bootstrap

```go
engine := sdk.NewWithConfig("settings/config.yaml")

engine.SetInitFunc(MyInit())
engine.RegisterInjector(&StudentInjector{})
engine.RegisterExternalAuthMethod(&PortalAuthMethod{})
engine.RegisterExternalWorkspaceResolver(&PortalWorkspaceResolver{})
engine.OnStart(func(ctx context.Context) error { return connectMongo(ctx) })
engine.OnShutdown(func(ctx context.Context) error { return closeMongo(ctx) })

if err := engine.Run(); err != nil {
    return err
}
```

## Injectors

Injectors provide code-defined template fields.

```go
type Injector interface {
    Code() string
    Resolve() (sdk.ResolveFunc, []string)
    IsCritical() bool
    Timeout() time.Duration
}
```

### When injectors run

Injectors run inside the send flow after workspace/template resolution and after the init function.

### What they can read

Use `*sdk.InjectorContext` to access:

- request headers
- send variables
- request injector overrides
- init data
- tenant/workspace IDs
- environment
- already resolved injector values

### Environment access

Use:

```go
env := injCtx.Environment()
```

Do not invent your own environment flag.

## Init function

Init runs once per send request.

```go
type InitFunc func(ctx context.Context, injCtx *sdk.InjectorContext) (any, error)
```

Use it when multiple injectors need the same request-scoped load.

The result is available to injectors through `injCtx.InitData()`.

## External auth methods

External integrations use custom auth, not OIDC.

```go
type ExternalAuthMethod interface {
    Name() string
    Description() string
    Authenticate(ctx context.Context, req *sdk.ExternalIntegrationRequest) (*sdk.ExternalAuthResult, error)
}
```

Use an external auth method to:

- validate tokens or headers from an iframe/portal
- normalize capability flags
- attach caller context

The auth method does **not** choose the final workspace.

## External workspace resolvers

Workspace mapping is a separate seam.

```go
type ExternalWorkspaceResolver interface {
    Name() string
    Description() string
    ResolveWorkspace(ctx context.Context, req *sdk.ExternalIntegrationRequest, auth *sdk.ExternalAuthResult) (*sdk.ExternalWorkspaceResolution, error)
}
```

Use it to:

- map an authenticated request to a workspace
- force read-only fallback
- centralize external workspace-routing logic

## External request context

`ExternalIntegrationRequest` includes:

- `ProfileSlug`
- `Environment`
- `TenantCode`
- `WorkspaceCodes`
- `Token`
- `Headers`
- `QueryParams`
- `Path`
- `Method`

The environment comes from the validated `X-Senda-Environment` header.

## Lifecycle hooks

### `OnStart(...)`

Runs after bootstrap and before the server starts serving.

Use it for:

- connecting external clients
- warming resources needed by injectors/auth/resolvers

### `OnShutdown(...)`

Runs during shutdown in reverse registration order.

Use it for:

- closing external clients
- flushing buffers
- stopping long-lived resources

## Send flow

1. request enters the send pipeline
2. tenant, workspace, and template type resolve
3. `InjectorContext` is created
4. DB injectors resolve and seed context
5. `InitFunc` runs once
6. code injectors run in dependency order
7. values merge and render proceeds
8. provider dispatch / queueing continues

## External integration flow

1. profile loads by slug
2. required headers and `X-Senda-Environment` validate
3. selected auth method authenticates
4. selected workspace resolver returns the effective workspace or read-only fallback
5. capability guards apply per route

## Design guidance for embedders

- Centralize registration in `extensions.Register(engine)`.
- Keep auth and workspace resolution separate.
- Use `InitFunc` to avoid duplicated per-request loads.
- Use `injCtx.Environment()` / `req.Environment` as the environment source of truth.
- Do not try to replace Senda internals like providers, queue, crypto, or middleware through the SDK. Those remain configuration-driven internals.
