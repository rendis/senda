# Senda

Open-source email orchestration platform with multi-tenant hierarchy, template versioning, and provider-agnostic delivery.

## What is Senda?

Senda lets you manage transactional email across organizations with a 3-level hierarchy: **Global → Tenant → Workspace**. Templates, injectors, and adapters inherit down the chain — configure once at the top, override where needed.

### Key Features

- **Multi-tenant hierarchy** — Global, Tenant, and Workspace scopes with inheritance chain resolution
- **Template versioning** — Draft → Published → Archived lifecycle with locale support (i18n)
- **Provider-agnostic** — Adapter system supports SES, Gmail, SMTP; any provider tomorrow
- **MJML-native** — Write responsive email templates in MJML, compiled to HTML at send time
- **Provider-managed security** — SPF, DKIM, and DMARC handled natively by email providers, not duplicated in application code
- **Dual auth** — OIDC/JWT for humans (management plane), API Keys for machines (data plane)
- **Webhook system** — Real-time event delivery with HMAC-SHA256 signatures
- **Full audit trail** — Every mutation logged with actor, scope, and change diff

## Stack

| Layer           | Technology              |
| --------------- | ----------------------- |
| Language        | Go 1.25+                |
| Database        | PostgreSQL 16 + pg_cron |
| HTTP            | Echo v5                 |
| Queue           | River (PG-native)       |
| DB Driver       | pgx v5                  |
| Migrations      | golang-migrate          |
| Email Templates | gomjml                  |
| Cache           | PG UNLOGGED tables      |
| Rate Limiting   | PL/pgSQL token bucket   |

No Redis. No external message broker. PostgreSQL handles everything.

## Architecture

Hexagonal (Ports & Adapters). Domain logic has zero infrastructure dependencies — all external systems are accessed through port interfaces.

```
cmd/senda/          → Entry point + DI composition
internal/
  domain/           → Entities, value objects, domain errors
  port/             → Interface definitions (contracts)
  service/          → Business logic orchestration
  resolution/       → Hierarchy chain resolution engine
  adapter/
    postgres/       → Store implementations (pgx v5)
    pgcache/        → PG UNLOGGED cache adapter
    ses/            → AWS SES email sender
    river/          → Background workers
    mjml/           → MJML → HTML compiler
    crypto/         → AES-256-GCM encryption
  http/
    handler/        → HTTP handlers
    middleware/      → Auth, RBAC, logging, metrics
pkg/                → Shared utilities (apperr, slug, tracking)
migrations/         → SQL migrations
config/             → YAML configuration
```

## Quick Start

```bash
# Clone
git clone https://github.com/senda-app/senda.git
cd senda

# Start dev environment (Go + PostgreSQL + pg_cron)
make dev

# Run migrations
make migrate-up

# Run tests
make test
```

### Prerequisites

- Docker & Docker Compose
- Go 1.25+ (for local development)
- Make

## Configuration

Senda uses YAML configuration with environment variable overrides (`SENDA_` prefix):

```bash
cp config/config.example.yaml config/config.yaml
# Edit config.yaml with your settings
```

Key configuration areas: database connection, OIDC provider, master encryption key, logging level. See `config/config.example.yaml` for all options.

## API Overview

### Send an email (API Key auth)

```bash
curl -X POST https://your-senda/api/v1/send \
  -H "Authorization: ApiKey senda_live_xxx..." \
  -H "Content-Type: application/json" \
  -d '{
    "to": "user@example.com",
    "template_type": "acme:main:welcome-email",
    "variables": { "name": "Jane", "activation_url": "https://..." },
    "locale": "es"
  }'
```

Response: `{ "tracking_id": "snd_...", "status": "queued" }`

### Management (OIDC auth)

All CRUD operations for tenants, workspaces, templates, adapters, members, and webhooks are available under `/api/v1/` with OIDC Bearer token authentication and RBAC enforcement.

## How it Works

1. **Parse address** — `tenant:workspace:template-type` identifies the target
2. **Resolve chain** — Walk the hierarchy: Workspace → \_system → Global
3. **Resolve template** — Find published version + locale fallback
4. **Merge injectors** — Field-by-field merge across scopes
5. **Resolve adapter** — Get provider credentials from template type
6. **Validate identity** — Verify from_email is a verified sender identity in the provider
7. **Check suppression** — Skip bounced/complained addresses
8. **Compile** — MJML → HTML with variable rendering
9. **Enqueue** — River job for async delivery with rate limiting
10. **Deliver** — Worker sends via provider, tracks events

### Email Security (SPF, DKIM, DMARC)

Senda **does not** implement SPF, DKIM, or DMARC at the application layer. These protocols are handled natively by the delivery provider:

- **SPF (Sender Policy Framework)** — DNS-level protocol. The provider's sending infrastructure is already authorized in their SPF records. No application code involved.
- **DKIM (DomainKeys Identified Mail)** — The provider signs outgoing messages with their own DKIM keys and manages DNS records. Implementing DKIM in the app would conflict with or duplicate the provider's signing.
- **DMARC (Domain-based Message Authentication)** — Policy enforcement configured via DNS. The domain owner sets the policy; providers align SPF and DKIM automatically.

Senda validates sender authorization through the provider's **identity verification system** (verified emails/domains in SES, OAuth scopes in Gmail). If a from_email is not verified with the provider, the provider itself rejects the send attempt — no application-level domain validation needed.

## Development

```bash
make dev              # Start full stack with Docker Compose
make build            # Build binary
make test             # Unit tests
make test-integration # Integration tests (TestContainers)
make lint             # golangci-lint
make migrate-up       # Apply migrations
make migrate-down     # Rollback migrations
```

### Testing

TDD mandatory. Manual mocks (no frameworks). TestContainers for integration tests with real PostgreSQL.

## Project Status

Under active development — See [stories/MANIFEST.md](stories/MANIFEST.md) for current progress.

## License

[MIT](LICENSE)
