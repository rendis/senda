# Senda Development Guide

## Prerequisites

| Tool                    | Version | Required                                          |
| ----------------------- | ------- | ------------------------------------------------- |
| Docker + Docker Compose | Latest  | Yes                                               |
| Go                      | 1.25+   | Yes                                               |
| Node.js                 | 22+     | Yes (frontend)                                    |
| Make                    | Any     | Yes                                               |
| golang-migrate CLI      | Latest  | Optional (migrations run on app start by default) |

## First-Time Setup

```bash
git clone https://github.com/senda-app/senda.git
cd senda
make dev          # starts senda + postgres + keycloak + mailpit
# wait ~30s for keycloak health check
curl localhost:8081/health  # verify backend
```

`make dev` brings up the full development stack via Docker Compose: the Senda API server with hot reload, PostgreSQL 16 with pg_cron, Keycloak as the OIDC provider, and Mailpit as the local email sink.

## Backend Development

- **Module:** `github.com/senda-app/senda`
- **Go version:** 1.25.1
- **Hot reload:** Provided by [Air](https://github.com/air-verse/air) via `docker/Dockerfile.dev`
- **Auto-rebuild:** Code changes under `internal/` trigger automatic rebuild inside the container

The dev container mounts the project root as a volume, so any file change in `internal/`, `cmd/`, or `pkg/` is picked up by Air and triggers a rebuild + restart of the server process.

No local Go installation is strictly required for backend work (everything runs inside Docker), but having Go installed locally is recommended for IDE support, running tests outside Docker, and using `go vet`/`golangci-lint`.

## Frontend Development

```bash
npm --prefix web install
npm --prefix web run dev    # starts on localhost:3000
```

- **Framework:** Next.js 16
- **UI:** React 19, TypeScript 5, Tailwind v4
- **API proxy:** All `/api/v1/*` requests are proxied to the backend (configured in `next.config.ts` rewrites)
- **Environment:** Copy `web/.env.local.example` to `web/.env.local` and configure:

| Variable                  | Description                                            |
| ------------------------- | ------------------------------------------------------ |
| `NEXT_PUBLIC_API_URL`     | Backend API base URL (default:`http://localhost:8081`) |
| `AUTH_SECRET`             | Auth.js session encryption secret                      |
| `AUTH_OIDC_ISSUER`        | OIDC issuer URL (Keycloak)                             |
| `AUTH_OIDC_CLIENT_ID`     | OIDC client ID                                         |
| `AUTH_OIDC_CLIENT_SECRET` | OIDC client secret                                     |

## Database

- **Engine:** PostgreSQL 16 + pg_cron extension
- **Auto-migrate:** Migrations run automatically on application start (config: `migrate_on_start: true`)
- **Manual migration:**
  ```bash
  make migrate-up     # apply all pending migrations
  make migrate-down   # rollback last migration
  ```
- **Direct connection:**
  ```bash
  psql postgres://senda:senda@localhost:5435/senda
  ```
- **Migrations directory:** `/migrations/` (24 migration files)
- **Migration tool:** [golang-migrate](https://github.com/golang-migrate/migrate)

## Docker Compose Stacks

The project provides two Docker Compose configurations for different purposes:

| Aspect       | Dev (`docker-compose.yml`) | E2E (`docker-compose.e2e.yml`) |
| ------------ | -------------------------- | ------------------------------ |
| Senda port   | 8081                       | 8090                           |
| PG port      | 5435                       | 5436                           |
| Mailpit SMTP | 1026                       | 2025                           |
| Mailpit UI   | 8026                       | 9025                           |
| OIDC mode    | `oidc`                     | `dual` (OIDC + test JWT)       |
| Hot reload   | Yes (Air)                  | No (compiled binary)           |

The E2E stack uses `dual` auth mode so tests can authenticate with both real OIDC tokens and lightweight test JWTs for speed and determinism.

## Makefile Reference

All 27 available targets:

### Development

| Target           | Description                                                              |
| ---------------- | ------------------------------------------------------------------------ |
| `make dev`       | Start the full development stack (senda + postgres + keycloak + mailpit) |
| `make dev-down`  | Stop the development stack                                               |
| `make dev-clean` | Stop and remove all dev containers, volumes, and networks                |

### Build

| Target       | Description            |
| ------------ | ---------------------- |
| `make build` | Build the Senda binary |

### Testing

| Target                    | Description                                             |
| ------------------------- | ------------------------------------------------------- |
| `make test`               | Run unit tests with race detector                       |
| `make test-integration`   | Run integration tests (TestContainers)                  |
| `make test-e2e`           | Start E2E stack, run deterministic gate, stop stack     |
| `make test-e2e-run`       | Run E2E tests (assumes stack is already up)             |
| `make test-e2e-full`      | Start E2E stack, run full E2E suite, stop stack         |
| `make test-e2e-full-run`  | Run full E2E suite (assumes stack is already up)        |
| `make test-e2e-core`      | Start E2E stack, run core E2E tests, stop stack         |
| `make test-e2e-core-run`  | Run core E2E tests (assumes stack is already up)        |
| `make test-e2e-chaos`     | Start E2E stack, run chaos/resilience tests, stop stack |
| `make test-e2e-chaos-run` | Run chaos tests (assumes stack is already up)           |
| `make test-e2e-up`        | Start the E2E Docker Compose stack                      |
| `make test-e2e-down`      | Stop the E2E Docker Compose stack                       |

### System Tests

| Target                          | Description                                                 |
| ------------------------------- | ----------------------------------------------------------- |
| `make system-validate-manifest` | Validate the system test manifest                           |
| `make system-matrix`            | Run the system test matrix                                  |
| `make system-pr`                | Run PR-level system tests (API contract + UI flow + visual) |
| `make system-nightly`           | Run full nightly system test suite                          |
| `make system-down`              | Stop system test infrastructure                             |

### Database

| Target              | Description                           |
| ------------------- | ------------------------------------- |
| `make migrate-up`   | Apply all pending database migrations |
| `make migrate-down` | Rollback the last database migration  |

### Quality

| Target      | Description       |
| ----------- | ----------------- |
| `make lint` | Run golangci-lint |

### Cleanup

| Target       | Description                                |
| ------------ | ------------------------------------------ |
| `make clean` | Remove build artifacts and temporary files |

### Help

| Target      | Description                                  |
| ----------- | -------------------------------------------- |
| `make help` | Show all available targets with descriptions |

## Dev Services Access

| Service                | URL                   | Credentials       |
| ---------------------- | --------------------- | ----------------- |
| Senda API              | http://localhost:8081 | --                |
| Keycloak Admin Console | http://localhost:9090 | `admin` / `admin` |
| Mailpit UI             | http://localhost:8026 | --                |
| PostgreSQL             | `localhost:5435`      | `senda` / `senda` |

## Keycloak Test Users

The development Keycloak realm (`senda`) is pre-configured with the following test users:

| Email                      | Password           | Role            |
| -------------------------- | ------------------ | --------------- |
| admin@senda.dev            | `admin`            | Superadmin      |
| tenant-admin@senda.dev     | `tenant-admin`     | TenantAdmin     |
| workspace-admin@senda.dev  | `workspace-admin`  | WorkspaceAdmin  |
| workspace-editor@senda.dev | `workspace-editor` | WorkspaceEditor |
| workspace-viewer@senda.dev | `workspace-viewer` | WorkspaceViewer |

These users cover all RBAC roles in the system. The Keycloak realm configuration lives in `docker/keycloak/realm-senda.json`.

## Testing Guide

### Unit Tests

```bash
make test
```

Runs `go test -race ./...`. Tests use manual mocks (no mock frameworks). All unit tests live alongside the code they test (e.g., `handler_test.go` next to `handler.go`).

### Integration Tests

```bash
make test-integration
```

Uses [TestContainers](https://golang.testcontainers.org/) to spin up real PostgreSQL instances. Integration test files are tagged with `//go:build integration` so they do not run during `make test`.

### E2E Tests

```bash
make test-e2e
```

Starts the full Docker Compose E2E stack, runs the deterministic E2E test gate, and tears down the stack when done. Use `make test-e2e-run` if you already have the stack running.

### E2E Chaos Tests

```bash
make test-e2e-chaos
```

Non-blocking resilience and chaos tests. These exercise failure modes (network partitions, provider timeouts, concurrent mutations) and are not part of the main gate.

### System Tests

```bash
make system-pr       # PR gate: API contract + UI flow + visual regression
make system-nightly  # Full nightly suite
```

System tests cover end-to-end API contracts, UI flows, and visual regression. The PR gate runs a subset for fast feedback; the nightly suite runs the complete matrix.

## Troubleshooting

### Port Conflicts

If default ports collide with other services on your machine, override them with environment variables:

```bash
SENDA_PORT=8082 SENDA_PG_PORT=5437 KEYCLOAK_PORT=9091 make dev
```

### Keycloak Slow Start

Keycloak has a 30-second health check start period configured in Docker Compose. This is normal. If `make dev` appears to hang, wait for the health check to pass. You can monitor progress with:

```bash
docker compose logs -f keycloak
```

### Migration Dirty State

If a migration fails mid-way, the schema_migrations table may be left in a dirty state. Fix it by forcing the version:

```bash
migrate -database "postgres://senda:senda@localhost:5435/senda?sslmode=disable" \
        -path migrations force <version>
```

Replace `<version>` with the last successfully applied migration version number.

### PostgreSQL Not Ready

If the application fails to connect to PostgreSQL on startup:

```bash
docker compose logs postgres    # check container health
docker compose ps               # verify container is running
```

PostgreSQL has a health check configured. If the container shows as `unhealthy`, check disk space and available memory.
