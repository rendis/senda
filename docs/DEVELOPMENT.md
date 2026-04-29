
# Senda Development Guide

## Prerequisites

- Docker
- Go 1.26+
- Node 25.x (aligned with `web/.nvmrc`)
- Make
- Corepack (recommended for `pnpm`)

## First-time setup

```bash
git clone https://github.com/rendis/senda.git
cd senda
corepack enable
(cd web && corepack install)
pnpm --dir web install
make dev
curl http://localhost:8081/health
```

`make dev` starts the full local stack: Senda API, PostgreSQL, Keycloak, Mailpit, and the Next.js frontend. If you only want the Docker services, use `make dev-stack`.

## Frontend development

Use `pnpm` only.

```bash
corepack enable
(cd web && corepack install)
pnpm --dir web install
make dev        # full local stack, including frontend
make dev-stack  # docker services only
```

Useful frontend checks:

```bash
pnpm --dir web test
pnpm --dir web typecheck
pnpm --dir web lint -- --max-warnings=0
```

The standard local validation flow does **not** require a local `next build`.

## Backend development

Senda is a Go monorepo with the public module:

`github.com/rendis/senda`

Useful areas:

- `sdk/` — public SDK
- `internal/domain/` — domain model
- `internal/port/` — contracts and extension seams
- `internal/service/` — business logic
- `internal/resolution/` — inheritance and injector resolution
- `internal/http/` — routes, handlers, middleware

## Dev services

| Service | URL / Port |
| --- | --- |
| API | `http://localhost:8081` |
| Keycloak | `http://localhost:9090` |
| Mailpit UI | `http://localhost:8026` |
| PostgreSQL | `localhost:5435` |
| Frontend | `http://localhost:3000` |

## Keycloak test users

| Email | Password | Role |
| --- | --- | --- |
| admin@senda.dev | `admin` | Superadmin |
| tenant-admin@senda.dev | `tenant-admin` | TenantAdmin |
| workspace-admin@senda.dev | `workspace-admin` | WorkspaceAdmin |
| workspace-editor@senda.dev | `workspace-editor` | WorkspaceEditor |
| workspace-viewer@senda.dev | `workspace-viewer` | WorkspaceViewer |

## Make targets

### Development

- `make dev`
- `make dev-stack`
- `make dev-down`
- `make dev-clean`

### Validation

- `make test`
- `make test-integration`
- `make test-e2e`
- `make test-e2e-full`
- `make test-e2e-core`
- `make test-e2e-chaos`
- `make test-e2e-ses`
- `make ci-backend-pr`
- `make ci-frontend`
- `make ci-pr`
- `make ci-taxonomy-check`
- `make lint`
- `make vet`

### System tests

- `make system-validate-manifest`
- `make system-matrix`
- `make system-pr`
- `make system-nightly`
- `make system-down`

### Database and helpers

- `make migrate-up`
- `make migrate-down`
- `make install-githooks`
- `make clean`
- `make help`

## Environment-aware testing

Senda now includes environment-aware runtime flows. When validating changes that touch workspace environment behavior, cover at least one of these:

- `make test-e2e`
- `make test-e2e-chaos` for crash/recovery or cross-layer runtime behavior
- `make system-pr` for manual / observational browser/API validation
- `make system-nightly` for manual / observational broader observability and resilience checks

The system harness includes a dedicated environment-mode stage:

- `test/system/subagents/ui-environment-mode-tester.sh`

## E2E notes

The deterministic and chaos E2E suites use self-managed Testcontainers harnesses. Prefer the Make targets over manual stack management.

Useful suites:

```bash
make test-e2e
make test-e2e-chaos
make test-e2e-ses
```

## Validation taxonomy

- `make ci-backend-pr` is the automatic backend PR gate.
- `make ci-frontend` is the automatic frontend PR gate and uses `pnpm --dir web test` as the canonical frontend test entrypoint.
- `make ci-pr` composes the automatic backend and frontend PR gates.
- `make ci-taxonomy-check` compares Makefile, workflows, and docs so drift cannot hide.
- `make system-pr` and `make system-nightly` are manual / observational workflow_dispatch gates, not automatic PR blockers.

## SDK / external integration development

When working on embedding or external integrations, verify against these source-of-truth files:

- `sdk/engine.go`
- `sdk/interfaces.go`
- `internal/port/code_injector.go`
- `internal/port/external_integration.go`
- `internal/http/server.go`
- `internal/http/middleware/external_integration.go`

## Migrations and data model

Database migrations live in `migrations/`. Do not hardcode migration counts in docs or tooling; rely on the directory contents.
