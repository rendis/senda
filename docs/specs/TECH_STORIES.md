# Technical Stories (HTs) — Senda P1

**Reference:** TECH_SPEC v1.4 | PRD v5.0

**Granularity:** Feature-level (3–5 days per HT)

**Conventions:**
- Every HT includes: objective, relevant spec sections, acceptance criteria, and dependencies.
- TDD is mandatory: every HT produces unit tests. E2E with TestContainers where applicable.
- HTs are grouped into **Epics** and ordered by dependency.

---

## Epic Map

| Epic | Scope | HTs |
|-------|-------|-----|
| E1 — Foundation | Scaffolding, DB, Config, Crypto | HT-01 to HT-04 |
| E2 — Core Domain | Models, Ports, Stores | HT-05 to HT-09 |
| E3 — Resolution Engine | Chain, Injectors, Templates, Adapters, Domains | HT-10 to HT-12 |
| E4 — Send Flow | SendService, Workers, DKIM, Rate Limiting | HT-13 to HT-16 |
| E5 — API Layer | Middleware, Auth, Handlers, Contract | HT-17 to HT-22 |
| E6 — Operations | Provider Events, Webhooks, Onboarding, Observability | HT-23 to HT-27 |

---

## E1 — Foundation

### HT-01: Project Scaffolding + Docker Compose

**Objective:** Create the Go project folder structure, initialize the module, set up Docker Compose with PostgreSQL 16 + pg_cron, and establish the base Makefile.

**Spec:** §9 (Folder Structure), §18 (Docker Compose)

**Deliverables:**
- `go.mod` initialized with module `github.com/rendis/senda`
- Folder structure: `cmd/senda/`, `internal/{domain,port,service,resolution,adapter,http}/`, `migrations/`, `config/`, `pkg/`, `docker/`
- `docker-compose.yml` with postgres:16-alpine + pg_cron (`shared_preload_libraries`), caddy (https profile)
- `Dockerfile` (multi-stage: build + alpine runtime)
- `Dockerfile.dev` (with hot reload via air or similar)
- `Makefile` with targets: `dev`, `build`, `test`, `migrate-up`, `migrate-down`, `lint`
- `.gitignore`, `.editorconfig`

**Acceptance Criteria:**
- [ ] `make dev` brings up the full stack (senda + postgres) with docker compose
- [ ] PostgreSQL accepts connections and pg_cron is enabled (`SELECT * FROM cron.job` does not fail)
- [ ] `make build` generates the binary
- [ ] `make test` runs tests (even if there are none yet)

**Dependencies:** None (first HT)

---

### HT-02: Configuration + Secrets Management

**Objective:** Implement configuration loading from YAML + environment variables, with validation and defaults.

**Spec:** §17 (Configuration)

**Deliverables:**
- `config/config.go` — `Config` struct with sub-configs: `ServerConfig`, `DatabaseConfig`, `OIDCConfig`, `CryptoConfig`, `LogConfig`
- YAML parsing + env var overrides (`SENDA_` prefix)
- `config/config.example.yaml` with all values documented
- Validation: required fields, URL formats, master key length

**Acceptance Criteria:**
- [ ] Config loads from YAML, env var overrides work
- [ ] Clear errors when required configuration is missing
- [ ] `config.example.yaml` includes all fields with comments
- [ ] Unit tests for parsing, defaults, and validation

**Dependencies:** HT-01

---

### HT-03: Database Connection + Migrations Runner

**Objective:** Implement the pgx v5 connection pool, integrate golang-migrate, and create all SQL migrations.

**Spec:** §7 (Full Migration Strategy — 19 migrations), §3 (SQL Schema), §4 (Partitioning), §5 (Indexes)

**Deliverables:**
- `internal/adapter/postgres/db.go` — connection pool (pgxpool), health check, graceful shutdown
- golang-migrate integration: auto-run on start (configurable)
- **19 migration files** (up + down):
  - 000001: Extensions (pgcrypto, pg_cron)
  - 000002: 10 ENUMs
  - 000003: tenants + workspaces (with CHECKs, EXCLUDE)
  - 000004: injectors (definitions, fields, values)
  - 000005: adapters (with default EXCLUDE, rate_limit_per_second)
  - 000006: domains (with domain_status, DKIM, dns_records)
  - 000007: template_types (with adapter_id) + templates
  - 000008: template_versions + locales (with one-published EXCLUDE)
  - 000009: members + member_roles (with scope_type, CHECK)
  - 000010: api_keys
  - 000011: emails + email_events (partitioned by month)
  - 000012: suppression lists
  - 000013: webhooks
  - 000014: audit_logs (partitioned)
  - 000015: global_config + seed data
  - 000016: UNLOGGED tables (cache, token_buckets)
  - 000017: PL/pgSQL functions (get_resolution_chain, take_send_token)
  - 000018: Performance indexes
  - 000019: pg_cron jobs (cache cleanup, partition creation)

**Acceptance Criteria:**
- [ ] `make migrate-up` runs all 19 migrations without errors
- [ ] `make migrate-down` rolls back all migrations
- [ ] All tables, constraints, ENUMs, functions, and indexes exist
- [ ] pg_cron jobs are registered (`SELECT * FROM cron.job` returns 2 jobs)
- [ ] Integration test with TestContainers: migrate up → verify tables → migrate down → verify clean state

**Dependencies:** HT-01, HT-02

---

### HT-04: Encryption Module (AES-256-GCM)

**Objective:** Implement symmetric encryption/decryption for adapter credentials and DKIM keys.

**Spec:** §1.7 (Encrypted at rest), §17 (CryptoConfig with master_key)

**Deliverables:**
- `internal/adapter/crypto/aes.go` — `AESCrypto` struct implementing the `Crypto` port
- `internal/port/crypto.go` — `Crypto` interface `{ Encrypt(plaintext []byte) ([]byte, error); Decrypt(ciphertext []byte) ([]byte, error) }`
- Key derivation from master key (HKDF or similar)
- Unique nonce per operation (GCM includes nonce in ciphertext)

**Acceptance Criteria:**
- [ ] Encrypt → Decrypt round-trip preserves the plaintext
- [ ] Different ciphertext for the same plaintext (random nonce)
- [ ] Clear error if the master key is invalid or ciphertext is corrupt
- [ ] Unit tests with test vectors

**Dependencies:** HT-02

---

## E2 — Core Domain

### HT-05: Domain Models + Error Types

**Objective:** Define all domain entities as Go structs and define domain errors.

**Spec:** §11 (Complete Domain Models — 11.1 to 11.11)

**Deliverables:**
- `internal/domain/tenant.go` — Tenant, Workspace
- `internal/domain/injector.go` — InjectorDefinition, InjectorField, InjectorValue
- `internal/domain/adapter.go` — Adapter (with AdapterType, RateLimitPerSecond)
- `internal/domain/template.go` — TemplateType (with AdapterID), Template, TemplateVersion, TemplateVersionLocale
- `internal/domain/email.go` — Email (with TrackingID, BodyMJML, snapshots), EmailEvent, EmailStatus
- `internal/domain/member.go` — Member (with OIDC), MemberRole, Role, ScopeType
- `internal/domain/apikey.go` — APIKey (with KeyPrefix, KeyHint, RevokedAt)
- `internal/domain/domain_record.go` — Domain (with DomainStatus, DKIMSelector, DNSRecords)
- `internal/domain/suppression.go` — SuppressionGlobal, SuppressionWorkspace
- `internal/domain/webhook.go` — Webhook (with ConsecutiveFailures, DisabledAt)
- `internal/domain/audit.go` — AuditLog (with AuditAction, ScopeType, Changes, Metadata)
- `internal/domain/config.go` — GlobalConfig
- `internal/domain/addressing.go` — Address struct (TenantCode:WorkspaceCode:TemplateTypeSlug)
- `internal/domain/errors.go` — ErrNotFound, ErrConflict, ErrValidation, ErrForbidden, ErrNoAdapterConfigured, etc.
- `pkg/apperr/errors.go` — application error types with HTTP status mapping
- `pkg/slug/slug.go` — slug validation (regex + reserved words)
- `pkg/tracking/id.go` — tracking ID generation

**Acceptance Criteria:**
- [ ] All §11 entities are defined with the same fields and types
- [ ] Enums are mapped correctly (EmailStatus, Role, ScopeType, etc.)
- [ ] Domain errors implement `error` and are distinguishable with `errors.Is`
- [ ] Slug validation covers the DB CHECK constraint regex
- [ ] Unit tests for: slug validation, tracking ID generation, address parsing

**Dependencies:** HT-01

---

### HT-06: Port Interfaces (Contracts)

**Objective:** Define all interfaces (ports) connecting the domain to infrastructure.

**Spec:** §10 (Port Interfaces — 10.1 to 10.5)

**Deliverables:**
- `internal/port/email_sender.go` — `EmailSender` interface (SendEmail, with context + EmailMessage)
- `internal/port/store.go` — all store interfaces:
  - `TenantStore` (Create, GetByID, GetByCode, List, Update, SoftDelete)
  - `WorkspaceStore` (Create, GetByID, GetByCode, ListByTenant, Update, SoftDelete, GetSystem)
  - `InjectorStore` (CRUD + GetByScope — resolution by workspace_id IN chain)
  - `TemplateTypeStore` (CRUD + GetBySlugInChain)
  - `TemplateStore` (CRUD + GetByTypeInChain)
  - `TemplateVersionStore` (Create, GetPublished, GetByID, List, Publish, Archive)
  - `TemplateVersionLocaleStore` (CRUD per version)
  - `AdapterStore` (CRUD + GetByID)
  - `DomainStore` (CRUD + GetByDomainInChain + GetPendingVerification)
  - `MemberStore` (CRUD + GetByEmail + GetByOIDC)
  - `MemberRoleStore` (Assign, Revoke, ListByMember, GetEffectiveRole)
  - `APIKeyStore` (Create, GetByHash, ListByWorkspace, Revoke)
  - `EmailStore` (Create, UpdateStatus, GetByID, GetByTrackingID, ListByWorkspace)
  - `EmailEventStore` (Append, ListByEmail)
  - `SuppressionStore` (Check, Add, Remove — both global and workspace)
  - `WebhookStore` (CRUD + GetActiveByWorkspace)
  - `AuditLogStore` (Append, ListByScope, ListByResource)
  - `GlobalConfigStore` (Get, Set, GetAll)
- `internal/port/queue.go` — `JobQueue` interface (Enqueue with job types: SendEmail, VerifyDomain, DeliverWebhook)
- `internal/port/template_compiler.go` — `TemplateCompiler` interface (CompileMJML → HTML)
- `internal/port/cache.go` — `Cache` interface (Get, Set, Delete, DeletePattern)
- Helper types: `ListOptions` (cursor-based), `PageResult[T]` (items + next_cursor + has_more)

**Acceptance Criteria:**
- [ ] All interfaces from §10 are defined
- [ ] Every method has `context.Context` as its first parameter
- [ ] `PageResult` is generic (`PageResult[T any]`)
- [ ] `ListOptions` supports cursor + limit
- [ ] Compiles without errors (interfaces are type-correct against domain models)

**Dependencies:** HT-05

---

### HT-07: PostgreSQL Stores — Tenants, Workspaces, Config

**Objective:** Implement the store interfaces for the base entities using pgx v5.

**Spec:** §10.2 (Store Ports), §3.2–3.3 (tenants/workspaces schema), §3.15 (global_config)

**Deliverables:**
- `internal/adapter/postgres/tenant_repo.go` — TenantStore implementation
- `internal/adapter/postgres/workspace_repo.go` — WorkspaceStore implementation
- `internal/adapter/postgres/global_config_repo.go` — GlobalConfigStore implementation
- Base pattern: `pgxpool.Pool` injected, queries with `pgx.NamedArgs`, scans with `pgx.CollectRows`
- Soft delete: queries filter `WHERE deleted_at IS NULL` by default
- Cursor-based pagination implemented as a reusable helper

**Acceptance Criteria:**
- [ ] Full CRUD for tenants and workspaces
- [ ] GetByCode resolves tenant + workspace by code
- [ ] Soft delete works (no physical delete)
- [ ] Cursor pagination produces stable, ordered results
- [ ] GlobalConfig get/set works with JSONB
- [ ] Integration tests with TestContainers (real PG)

**Dependencies:** HT-03, HT-05, HT-06

---

### HT-08: PostgreSQL Stores — Injectors, Adapters, Domains, Templates

**Objective:** Implement stores for email configuration entities.

**Spec:** §10.2, §3.4–3.8

**Deliverables:**
- `internal/adapter/postgres/injector_repo.go` — InjectorStore (definitions + fields + values, with chain-based resolution)
- `internal/adapter/postgres/adapter_repo.go` — AdapterStore (with encrypt/decrypt of credentials)
- `internal/adapter/postgres/domain_repo.go` — DomainStore (with chain lookup, pending verification)
- `internal/adapter/postgres/template_repo.go` — TemplateTypeStore + TemplateStore + TemplateVersionStore + TemplateVersionLocaleStore
- Chain resolution: queries use `workspace_id IN (unnest(get_resolution_chain($1)))` or equivalent Go logic
- Version management: publish (with one-published validation), archive, draft

**Acceptance Criteria:**
- [ ] Injector values are created/resolved by scope (workspace → _system → global)
- [ ] Adapter credentials are encrypted on save and decrypted on read
- [ ] Template versions respect the one-published constraint
- [ ] Domain lookup searches the resolution chain
- [ ] Integration tests: create data in different scopes and verify resolution

**Dependencies:** HT-04, HT-07

---

### HT-09: PostgreSQL Stores — Members, API Keys, Emails, Audit, Suppression, Webhooks

**Objective:** Implement the remaining stores for auth, tracking, and operations.

**Spec:** §10.2, §3.9–3.14

**Deliverables:**
- `internal/adapter/postgres/member_repo.go` — MemberStore + MemberRoleStore (with scope_type checks)
- `internal/adapter/postgres/apikey_repo.go` — APIKeyStore (hash lookup, revoke)
- `internal/adapter/postgres/email_repo.go` — EmailStore (insert, status update, paginated query)
- `internal/adapter/postgres/suppression_repo.go` — SuppressionStore (check global + workspace, add, remove)
- `internal/adapter/postgres/audit_repo.go` — AuditLogStore (append-only, query by scope/resource/member)
- `internal/adapter/postgres/webhook_repo.go` — WebhookStore (CRUD, get active by workspace)
- `internal/adapter/postgres/email_event_repo.go` — EmailEventStore (append, list by email)

**Acceptance Criteria:**
- [ ] Member roles respect the CHECK constraint (scope_type vs role)
- [ ] API key lookup by hash works in under 5ms
- [ ] Email insert includes all columns from §3.11
- [ ] Suppression check queries global + workspace in a single transaction
- [ ] Audit log is append-only (no update, no delete)
- [ ] Integration tests with TestContainers

**Dependencies:** HT-07

---

## E3 — Resolution Engine

### HT-10: ChainResolver + InjectorMerger

**Objective:** Implement the workspace → _system → global hierarchy resolution and field-by-field injector merging.

**Spec:** §12.1 (ChainResolver), §12.2 (InjectorMerger)

**Deliverables:**
- `internal/resolution/chain.go` — `ChainResolver`
  - `Resolve(ctx, tenantCode, workspaceCode)` → `ResolvedChain{Workspace, SystemWorkspace, IsGlobal}`
  - Uses cache (port.Cache) with a 5-minute TTL, key `chain:{tenantCode}:{workspaceCode}`
- `internal/resolution/injector_merger.go` — `InjectorMerger`
  - `Merge(ctx, workspaceID, injectorNames)` → `map[string]map[string]any`
  - Field-by-field merge: workspace values override _system, _system overrides global
  - If a field has no value at any level and is required → error

**Acceptance Criteria:**
- [ ] ChainResolver returns the correct 3 levels (workspace, _system, global)
- [ ] Cache hits avoid DB queries
- [ ] InjectorMerger combines fields from the 3 levels correctly
- [ ] Required fields without values produce a descriptive error
- [ ] Unit tests with mock stores and mock cache

**Dependencies:** HT-06 (ports), HT-08 (stores for integration tests)

---

### HT-11: TemplateResolver + AdapterResolver

**Objective:** Implement template resolution (published version + locale) and adapter resolution (per template type).

**Spec:** §12.3 (TemplateResolver), §12.4 (AdapterResolver)

**Deliverables:**
- `internal/resolution/template_resolver.go` — `TemplateResolver`
  - `Resolve(ctx, workspaceID, templateTypeSlug, locale)` → `ResolvedTemplate{TemplateType, Template, Version, Locale, FinalSubject, FinalBody}`
  - Searches for the template in the chain (workspace → _system → global)
  - Selects the published version
  - Applies locale fallback (requested → default_locale → base)
- `internal/resolution/adapter_resolver.go` — `AdapterResolver`
  - `ResolveForTemplateType(ctx, templateType)` → `ResolvedAdapter`
  - Reads `templateType.AdapterID` → looks up adapter → decrypts credentials
  - If AdapterID is nil → `ErrNoAdapterConfigured` (422)

**Acceptance Criteria:**
- [ ] Template resolution searches in the chain: workspace first, then _system, then global
- [ ] If there is no published template → descriptive error
- [ ] Locale fallback works: es-CO → es → default
- [ ] Adapter resolves from `template_type.adapter_id` (not by chain)
- [ ] No adapter_id → 422 with a clear message
- [ ] Unit tests with mocks; integration test with data in 3 levels

**Dependencies:** HT-10

---

### HT-12: DomainResolver + Cache Invalidation

**Objective:** Validate `from_email` against verified domains and implement cache invalidation strategy.

**Spec:** §12.5 (DomainResolver), §12.6 (Cache Invalidation Strategy)

**Deliverables:**
- `internal/resolution/domain_resolver.go` — `DomainResolver`
  - `Validate(ctx, workspaceID, fromEmail)` → `ResolvedDomain` or error
  - Extracts the domain from `from_email`, searches the chain
  - Accepts only domains with `status = verified`
- `internal/service/cache_invalidator.go` — `CacheInvalidator`
  - `InvalidateWorkspace(ctx, workspaceID)` — deletes keys for that workspace
  - `InvalidateAdapter(ctx, adapterID)` — deletes keys for template types that use that adapter
  - `InvalidateTenantWorkspaces(ctx, tenantID)` — deletes keys for all tenant workspaces
  - `InvalidateGlobal(ctx)` — deletes all chain/template/adapter keys

**Acceptance Criteria:**
- [ ] Unverified-domain `from_email` → 422 error
- [ ] Verified domain in _system is valid for tenant workspaces
- [ ] CacheInvalidator deletes the correct patterns
- [ ] After invalidation, the next resolution queries the DB (no stale cache)
- [ ] Unit tests

**Dependencies:** HT-10, HT-11

---

## E4 — Send Flow

### HT-13: PG Cache + Token Bucket Rate Limiter

**Objective:** Implement the cache adapter using a PG UNLOGGED table and the provider rate limiter using a token bucket PL/pgSQL function.

**Spec:** §23 (PG Cache), §24 (Token Bucket)

**Deliverables:**
- `internal/adapter/pgcache/client.go` — `PGCache` implementing `port.Cache`
  - Get: `SELECT value FROM cache WHERE key = $1 AND expires_at > now()`
  - Set: `INSERT ... ON CONFLICT (key) DO UPDATE`
  - Delete: `DELETE WHERE key = $1`
  - DeletePattern: `DELETE WHERE key LIKE $1`
  - StartCleanup goroutine (fallback for pg_cron)
- `internal/adapter/postgres/rate_limiter.go` — `RateLimiter`
  - `TakeToken(ctx, adapterID) (bool, error)` — calls `SELECT take_send_token($1)`
  - `WaitForToken(ctx, adapterID, timeout)` — retries with backoff until a token is available
- Documented cache TTLs: chain=5min, template=10min, adapter=10min, suppression=1min

**Acceptance Criteria:**
- [ ] Cache Get/Set/Delete works with JSONB
- [ ] Cache entries expire correctly
- [ ] DeletePattern deletes by prefix
- [ ] TakeToken returns true when tokens are available, false when the bucket is empty
- [ ] Token bucket auto-creates from `adapter.rate_limit_per_second`
- [ ] Refill is proportional to elapsed time
- [ ] Integration tests with TestContainers

**Dependencies:** HT-03 (migrations include UNLOGGED tables + PL/pgSQL)

---

### HT-14: MJML Compiler + DKIM Signer

**Objective:** Implement MJML → HTML compilation and DKIM signing for emails.

**Spec:** §22 (DKIM Signing), §10.4 (TemplateCompiler port)

**Deliverables:**
- `internal/adapter/mjml/compiler.go` — `MJMLCompiler` implementing `port.TemplateCompiler`
  - Uses gomjml (native Go) to convert MJML → responsive HTML
  - Fallback: if gomjml fails (unsupported feature), returns a descriptive error
- `internal/adapter/dkim/signer.go` — `DKIMSigner`
  - `GenerateKeyPair()` → (privateKey, publicKey, error) — RSA-2048
  - `Sign(message []byte, domain, selector string, privateKey []byte)` → signed message
  - Uses go-msgauth for DKIM signing
- `internal/adapter/dkim/dns.go` — helpers to generate DNS records (DKIM TXT, SPF, DMARC)

**Acceptance Criteria:**
- [ ] Valid MJML compiles to responsive HTML
- [ ] Invalid MJML returns a descriptive error (no panic)
- [ ] DKIM sign + verify round-trip works
- [ ] Generated DNS records are correct for the config
- [ ] Unit tests with MJML fixtures

**Dependencies:** HT-01 (go.mod deps)

---

### HT-15: SendService — Orchestration Core

**Objective:** Implement the main send service that orchestrates resolution, compilation, and queueing.

**Spec:** §13 (SendService — Main Flow)

**Deliverables:**
- `internal/service/send.go` — `SendService`
  - `Send(ctx, request) → (trackingID, error)`
  - Flow:
    1. Parse address (tenantCode:workspaceCode:templateTypeSlug)
    2. ChainResolver → resolve workspace chain
    3. TemplateResolver → resolve template + locale
    4. InjectorMerger → merge injector values
    5. Render variables into the template (Go text/template)
    6. AdapterResolver → get adapter for the template type
    7. DomainResolver → validate the `from_email` domain
    8. Check suppression (global + workspace)
    9. Compile MJML → HTML
    10. Create email record (status=queued, with snapshots)
    11. Enqueue SendEmail job
    12. Return tracking ID
- Request DTO validation (required fields, email format)

**Acceptance Criteria:**
- [ ] Happy path: send request → email record created → job enqueued → tracking ID returned
- [ ] Suppressed recipient → 422 with reason
- [ ] No adapter configured → 422
- [ ] Unverified domain → 422
- [ ] Template not found → 404
- [ ] Invalid variables (schema mismatch) → 400
- [ ] Snapshots are saved (variables, injectors) for auditability
- [ ] Unit tests with all resolvers mocked
- [ ] Integration test with real stack: send → verify email record + job in queue

**Dependencies:** HT-10, HT-11, HT-12, HT-13, HT-14

---

### HT-16: River Workers (Send, Domain Verify, Webhook)

**Objective:** Implement the background workers that process River queue jobs.

**Spec:** §16 (Background Workers — 16.1 to 16.4)

**Deliverables:**
- `internal/adapter/river/client.go` — River client setup, job type registration
- `internal/adapter/river/send_worker.go` — `SendWorker`
  - Loads the email record, decrypts adapter credentials
  - Rate limit check (TakeToken) — if no token is available, requeue with a delay
  - DKIM signs the message
  - Sends via EmailSender port (SES adapter)
  - Updates email status (sent/failed)
  - Dispatches email events
  - Retry logic: max 3, exponential backoff
- `internal/adapter/river/verify_worker.go` — `DomainVerifyWorker`
  - DNS lookup for DKIM, SPF, DMARC records
  - Update domain status (pending → verified / error)
  - Schedule a recheck
- `internal/adapter/river/webhook_worker.go` — `WebhookWorker`
  - HTTP POST with JSON payload
  - HMAC-SHA256 signature header (`X-Senda-Signature`)
  - Retry: max 5, exponential backoff
  - Track consecutive failures → auto-disable webhook after 10 failures
- `internal/adapter/ses/adapter.go` — `SESAdapter` implementing `port.EmailSender`
  - AWS SDK v2, uses decrypted adapter credentials

**Acceptance Criteria:**
- [ ] SendWorker: sends email via SES, updates status, records event
- [ ] SendWorker: respects rate limiting (requeues if the bucket is empty)
- [ ] SendWorker: DKIM-signs the message before sending
- [ ] SendWorker: retries with backoff on transient errors
- [ ] DomainVerifyWorker: DNS check works, updates status
- [ ] WebhookWorker: HMAC signature is verifiable by the receiver
- [ ] WebhookWorker: auto-disables after 10 consecutive failures
- [ ] SES adapter: sends a real email (test with SES sandbox or mock)
- [ ] Unit tests for each worker; integration test for the full send flow

**Dependencies:** HT-13, HT-14, HT-15

---

## E5 — API Layer

### HT-17: Echo v5 Server + Base Middleware

**Objective:** Configure the Echo v5 HTTP server with base middleware and graceful shutdown.

**Spec:** §14 (Middleware Chain), §8.2 (DI in main.go)

**Deliverables:**
- `internal/http/server.go` — Echo v5 setup, route registration, graceful shutdown
- `internal/http/middleware/requestid.go` — X-Request-ID generation/propagation
- `internal/http/middleware/logger.go` — structured access logging with slog
- `internal/http/middleware/recovery.go` — panic recovery with stack trace logging
- `internal/http/middleware/scope.go` — extract tenant_code/workspace_code from URL params, store them in context
- `cmd/senda/main.go` — DI composition root (manual wiring of all components)

**Acceptance Criteria:**
- [ ] Server starts, responds to requests, and shuts down gracefully on SIGTERM
- [ ] Request ID is generated and propagated in headers + logs
- [ ] Access logs include method, path, status, duration, request_id
- [ ] Panic in a handler does not crash the server; it returns 500
- [ ] Scope middleware extracts tenant/workspace and stores them in context
- [ ] main.go composes all components manually (no DI framework)

**Dependencies:** HT-02

---

### HT-18: Auth Middleware (OIDC + API Keys)

**Objective:** Implement dual authentication: OIDC JWT for the management plane, API Keys for the data plane.

**Spec:** §14.2 (Auth Middleware), §14.3 (RBAC Middleware)

**Deliverables:**
- `internal/http/middleware/auth.go` — `AuthMiddleware`
  - Detects auth type: `Bearer` → OIDC, `ApiKey` → API Key
  - OIDC: verifies JWT against discovery URL, extracts email → looks up member
  - API Key: hash → lookup → verify active + not expired
  - Sets auth context: member, role, scope, auth type
- `internal/http/middleware/rbac.go` — `RequireRole(minRole)`
  - Verifies that the member's role is >= minRole for the current scope
  - Hierarchy: superadmin > tenant_admin > workspace_admin > workspace_editor > workspace_viewer

**Acceptance Criteria:**
- [ ] Valid OIDC token → authenticated, member info in context
- [ ] Invalid OIDC token → 401
- [ ] Valid API Key → authenticated, workspace scope in context
- [ ] Revoked API Key → 401
- [ ] Missing auth header → 401
- [ ] RBAC: viewer cannot perform POST → 403
- [ ] RBAC: superadmin can access any scope
- [ ] Unit tests with OIDC mock and API key fixtures

**Dependencies:** HT-09 (member + apikey stores), HT-17

---

### HT-19: CRUD Handlers — Tenants, Workspaces, Members

**Objective:** Implement HTTP handlers for managing tenants, workspaces, and members.

**Spec:** §15.3 (Routes), §15.1 (Pagination), §15.2 (Error Response)

**Deliverables:**
- `internal/http/handler/tenant.go` — tenant CRUD (superadmin only)
- `internal/http/handler/workspace.go` — workspace CRUD (tenant_admin+)
- `internal/http/handler/member.go` — member CRUD + role assignment
- `internal/http/handler/config.go` — global config get/set (superadmin only)
- `internal/http/request/` — request DTOs with validation
- `internal/http/response/` — response DTOs (standardized format)
- Error response contract: `{"error": {"code": "...", "message": "...", "details": [...]}}`
- Cursor pagination: `?cursor=xxx&limit=50` → `{"items": [...], "next_cursor": "...", "has_more": true}`

**Acceptance Criteria:**
- [ ] All tenant/workspace/member/config endpoints from §15.3
- [ ] Pagination works with cursor
- [ ] Error responses follow the §15.2 contract
- [ ] Request body validation (required fields, formats)
- [ ] RBAC applied: only allowed roles can access endpoints
- [ ] Integration tests: HTTP request → response verification

**Dependencies:** HT-07, HT-17, HT-18

---

### HT-20: CRUD Handlers — Injectors, Adapters, Domains

**Objective:** Implement handlers for email configuration resources.

**Spec:** §15.3 (Routes for injectors, adapters, domains)

**Deliverables:**
- `internal/http/handler/injector.go` — injector definitions + fields + values CRUD
- `internal/http/handler/adapter.go` — adapter CRUD (encrypt credentials on create/update)
- `internal/http/handler/domain.go` — domain CRUD + verification trigger
- `internal/service/domain.go` — `DomainService`
  - Create: generates DKIM key pair, encrypts the private key, generates DNS records
  - Verify: enqueues a DomainVerify job
  - GetDNSRecords: returns records for the admin to configure

**Acceptance Criteria:**
- [ ] Injector CRUD respects scope (workspace_id context)
- [ ] Adapter create encrypts credentials; list does not expose credentials
- [ ] Domain create generates DKIM keys and DNS records
- [ ] Domain verify enqueues a job that runs DNS checks
- [ ] Security assertions: credentials are never returned in responses

**Dependencies:** HT-08, HT-14 (DKIM), HT-17, HT-18

---

### HT-21: CRUD Handlers — Templates, Versions, Locales

**Objective:** Implement handlers for the template system with versioning and i18n.

**Spec:** §15.3 (Template routes)

**Deliverables:**
- `internal/http/handler/template_type.go` — template type CRUD (with adapter_id assignment)
- `internal/http/handler/template.go` — template + version + locale CRUD
- `internal/service/template.go` — `TemplateService`
  - Create version (draft)
  - Publish version (validates MJML, archives previous published version)
  - Archive version
  - CRUD locales per version
- `internal/service/template_type.go` — `TemplateTypeService`
  - CRUD with variable_schema validation (JSON Schema format)
  - Assign/unassign adapter_id

**Acceptance Criteria:**
- [ ] Template type CRUD works, adapter assignment persists
- [ ] Version lifecycle: draft → published → archived
- [ ] Only one version can be published per template (constraint)
- [ ] Locales are CRUD within a version
- [ ] MJML preview endpoint compiles and returns HTML
- [ ] Tests: create type → template → version → locale → publish → verify

**Dependencies:** HT-08, HT-14 (MJML), HT-17, HT-18

---

### HT-22: Send Endpoint + Email Query + Tracking

**Objective:** Implement the send endpoint, email queries, and lifecycle events.

**Spec:** §15.3 (POST /send, GET /emails), §13 (SendService)

**Deliverables:**
- `internal/http/handler/send.go` — `POST /api/v1/send`
  - Auth: API Key only
  - Request: `{to, template_type, variables, locale?, from_email?, from_name?}`
  - Response: `{tracking_id, status}`
  - Calls `SendService.Send()`
- `internal/http/handler/email.go` — email query handlers
  - `GET /api/v1/emails` — list by workspace (pagination)
  - `GET /api/v1/emails/:id` — detail with events
  - `GET /api/v1/emails/:id/events` — lifecycle events
- `internal/http/handler/suppression.go` — suppression list management
  - `GET /api/v1/suppressions` — list (global + workspace)
  - `POST /api/v1/suppressions` — add manual suppression
  - `DELETE /api/v1/suppressions/:id` — remove
- `internal/http/handler/audit.go` — audit log queries
  - `GET /api/v1/audit-logs` — list by scope (pagination, filters)

**Acceptance Criteria:**
- [ ] POST /send: happy path returns tracking_id + 202
- [ ] POST /send: validation errors return 400 with details
- [ ] POST /send: suppressed → 422, no adapter → 422, no template → 404
- [ ] GET /emails: pagination, filters by status/date
- [ ] GET /emails/:id: includes events timeline
- [ ] Suppression CRUD works for global and workspace scope
- [ ] Audit logs queryable by scope, resource, actor
- [ ] E2E test: send → verify email created → verify event logged → verify audit logged

**Dependencies:** HT-15, HT-16, HT-17, HT-18

---

## E6 — Operations

### HT-23: Provider Event Ingestion (SES Webhooks)

**Objective:** Receive provider events (SES bounces, complaints, deliveries) and process them.

**Spec:** §19 (Provider Event Ingestion — 19.1 to 19.5)

**Deliverables:**
- `internal/http/handler/provider_webhook.go` — `POST /api/v1/webhooks/ses/inbound`
  - SNS message parsing (SubscriptionConfirmation + Notification)
  - SNS signature verification
  - Extract SES event → normalize to `ProviderEvent`
- `internal/service/event_processor.go` — `EventProcessor`
  - Update email status based on event type
  - Side effects:
    - Hard bounce → add to `suppression_global`
    - Complaint → add to `suppression_workspace`
    - Delivered/Opened → update email timestamps
  - Dispatch webhook events to workspace webhooks

**Acceptance Criteria:**
- [ ] SNS SubscriptionConfirmation is auto-confirmed
- [ ] SNS signature verification blocks invalid payloads
- [ ] SES bounce → email status updated + suppression entry created
- [ ] SES complaint → workspace suppression entry created
- [ ] SES delivery → email `delivered_at` updated
- [ ] SES open → email `opened_at` updated
- [ ] Events are dispatched to active webhooks
- [ ] Unknown provider events are logged but not processed (no error)
- [ ] Tests with SNS payload fixtures

**Dependencies:** HT-09, HT-16 (webhook worker), HT-17

---

### HT-24: Webhook System (Dispatch + CRUD)

**Objective:** Implement the webhook system to notify consumers about email events.

**Spec:** §16.3–16.4 (WebhookWorker + WebhookService), §15.3 (Routes)

**Deliverables:**
- `internal/http/handler/webhook.go` — webhook CRUD
  - Create: auto-generates the secret
  - List/Get/Update/Delete by workspace
  - Test endpoint: sends a ping webhook
- `internal/service/webhook.go` — `WebhookService`
  - `Dispatch(ctx, workspaceID, eventType, payload)` → enqueue webhook jobs
  - Filters webhooks by event-type subscription

**Acceptance Criteria:**
- [ ] Webhook CRUD works
- [ ] Secret is auto-generated on create (not returned in list responses)
- [ ] Dispatch enqueues one job per active webhook subscribed to the event
- [ ] WebhookWorker (from HT-16) delivers with a verifiable HMAC signature
- [ ] Test webhook endpoint sends a ping and reports the result
- [ ] Integration tests: create webhook → trigger event → verify delivery

**Dependencies:** HT-16, HT-17, HT-18

---

### HT-25: Onboarding Flow

**Objective:** Implement the first-use wizard: first OIDC login → superadmin → tenant + workspace.

**Spec:** §20 (Onboarding Flow)

**Deliverables:**
- `internal/service/onboarding.go` — `OnboardingService`
  - `Status(ctx)` → `{completed: bool, step: string}` (public, no auth)
  - `Setup(ctx, request)` — guard: `count(members) == 0`
    - Create member from OIDC claims
    - Assign superadmin role (scope=global)
    - Create first tenant + _system workspace
    - Set `onboarding.completed = true` in global_config
- `internal/http/handler/onboarding.go`
  - `GET /api/v1/onboarding/status` — public
  - `POST /api/v1/onboarding/setup` — requires OIDC auth, guard `count==0`

**Acceptance Criteria:**
- [ ] Status endpoint is public (no auth)
- [ ] Setup works only when there are no members (guard)
- [ ] Setup creates member + superadmin + tenant + _system workspace in one transaction
- [ ] Second call to setup → 409 Conflict
- [ ] After setup, status returns `completed=true`
- [ ] E2E test: fresh DB → status=false → setup → status=true → second setup fails

**Dependencies:** HT-07, HT-09, HT-17, HT-18

---

### HT-26: Observability (Metrics + Health + Logging)

**Objective:** Implement Prometheus `/metrics`, health checks, and global structured logging.

**Spec:** §21 (Observability — 21.1 to 21.3)

**Deliverables:**
- `internal/http/middleware/metrics.go` — Prometheus middleware
  - `http_requests_total{method, path, status}`
  - `http_request_duration_seconds{method, path}`
- `internal/http/handler/health.go` — `GET /healthz`
  - Checks: DB ping, River queue status
  - Response: `{status: "ok"|"degraded", checks: {db: "ok", queue: "ok"}}`
- `GET /metrics` — Prometheus handler (promhttp)
- Application metrics:
  - `senda_emails_sent_total{adapter, status}`
  - `senda_send_duration_seconds{adapter}`
  - `senda_queue_depth{job_type}`
  - `senda_provider_errors_total{adapter, error_type}`
  - `senda_bounce_rate{workspace}` (gauge)
- slog setup: JSON in prod, text in dev, configurable level

**Acceptance Criteria:**
- [ ] GET /metrics returns Prometheus-formatted metrics
- [ ] GET /healthz returns status with checks
- [ ] All metrics from §21.2 are recorded correctly
- [ ] Logs are structured JSON in production
- [ ] Request ID is propagated in all logs for a request
- [ ] Tests: verify metrics increment after operations

**Dependencies:** HT-17

---

### HT-27: API Keys Service + Management Endpoints

**Objective:** Implement API key generation, validation, and management.

**Spec:** §3.10 (Schema), §10.2 (APIKeyStore), §15.3 (Routes)

**Deliverables:**
- `internal/service/apikey.go` — `APIKeyService`
  - `Generate(ctx, workspaceID, name, createdBy)` → `{key: "senda_live_xxx...", id, key_hint}` (key visible only once)
  - Key format: `senda_live_` + 32 random hex chars
  - Storage: SHA-256 hash of the key, last 8 chars as hint
  - `Validate(ctx, rawKey)` → `(APIKey, error)`
  - `Revoke(ctx, keyID)` → soft revoke (set `revoked_at`)
  - `ListByWorkspace(ctx, workspaceID)` → list (no key, only hint)
- `internal/http/handler/apikey.go`
  - `POST /workspaces/:code/api-keys` — generate
  - `GET /workspaces/:code/api-keys` — list
  - `DELETE /workspaces/:code/api-keys/:id` — revoke

**Acceptance Criteria:**
- [ ] Generate returns the full key only once (never recoverable again)
- [ ] Key hash lookup works for auth
- [ ] Revoked key rejects auth
- [ ] List does not expose the key or hash (only hint)
- [ ] Tests: generate → validate → revoke → validate fails

**Dependencies:** HT-09, HT-17, HT-18

---

## Dependency Graph

```
HT-01 (Scaffolding)
├── HT-02 (Config)
│   ├── HT-03 (DB + Migrations)
│   │   ├── HT-13 (PG Cache + Rate Limiter)
│   │   └─ (all stores depend on HT-03)
│   ├── HT-04 (Encryption)
│   └── HT-17 (Echo Server)
│       ├── HT-18 (Auth Middleware)
│       │   ├── HT-19 (CRUD: Tenants/Workspaces/Members)
│       │   ├── HT-20 (CRUD: Injectors/Adapters/Domains)
│       │   ├── HT-21 (CRUD: Templates)
│       │   ├── HT-22 (Send Endpoint + Email Query)
│       │   ├── HT-24 (Webhooks)
│       │   ├── HT-25 (Onboarding)
│       │   └── HT-27 (API Keys)
│       ├── HT-23 (Provider Events)
│       └── HT-26 (Observability)
│
├── HT-05 (Domain Models) ──→ HT-06 (Ports)
│   ├── HT-07 (Stores: Base) ──→ HT-08 (Stores: Config) ──→ HT-09 (Stores: Rest)
│   └── HT-10 (ChainResolver + InjectorMerger)
│       └── HT-11 (TemplateResolver + AdapterResolver)
│           └── HT-12 (DomainResolver + Cache Invalidation)
│
├── HT-14 (MJML + DKIM)
│
└── HT-15 (SendService) ← depends on: HT-10..12, HT-13, HT-14
    └── HT-16 (River Workers) ← depends on: HT-13, HT-14, HT-15
```

---

## Suggested Implementation Order (4 parallelizable tracks)

### Track A — Infrastructure (1 dev)
```
HT-01 → HT-02 → HT-03 → HT-04 → HT-13 → HT-14
```
*~4 weeks*

### Track B — Domain + Resolution (1 dev, starts after HT-01)
```
HT-05 → HT-06 → HT-07 → HT-08 → HT-09 → HT-10 → HT-11 → HT-12
```
*~5 weeks*

### Track C — API Layer (1 dev, starts after HT-17+HT-18)
```
HT-17 → HT-18 → HT-19 → HT-20 → HT-21 → HT-27 → HT-25
```
*~5 weeks*

### Track D — Send Flow + Operations (1 dev, starts after dependencies are met)
```
HT-15 → HT-16 → HT-22 → HT-23 → HT-24 → HT-26
```
*~4 weeks*
