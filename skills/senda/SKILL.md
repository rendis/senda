---
name: senda
description: >-
  Operate and extend the Senda email orchestration platform via MCP and the Go SDK.
  Use this skill when working with tenants, workspaces, templates, injectors, adapters,
  external integrations, API keys, or when embedding Senda as a library from another repo.
allowed-tools:
  - mcp__senda__*
---

# Senda

Operational skill for agents working with **Senda** as a product API **and** as an embeddable Go library.

This skill is intentionally self-contained. Do not depend on repo docs to understand the platform.
If you need deeper detail, load the bundled references from this skill package.

## What Senda is

Senda is a multi-tenant email orchestration platform with four operational surfaces:

1. **Management plane** — OIDC-authenticated CRUD for tenants, workspaces, templates, injectors, adapters, API keys, policies, and configuration.
2. **Data plane** — workspace-scoped sending and email query using raw API keys.
3. **External integration surface** — embeddable builder/editor APIs protected by custom auth methods and workspace resolvers.
4. **Go SDK / embedder mode** — register code injectors, per-request init, external auth methods, external workspace resolvers, and lifecycle hooks.

## Use this skill when

- operating Senda through MCP tools
- creating or managing tenants, workspaces, templates, template versions, adapters, injectors, API keys, webhooks, or members
- sending emails or querying delivery/event history
- working with workspace environments `prod` / `test`
- integrating an external builder or portal against Senda
- embedding Senda in another Go repo and registering extensions

## Mental model

### Scope hierarchy

Resolution is hierarchical:

`global -> tenant -> tenant _system workspace -> workspace`

Key rules:

- `_system` is a special workspace owned by a tenant and used for tenant-wide defaults and selective sharing.
- Regular workspaces can inherit from `_system` and global.
- Resolution is generally **workspace > _system > global**.
- Shared resources are not the same as owned resources. Track whether something is owned, inherited, or shared.

For the detailed inheritance and sharing rules, load:
- `references/resolution-and-inheritance.md`

### Environment model

Each logical workspace has two operational environments:

- `prod`
- `test`

The environment is not a label; it changes runtime behavior and routing.

Key rules:

- Do **not** put the environment into the send `ref`.
- Management routes can be shared or environment-scoped.
- Data-plane environment is determined by the API key prefix.
- External integration environment is provided explicitly through `X-Senda-Environment`.
- `test` has isolated runtime state and recipient safety controls.

For the detailed model, load:
- `references/environment-model.md`

## Golden rules

- **Use only the environment-aware raw API key prefixes** `senda_prod_` and `senda_test_`.
- **Never infer environment from workspace code or name.** Use the route/header/token source of truth.
- **Never put environment into `ref`.** `ref` stays `tenant:workspace:templateType`.
- **Always distinguish shared routes from environment-scoped routes.**
- **Treat `_system` as a special control workspace**, not a normal business workspace.
- **Use `X-Senda-Environment` for external integration requests.**
- **Runtime reset is test-only.** It is available only on the management environment-scoped test surface.
- **Test recipient policies are test-only.** Do not assume they exist in prod.

## MCP operating playbooks

### 1) Discover what exists

Start with endpoint discovery, then inspect or call only the relevant endpoints.

Typical sequence:

1. `senda_list_endpoints`
2. `senda_describe_endpoint`
3. `senda_call_endpoint`

### 2) Choose the right plane

- **Management plane** — humans/admin workflows, OIDC auth, CRUD, configuration, environment-scoped workspace operations.
- **Data plane** — send/query with `Authorization: Bearer senda_prod_...` or `senda_test_...`.
- **External integration surface** — `/api/v1/external/:profile_slug/...`, custom auth + workspace resolver, header `X-Senda-Environment` required.

### 3) Common MCP flows

Load this reference when you need ready-to-run workflows:
- `references/mcp-workflows.md`

That reference covers:

- onboarding
- tenant/workspace CRUD
- environment-scoped workspace operations
- API key creation and usage
- send + send batch + email query
- external bootstrap/session and builder flows
- config and external integration profile management

## SDK / embedder playbook

Use the Go SDK when another repo needs to run Senda and add business-specific behavior.

### Engine entry point

```go
engine := sdk.NewWithConfig("settings/config.yaml")
```

Then register extensions before `Run()`.

### Public registration points

| Method | What it registers | Multiplicity | When it runs |
|---|---|---:|---|
| `RegisterInjector(...)` | Code injector | many | during send resolution |
| `SetInitFunc(...)` | Per-request init function | one (last wins) | before code injectors |
| `RegisterExternalAuthMethod(...)` | External integration auth method | many | on external integration requests |
| `RegisterExternalWorkspaceResolver(...)` | External workspace resolver | many | after external auth succeeds |
| `OnStart(...)` | Startup hook | many | after bootstrap, before serving |
| `OnShutdown(...)` | Shutdown hook | many | during shutdown, reverse order |
| `Run()` | Starts Senda | one | blocks until shutdown |

### Operational guidance

- `InitFunc` is for per-request shared context.
- `Injector` is for code-provided template fields.
- `ExternalAuthMethod` validates an external request and returns normalized permissions/context.
- `ExternalWorkspaceResolver` maps that normalized request to a workspace or read-only fallback.
- `OnStart` / `OnShutdown` are for infrastructure lifecycle, not request logic.

Load this reference when implementing or reviewing embedder code:
- `references/sdk-extension-points.md`

## Real execution flows

### Send flow

1. request enters the send pipeline
2. tenant/workspace/template type are resolved
3. `InjectorContext` is created
4. DB injectors are resolved and seeded
5. `InitFunc` runs once
6. code injectors run in dependency order
7. values merge with precedence rules
8. template renders and dispatch continues

### External integration flow

1. external profile loads by slug
2. required headers and `X-Senda-Environment` are validated
3. registered auth method authenticates
4. registered workspace resolver returns effective workspace or read-only fallback
5. effective permissions are computed
6. the request proceeds against the external builder surface

### Resolution flow

1. workspace scope is checked first
2. tenant `_system` fallback is checked next
3. global fallback is checked last
4. shared/default/forked resources participate according to their own rules

For detailed flow explanations, load:
- `references/external-integration-flow.md`
- `references/resolution-and-inheritance.md`

## Public contracts you must understand

These are the primary public contracts for agents and embedders:

- `sdk.Engine`
- `sdk.Injector`
- `sdk.ResolveFunc`
- `sdk.InitFunc`
- `sdk.ExternalAuthMethod`
- `sdk.ExternalWorkspaceResolver`
- `sdk.ExternalIntegrationRequest`
- `sdk.ExternalAuthResult`
- `sdk.ExternalWorkspaceResolution`
- `sdk.InjectorContext`

Important runtime facts:

- `InjectorContext.Environment()` tells injectors/init whether the request is in `prod` or `test`.
- `ExternalIntegrationRequest.Environment` is the normalized environment from the external request.
- Data plane uses API key prefix as environment selector.
- Management plane uses route path for environment-scoped workspace operations.

## Which reference to load next

- **Need exact MCP workflows or endpoint usage** → `references/mcp-workflows.md`
- **Need to register SDK functions or understand interfaces** → `references/sdk-extension-points.md`
- **Need to understand `prod/test` behavior** → `references/environment-model.md`
- **Need external embed/bootstrap/session/auth/resolver details** → `references/external-integration-flow.md`
- **Need `_system`, sharing, defaults, forks, inheritance** → `references/resolution-and-inheritance.md`
