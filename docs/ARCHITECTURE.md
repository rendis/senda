
# Senda Architecture

Senda is a PostgreSQL-first email orchestration platform with hierarchical resolution, environment-aware workspaces, and a public embedding SDK.

## Core topology

Senda has four surfaces:

1. **Management plane** — OIDC + RBAC for admin and content operations
2. **Data plane** — workspace API keys for send/query
3. **External integration surface** — embeddable builder/editor APIs with custom auth/resolver seams
4. **SDK surface** — Go extension points for embedders

## Layering

- `internal/domain/` — pure domain model and environment/policy types
- `internal/port/` — contracts for stores, senders, and public extension seams
- `internal/service/` — business orchestration
- `internal/resolution/` — scope, injector, and template resolution
- `internal/adapter/` — Postgres, River, SES, Gmail, SMTP, MJML, crypto, cache
- `internal/http/` — handlers and middleware
- `sdk/` — public aliases and engine builder for embedders

## Domain entities

### Hierarchy and auth

| Entity | Purpose |
| --- | --- |
| `Tenant` | top-level organization |
| `Workspace` | operational unit inside a tenant |
| `_system` workspace | tenant-wide defaults and selective sharing control point |
| `Member` / `MemberRole` | OIDC-backed human access and RBAC |
| `APIKey` | machine credential for one workspace environment |

### Messaging model

| Entity | Purpose |
| --- | --- |
| `Adapter` | provider configuration |
| `AdapterIdentity` | provider identity (email/domain) |
| `TemplateType` | slug-addressable sending category |
| `Template` | concrete content holder for a template type |
| `TemplateVersion` | draft/published/archive version |
| `TemplateVersionLocale` | locale-specific override |
| `Email` / `EmailEvent` | send state and event history |
| `Webhook` | outbound notification target |

## Environment model

Each logical workspace exists as two operational environments:

- `prod`
- `test`

Environment affects:

- API key generation (`senda_prod_...`, `senda_test_...`)
- management routing (`/manage/environments/:environment/...`)
- external integration requests (`X-Senda-Environment`)
- runtime safety controls in test
- runtime reset availability in test

## Resolution and inheritance

Resolution is scope-aware:

`workspace -> tenant _system -> global`

Three important states:

- **owned**
- **inherited**
- **shared**

Selective sharing rules:

- Gmail sharing is adapter-level from `_system`
- SES sharing is email-identity-level from `_system`
- shared child-workspace resources are read-only
- template forks create local ownership from inherited resources
- exact version cloning copies a version and all locales into a new draft

## External integration architecture

External builder/editor access is profile-driven.

A global external integration profile binds:

- auth method name
- workspace resolver name
- allowed origins
- allowed/required headers
- capability flags

Runtime pipeline:

1. load profile
2. validate headers and environment
3. authenticate with registered auth method
4. resolve workspace with registered workspace resolver
5. enforce effective permissions on external routes

## Public SDK architecture

The public SDK is intentionally narrow.

Supported extension points:

- code injectors
- one per-request init function
- external auth methods
- external workspace resolvers
- startup hooks
- shutdown hooks

Not exposed through the SDK:

- provider adapter registration
- internal stores
- internal queue/cache/crypto wiring
- internal HTTP middleware composition

## Auth model

| Plane | Auth |
| --- | --- |
| Management | OIDC bearer token |
| Data plane | workspace API key |
| External integration | custom profile-driven auth + `X-Senda-Environment` |

RBAC applies to management routes. External integration permissions are profile/auth-result driven rather than normal RBAC roles.

## Infrastructure decisions

- PostgreSQL is the system-of-record and operational backbone.
- River provides PG-backed background jobs.
- Cache uses PostgreSQL-backed mechanisms rather than Redis.
- Encryption uses AES-256-GCM with HKDF-derived keys.
- Soft delete and cursor-based pagination are standard patterns across the app.
