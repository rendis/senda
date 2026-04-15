
<div align="center">
  <img src="web/public/senda-logo.svg" width="80" alt="Senda" />
  <h1>Senda</h1>
  <p><strong>Open-source email orchestration for multi-tenant SaaS</strong></p>
  <p>Templates, inheritance, environment-aware delivery, and embeddable builder flows — one platform.</p>

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Next.js](https://img.shields.io/badge/Next.js-16-000?logo=nextdotjs&logoColor=white)](https://nextjs.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![CI](https://github.com/rendis/senda/actions/workflows/backend-gate.yml/badge.svg)](https://github.com/rendis/senda/actions)

[Documentation](docs/) &middot; [API Reference](docs/API.md) &middot; [Quick Start](#quick-start) &middot; [Contributing](#contributing)

</div>

---

## Why Senda?

Most email products are either too simple for multi-tenant SaaS or too operationally heavy. Senda stays in the middle:

- **PostgreSQL-first** — queue, cache, rate limiting, and state all stay close to the core application. No Redis required.
- **Hierarchical resolution** — global, tenant, `_system`, and workspace cooperate through explicit inheritance and selective sharing.
- **Real environments** — each logical workspace operates as `prod` and `test`, with isolated API keys, runtime state, and safety controls.
- **Embeddable** — external builder/editor surfaces can be exposed through custom auth methods and workspace resolvers.
- **SDK-friendly** — embedders can register code injectors, per-request init, external auth/resolver seams, and lifecycle hooks without forking Senda.

## Features

| Area | Feature | Description |
| --- | --- | --- |
| Hierarchy | Scope resolution | Global → Tenant → `_system` → Workspace resolution with owned, inherited, and shared resources |
| Environments | `prod` / `test` workspaces | One logical workspace, two operational environments with isolated runtime state |
| Templates | Versioned + localized | Draft/published/archive lifecycle, locale overrides, exact version cloning |
| Providers | Adapter model | SES, Gmail, SMTP built in; adapter and identity sharing from `_system` |
| Safety | Test recipient policy | `replace` or `append` safe-recipient behavior in `test` only |
| External | Embeddable builder surface | Bootstrap, session, template editing, locale editing, preview, and test-send via external profiles |
| Security | OIDC + API keys | OIDC for management, environment-aware API keys for data plane |
| SDK | Public extension seams | Injectors, init function, external auth method, workspace resolver, lifecycle hooks |

## Environment model

Each logical workspace has two physical operational environments:

- `prod`
- `test`

Important rules:

- The **send ref stays** `tenant:workspace:templateType`.
- Environment is selected by **route**, **API key prefix**, or **external header**, depending on the surface.
- Test-only runtime reset is exposed on the environment-scoped management surface.
- Test-only recipient safety controls exist at workspace level and can be overridden by template type in the test environment.

See `docs/API.md` and `skills/senda/` for the operational details.

## Shared adapters from tenant `_system`

Senda supports **selective sharing** from a tenant `_system` workspace:

- **Gmail** sharing is adapter-level.
- **SES** sharing is email-identity-level; domain identities are not shareable.
- Shared entries are read-only in child workspaces.
- Child workspaces can fork inherited template content when they need local ownership.

## Quick Start

**Prerequisites:** Docker, Go 1.25+, Node 25 (or `web/.nvmrc`), Make, Corepack.

```bash
git clone https://github.com/rendis/senda.git
cd senda
corepack enable
(cd web && corepack install)
pnpm --dir web install
make dev
curl http://localhost:8081/health
```

`make dev` now starts the Docker services and the Next.js frontend together. If you only want the Docker services, use `make dev-stack`.

Example data-plane send:

```bash
curl -X POST http://localhost:8081/api/v1/send \
  -H "Authorization: Bearer senda_prod_YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "ref": "acme:main:welcome-email",
    "to": ["user@example.com"],
    "variables": {"name": "Jane"},
    "locale": "es"
  }'
```

For local validation, prefer the repo gates used by CI:

```bash
make ci-backend-pr
make ci-frontend      # if you changed web/
make ci-pr            # backend + frontend
make ci-taxonomy-check # docs/workflows/Makefile drift guard
make install-githooks
```

## Use as a Library

Senda exposes a public Go SDK for embedders.

```bash
go get github.com/rendis/senda
```

```go
package main

import (
    "context"
    "github.com/rendis/senda/sdk"
)

func main() {
    engine := sdk.NewWithConfig("config.yaml")

    engine.SetInitFunc(func(ctx context.Context, injCtx *sdk.InjectorContext) (any, error) {
        return loadStudent(ctx, injCtx.Header("X-Case-Id"))
    })

    engine.RegisterInjector(&StudentInjector{})
    engine.RegisterExternalAuthMethod(&PortalAuthMethod{})
    engine.RegisterExternalWorkspaceResolver(&PortalWorkspaceResolver{})

    engine.OnStart(func(ctx context.Context) error { return connectMongo(ctx) })
    engine.OnShutdown(func(ctx context.Context) error { return closeMongo(ctx) })

    _ = engine.Run()
}
```

Public extension seams:

- `RegisterInjector(...)`
- `SetInitFunc(...)`
- `RegisterExternalAuthMethod(...)`
- `RegisterExternalWorkspaceResolver(...)`
- `OnStart(...)`
- `OnShutdown(...)`

`InjectorContext.Environment()` and `ExternalIntegrationRequest.Environment` are the public runtime sources of truth for `prod` / `test` behavior.

## External integrations

Senda exposes an embeddable external surface under:

- `/api/v1/external/:profile_slug/bootstrap`
- `/api/v1/external/:profile_slug/environments/:environment/bootstrap`
- `/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/...`

External authenticated requests must include:

```http
X-Senda-Environment: prod|test
```

External profiles select a registered auth method and workspace resolver, which together determine effective permissions and effective workspace.

## API at a glance

| Group | Base | Auth | Notes |
| --- | --- | --- | --- |
| Health | `/health`, `/healthz`, `/metrics` | None / token | Liveness, readiness, Prometheus |
| Data plane | `/api/v1/send`, `/api/v1/emails` | API key | Environment selected by `senda_prod_...` / `senda_test_...` |
| Management (shared) | `/api/v1/manage/tenants/.../workspaces/...` | OIDC | Logical workspace CRUD |
| Management (env) | `/api/v1/manage/environments/:environment/...` | OIDC | Environment-specific workspace runtime/resources |
| External | `/api/v1/external/:profile_slug/...` | Custom | Embeddable builder/editor surface |
| Global | `/api/v1/manage/global/...` | OIDC | Superadmin-only global resources |
| Webhooks | `/api/v1/webhooks/ses/inbound` | SNS sig | Provider event ingestion |

## MCP integration

Senda ships a dedicated MCP skill and OpenAPI-backed MCP server configuration.

- Skill bundle: `skills/senda/`
- Setup guide: `docs/mcp_setup.md`
- OpenAPI-backed server name: `senda`

For data-plane MCP usage, authenticate with a raw workspace API key such as `senda_prod_...` or `senda_test_...`.

## Development

| Command | Description |
| --- | --- |
| `make dev` | Start the Docker development stack |
| `make test` | Unit tests with race detector |
| `make test-integration` | Integration tests |
| `make test-e2e` | Deterministic E2E gate |
| `make test-e2e-chaos` | Chaos/resilience E2E gate |
| `make system-pr` | Manual / observational system browser/API gate |
| `make system-nightly` | Manual / observational nightly system gate |
| `make lint` | Go linting |
| `make ci-taxonomy-check` | Drift check for commands, docs, and workflows |
| `pnpm --dir web typecheck` | Frontend typecheck |
| `pnpm --dir web test` | Canonical frontend test entrypoint |
| `pnpm --dir web lint` | Frontend lint |

## Project structure

```text
senda/
├── sdk/                    Public SDK
├── cmd/senda/              Server entry point
├── internal/
│   ├── domain/             Domain model and environment types
│   ├── port/               Public extension and internal contracts
│   ├── service/            Business logic
│   ├── resolution/         Scope and injector resolution
│   ├── adapter/            Postgres, River, SES, Gmail, SMTP, MJML, crypto
│   ├── http/               Handlers and middleware
│   └── app/                Bootstrap and extension bridge
├── migrations/             SQL migrations
├── docs/                   Human-facing docs
├── skills/senda/           Self-contained skill bundle for agents
└── web/                    Next.js frontend
```
