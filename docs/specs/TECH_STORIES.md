# Historias Técnicas (HTs) — Senda P1

**Referencia:** TECH_SPEC v1.4 | PRD v5.0

**Granularidad:** Feature-level (3–5 días por HT)

**Convenciones:**
- Cada HT incluye: objetivo, secciones del spec relevantes, criterios de aceptación, y dependencias.
- TDD obligatorio: cada HT produce tests unitarios. E2E con TestContainers donde aplique.
- Las HTs están agrupadas en **Épicas** y ordenadas por dependencia.

---

## Mapa de Épicas

| Épica | Scope | HTs |
|-------|-------|-----|
| E1 — Foundation | Scaffolding, DB, Config, Crypto | HT-01 a HT-04 |
| E2 — Core Domain | Modelos, Ports, Stores | HT-05 a HT-09 |
| E3 — Resolution Engine | Chain, Injectors, Templates, Adapters, Domains | HT-10 a HT-12 |
| E4 — Send Flow | SendService, Workers, DKIM, Rate Limiting | HT-13 a HT-16 |
| E5 — API Layer | Middleware, Auth, Handlers, Contract | HT-17 a HT-22 |
| E6 — Operations | Provider Events, Webhooks, Onboarding, Observability | HT-23 a HT-27 |

---

## E1 — Foundation

### HT-01: Project Scaffolding + Docker Compose

**Objetivo:** Crear la estructura de carpetas del proyecto Go, configurar el módulo, Docker Compose con PostgreSQL 16 + pg_cron, y Makefile base.

**Spec:** §9 (Estructura de Carpetas), §18 (Docker Compose)

**Entregables:**
- `go.mod` inicializado con módulo `github.com/rendis/senda`
- Estructura de carpetas: `cmd/senda/`, `internal/{domain,port,service,resolution,adapter,http}/`, `migrations/`, `config/`, `pkg/`, `docker/`
- `docker-compose.yml` con postgres:16-alpine + pg_cron (`shared_preload_libraries`), caddy (profile https)
- `Dockerfile` (multi-stage: build + alpine runtime)
- `Dockerfile.dev` (con hot reload via air o similar)
- `Makefile` con targets: `dev`, `build`, `test`, `migrate-up`, `migrate-down`, `lint`
- `.gitignore`, `.editorconfig`

**Criterios de Aceptación:**
- [ ] `make dev` levanta el stack completo (senda + postgres) con docker compose
- [ ] PostgreSQL acepta conexiones y pg_cron está habilitado (`SELECT * FROM cron.job` no falla)
- [ ] `make build` genera el binario
- [ ] `make test` ejecuta tests (aunque aún no haya ninguno)

**Dependencias:** Ninguna (primera HT)

---

### HT-02: Configuration + Secrets Management

**Objetivo:** Implementar carga de configuración desde YAML + env vars, con validación y defaults.

**Spec:** §17 (Configuration)

**Entregables:**
- `config/config.go` — struct `Config` con sub-configs: `ServerConfig`, `DatabaseConfig`, `OIDCConfig`, `CryptoConfig`, `LogConfig`
- Parsing YAML + override por env vars (`SENDA_` prefix)
- `config/config.example.yaml` con todos los valores documentados
- Validación: campos requeridos, formatos de URL, master key length

**Criterios de Aceptación:**
- [ ] Config se carga desde YAML, env vars override funcionan
- [ ] Errores claros si falta configuración requerida
- [ ] `config.example.yaml` incluye todos los campos con comentarios
- [ ] Tests unitarios para parsing, defaults, y validación

**Dependencias:** HT-01

---

### HT-03: Database Connection + Migrations Runner

**Objetivo:** Implementar pool de conexiones pgx v5, integrar golang-migrate, y crear todas las migrations SQL.

**Spec:** §7 (Migration Strategy completa — 19 migrations), §3 (Schema SQL), §4 (Partitioning), §5 (Índices)

**Entregables:**
- `internal/adapter/postgres/db.go` — connection pool (pgxpool), health check, graceful shutdown
- Integración golang-migrate: auto-run on start (configurable)
- **19 archivos de migration** (up + down):
  - 000001: Extensions (pgcrypto, pg_cron)
  - 000002: 10 ENUMs
  - 000003: tenants + workspaces (con CHECKs, EXCLUDE)
  - 000004: injectors (definitions, fields, values)
  - 000005: adapters (con EXCLUDE default, rate_limit_per_second)
  - 000006: domains (con domain_status, DKIM, dns_records)
  - 000007: template_types (con adapter_id) + templates
  - 000008: template_versions + locales (con EXCLUDE one-published)
  - 000009: members + member_roles (con scope_type, CHECK)
  - 000010: api_keys
  - 000011: emails + email_events (partitioned by month)
  - 000012: suppression lists
  - 000013: webhooks
  - 000014: audit_logs (partitioned)
  - 000015: global_config + seed data
  - 000016: UNLOGGED tables (cache, token_buckets)
  - 000017: PL/pgSQL functions (get_resolution_chain, take_send_token)
  - 000018: Performance indices
  - 000019: pg_cron jobs (cache-cleanup, create-partitions)

**Criterios de Aceptación:**
- [ ] `make migrate-up` ejecuta las 19 migrations sin error
- [ ] `make migrate-down` revierte todas las migrations
- [ ] Todas las tablas, constraints, ENUMs, funciones e índices existen
- [ ] pg_cron jobs registrados (`SELECT * FROM cron.job` muestra 2 jobs)
- [ ] Test de integración con TestContainers: migrate up → verify tables → migrate down → verify clean

**Dependencias:** HT-01, HT-02

---

### HT-04: Encryption Module (AES-256-GCM)

**Objetivo:** Implementar encriptación/decriptación simétrica para credentials de adapters y DKIM keys.

**Spec:** §1.7 (Encrypted at rest), §17 (CryptoConfig con master_key)

**Entregables:**
- `internal/adapter/crypto/aes.go` — `AESCrypto` struct implementando el port `Crypto`
- `internal/port/crypto.go` — interface `Crypto { Encrypt(plaintext []byte) ([]byte, error); Decrypt(ciphertext []byte) ([]byte, error) }`
- Derivación de key desde master key (HKDF o similar)
- Nonce único por operación (GCM incluye nonce en ciphertext)

**Criterios de Aceptación:**
- [ ] Encrypt → Decrypt round-trip preserva el plaintext
- [ ] Distinto ciphertext para mismo plaintext (nonce aleatorio)
- [ ] Error claro si master key es inválida o ciphertext corrupto
- [ ] Tests unitarios con vectores de prueba

**Dependencias:** HT-02

---

## E2 — Core Domain

### HT-05: Domain Models + Error Types

**Objetivo:** Definir todas las entidades del dominio como structs Go y los errores de dominio.

**Spec:** §11 (Domain Models completo — 11.1 a 11.11)

**Entregables:**
- `internal/domain/tenant.go` — Tenant, Workspace
- `internal/domain/injector.go` — InjectorDefinition, InjectorField, InjectorValue
- `internal/domain/adapter.go` — Adapter (con AdapterType, RateLimitPerSecond)
- `internal/domain/template.go` — TemplateType (con AdapterID), Template, TemplateVersion, TemplateVersionLocale
- `internal/domain/email.go` — Email (con TrackingID, BodyMJML, snapshots), EmailEvent, EmailStatus
- `internal/domain/member.go` — Member (con OIDC), MemberRole, Role, ScopeType
- `internal/domain/apikey.go` — APIKey (con KeyPrefix, KeyHint, RevokedAt)
- `internal/domain/domain_record.go` — Domain (con DomainStatus, DKIMSelector, DNSRecords)
- `internal/domain/suppression.go` — SuppressionGlobal, SuppressionWorkspace
- `internal/domain/webhook.go` — Webhook (con ConsecutiveFailures, DisabledAt)
- `internal/domain/audit.go` — AuditLog (con AuditAction, ScopeType, Changes, Metadata)
- `internal/domain/config.go` — GlobalConfig
- `internal/domain/addressing.go` — Address struct (TenantCode:WorkspaceCode:TemplateTypeSlug)
- `internal/domain/errors.go` — ErrNotFound, ErrConflict, ErrValidation, ErrForbidden, ErrNoAdapterConfigured, etc.
- `pkg/apperr/errors.go` — Application error types con HTTP status mapping
- `pkg/slug/slug.go` — Validación de slugs (regex + reserved words)
- `pkg/tracking/id.go` — Generación de tracking IDs

**Criterios de Aceptación:**
- [ ] Todas las entidades de §11 están definidas con los mismos campos y tipos
- [ ] Enums mapeados correctamente (EmailStatus, Role, ScopeType, etc.)
- [ ] Domain errors implementan `error` interface y son distinguibles con `errors.Is`
- [ ] Slug validation cubre el regex del CHECK constraint de la DB
- [ ] Tests unitarios para: slug validation, tracking ID generation, address parsing

**Dependencias:** HT-01

---

### HT-06: Port Interfaces (Contratos)

**Objetivo:** Definir todas las interfaces (ports) que conectan el domain con la infraestructura.

**Spec:** §10 (Port Interfaces — 10.1 a 10.5)

**Entregables:**
- `internal/port/email_sender.go` — `EmailSender` interface (SendEmail, con context + EmailMessage)
- `internal/port/store.go` — Todas las store interfaces:
  - `TenantStore` (Create, GetByID, GetByCode, List, Update, SoftDelete)
  - `WorkspaceStore` (Create, GetByID, GetByCode, ListByTenant, Update, SoftDelete, GetSystem)
  - `InjectorStore` (CRUD + GetByScope — resolución por workspace_id IN chain)
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
- `internal/port/queue.go` — `JobQueue` interface (Enqueue con job types: SendEmail, VerifyDomain, DeliverWebhook)
- `internal/port/template_compiler.go` — `TemplateCompiler` interface (CompileMJML → HTML)
- `internal/port/cache.go` — `Cache` interface (Get, Set, Delete, DeletePattern)
- Tipos auxiliares: `ListOptions` (cursor-based), `PageResult[T]` (items + next_cursor + has_more)

**Criterios de Aceptación:**
- [ ] Todas las interfaces de §10 definidas
- [ ] Cada método tiene context.Context como primer parámetro
- [ ] PageResult es genérico (`PageResult[T any]`)
- [ ] ListOptions soporta cursor + limit
- [ ] Compila sin errores (interfaces bien tipadas contra domain models)

**Dependencias:** HT-05

---

### HT-07: PostgreSQL Stores — Tenants, Workspaces, Config

**Objetivo:** Implementar las store interfaces para las entidades base usando pgx v5.

**Spec:** §10.2 (Store Ports), §3.2–3.3 (Schema tenants/workspaces), §3.15 (global_config)

**Entregables:**
- `internal/adapter/postgres/tenant_repo.go` — TenantStore implementation
- `internal/adapter/postgres/workspace_repo.go` — WorkspaceStore implementation
- `internal/adapter/postgres/global_config_repo.go` — GlobalConfigStore implementation
- Patrón base: `pgxpool.Pool` inyectado, queries con `pgx.NamedArgs`, scans con `pgx.CollectRows`
- Soft delete: queries filtran `WHERE deleted_at IS NULL` por defecto
- Cursor-based pagination implementada como helper reutilizable

**Criterios de Aceptación:**
- [ ] CRUD completo para tenants y workspaces
- [ ] GetByCode resuelve tenant+workspace por código
- [ ] Soft delete funciona (no borra físicamente)
- [ ] Cursor pagination produce resultados ordenados y estables
- [ ] GlobalConfig get/set funciona con JSONB
- [ ] Tests de integración con TestContainers (PG real)

**Dependencias:** HT-03, HT-05, HT-06

---

### HT-08: PostgreSQL Stores — Injectors, Adapters, Domains, Templates

**Objetivo:** Implementar stores para las entidades de configuración de envío.

**Spec:** §10.2, §3.4–3.8

**Entregables:**
- `internal/adapter/postgres/injector_repo.go` — InjectorStore (definitions + fields + values, con resolución por chain)
- `internal/adapter/postgres/adapter_repo.go` — AdapterStore (con encrypt/decrypt de credentials)
- `internal/adapter/postgres/domain_repo.go` — DomainStore (con búsqueda por chain, pending verification)
- `internal/adapter/postgres/template_repo.go` — TemplateTypeStore + TemplateStore + TemplateVersionStore + TemplateVersionLocaleStore
- Resolución por chain: queries usan `workspace_id IN (unnest(get_resolution_chain($1)))` o equivalent Go logic
- Version management: publish (con validación one-published), archive, draft

**Criterios de Aceptación:**
- [ ] Injector values se crean/resuelven por scope (workspace → _system → global)
- [ ] Adapter credentials se encriptan al guardar y decriptan al leer
- [ ] Template versions respetan el constraint one-published
- [ ] Domain lookup busca en la cadena de resolución
- [ ] Tests de integración: crear datos en distintos scopes y verificar resolución

**Dependencias:** HT-04, HT-07

---

### HT-09: PostgreSQL Stores — Members, API Keys, Emails, Audit, Suppression, Webhooks

**Objetivo:** Implementar las stores restantes para auth, tracking, y operaciones.

**Spec:** §10.2, §3.9–3.14

**Entregables:**
- `internal/adapter/postgres/member_repo.go` — MemberStore + MemberRoleStore (con scope_type check)
- `internal/adapter/postgres/apikey_repo.go` — APIKeyStore (hash lookup, revoke)
- `internal/adapter/postgres/email_repo.go` — EmailStore (insert, update status, query con pagination)
- `internal/adapter/postgres/suppression_repo.go` — SuppressionStore (check global + workspace, add, remove)
- `internal/adapter/postgres/audit_repo.go` — AuditLogStore (append-only, query by scope/resource/member)
- `internal/adapter/postgres/webhook_repo.go` — WebhookStore (CRUD, get active by workspace)
- `internal/adapter/postgres/email_event_repo.go` — EmailEventStore (append, list by email)

**Criterios de Aceptación:**
- [ ] Member roles respetan CHECK constraint (scope_type vs role)
- [ ] API key lookup por hash funciona en < 5ms
- [ ] Email insert incluye todas las columnas de §3.11
- [ ] Suppression check consulta global + workspace en una sola transacción
- [ ] Audit log es append-only (no update, no delete)
- [ ] Tests de integración con TestContainers

**Dependencias:** HT-07

---

## E3 — Resolution Engine

### HT-10: ChainResolver + InjectorMerger

**Objetivo:** Implementar la resolución jerárquica workspace → _system → global y el merge de injectors campo por campo.

**Spec:** §12.1 (ChainResolver), §12.2 (InjectorMerger)

**Entregables:**
- `internal/resolution/chain.go` — `ChainResolver`
  - `Resolve(ctx, tenantCode, workspaceCode)` → `ResolvedChain{Workspace, SystemWorkspace, IsGlobal}`
  - Usa cache (port.Cache) con TTL 5min, key `chain:{tenantCode}:{workspaceCode}`
- `internal/resolution/injector_merger.go` — `InjectorMerger`
  - `Merge(ctx, workspaceID, injectorNames)` → `map[string]map[string]any`
  - Field-by-field merge: workspace values override _system, _system overrides global
  - Si un campo no tiene valor en ningún nivel y es required → error

**Criterios de Aceptación:**
- [ ] ChainResolver retorna los 3 niveles correctos (workspace, _system, global)
- [ ] Cache hit evita query a DB
- [ ] InjectorMerger combina campos de 3 niveles correctamente
- [ ] Required fields sin valor producen error descriptivo
- [ ] Tests unitarios con mock stores y mock cache

**Dependencias:** HT-06 (ports), HT-08 (stores para integration tests)

---

### HT-11: TemplateResolver + AdapterResolver

**Objetivo:** Implementar resolución de template (versión publicada + locale) y adapter (por template type).

**Spec:** §12.3 (TemplateResolver), §12.4 (AdapterResolver)

**Entregables:**
- `internal/resolution/template_resolver.go` — `TemplateResolver`
  - `Resolve(ctx, workspaceID, templateTypeSlug, locale)` → `ResolvedTemplate{TemplateType, Template, Version, Locale, FinalSubject, FinalBody}`
  - Busca template en chain (workspace → _system → global)
  - Selecciona versión published
  - Aplica locale fallback (requested → default_locale → base)
- `internal/resolution/adapter_resolver.go` — `AdapterResolver`
  - `ResolveForTemplateType(ctx, templateType)` → `ResolvedAdapter`
  - Lee `templateType.AdapterID` → busca adapter → decrypt credentials
  - Si AdapterID es nil → `ErrNoAdapterConfigured` (422)

**Criterios de Aceptación:**
- [ ] Template se resuelve buscando en la cadena: primero workspace, luego _system, luego global
- [ ] Si no hay template publicado → error descriptivo
- [ ] Locale fallback funciona: es-CO → es → default
- [ ] Adapter se resuelve desde template_type.adapter_id (no por chain)
- [ ] Sin adapter_id → 422 con mensaje claro
- [ ] Tests unitarios con mocks; integration test con datos en 3 niveles

**Dependencias:** HT-10

---

### HT-12: DomainResolver + Cache Invalidation

**Objetivo:** Validar from_email contra dominios verificados y implementar estrategia de invalidación de cache.

**Spec:** §12.5 (DomainResolver), §12.6 (Cache Invalidation Strategy)

**Entregables:**
- `internal/resolution/domain_resolver.go` — `DomainResolver`
  - `Validate(ctx, workspaceID, fromEmail)` → `ResolvedDomain` o error
  - Extrae dominio del from_email, busca en chain
  - Solo acepta dominios con `status = verified`
- `internal/service/cache_invalidator.go` — `CacheInvalidator`
  - `InvalidateWorkspace(ctx, workspaceID)` — borra keys de ese workspace
  - `InvalidateAdapter(ctx, adapterID)` — borra keys de template types que usan ese adapter
  - `InvalidateTenantWorkspaces(ctx, tenantID)` — borra keys de todos los workspaces del tenant
  - `InvalidateGlobal(ctx)` — borra todas las keys de chain/template/adapter

**Criterios de Aceptación:**
- [ ] from_email con dominio no verificado → error 422
- [ ] Dominio verificado en _system es válido para workspaces del tenant
- [ ] CacheInvalidator borra patterns correctos
- [ ] Después de invalidar, la siguiente resolución consulta DB (no cache stale)
- [ ] Tests unitarios

**Dependencias:** HT-10, HT-11

---

## E4 — Send Flow

### HT-13: PG Cache + Token Bucket Rate Limiter

**Objetivo:** Implementar el adapter de cache con PG UNLOGGED table y el rate limiter de providers con token bucket PL/pgSQL.

**Spec:** §23 (PG Cache), §24 (Token Bucket)

**Entregables:**
- `internal/adapter/pgcache/client.go` — `PGCache` implementando `port.Cache`
  - Get: `SELECT value FROM cache WHERE key = $1 AND expires_at > now()`
  - Set: `INSERT ... ON CONFLICT (key) DO UPDATE`
  - Delete: `DELETE WHERE key = $1`
  - DeletePattern: `DELETE WHERE key LIKE $1`
  - StartCleanup goroutine (backup para pg_cron)
- `internal/adapter/postgres/rate_limiter.go` — `RateLimiter`
  - `TakeToken(ctx, adapterID) (bool, error)` — llama `SELECT take_send_token($1)`
  - `WaitForToken(ctx, adapterID, timeout)` — retry con backoff hasta obtener token
- Cache TTLs documentados: chain=5min, template=10min, adapter=10min, suppression=1min

**Criterios de Aceptación:**
- [ ] Cache Get/Set/Delete funciona con JSONB
- [ ] Cache entries expiran correctamente
- [ ] DeletePattern borra por prefijo
- [ ] TakeToken retorna true cuando hay tokens, false cuando bucket vacío
- [ ] Token bucket se auto-crea desde adapter.rate_limit_per_second
- [ ] Refill es proporcional al tiempo transcurrido
- [ ] Tests de integración con TestContainers

**Dependencias:** HT-03 (migrations incluyen UNLOGGED tables + PL/pgSQL)

---

### HT-14: MJML Compiler + DKIM Signer

**Objetivo:** Implementar compilación MJML → HTML y firma DKIM de emails.

**Spec:** §22 (DKIM Signing), §10.4 (TemplateCompiler port)

**Entregables:**
- `internal/adapter/mjml/compiler.go` — `MJMLCompiler` implementando `port.TemplateCompiler`
  - Usa gomjml (Go nativo) para convertir MJML → HTML responsive
  - Fallback: si gomjml falla (feature no soportado), retorna error descriptivo
- `internal/adapter/dkim/signer.go` — `DKIMSigner`
  - `GenerateKeyPair()` → (privateKey, publicKey, error) — RSA-2048
  - `Sign(message []byte, domain, selector string, privateKey []byte)` → signed message
  - Usa go-msgauth para firma DKIM
- `internal/adapter/dkim/dns.go` — helpers para generar DNS records (DKIM TXT, SPF, DMARC)

**Criterios de Aceptación:**
- [ ] MJML válido se compila a HTML responsive
- [ ] MJML inválido produce error descriptivo (no panic)
- [ ] DKIM sign + verify round-trip funciona
- [ ] DNS records generados son correctos para la config
- [ ] Tests unitarios con fixtures MJML

**Dependencias:** HT-01 (go.mod deps)

---

### HT-15: SendService — Orchestration Core

**Objetivo:** Implementar el servicio principal de envío que orquesta resolución, compilación, y encolado.

**Spec:** §13 (SendService — Flujo Principal)

**Entregables:**
- `internal/service/send.go` — `SendService`
  - `Send(ctx, request) → (trackingID, error)`
  - Flujo:
    1. Parse address (tenantCode:workspaceCode:templateTypeSlug)
    2. ChainResolver → resolve workspace chain
    3. TemplateResolver → resolve template + locale
    4. InjectorMerger → merge injector values
    5. Render variables into template (Go text/template)
    6. AdapterResolver → get adapter for template type
    7. DomainResolver → validate from_email domain
    8. Check suppression (global + workspace)
    9. Compile MJML → HTML
    10. Create email record (status=queued, with snapshots)
    11. Enqueue SendEmail job
    12. Return tracking ID
- Request DTO validation (required fields, email format)

**Criterios de Aceptación:**
- [ ] Happy path: send request → email record created → job enqueued → tracking ID returned
- [ ] Suppressed recipient → 422 con motivo
- [ ] No adapter configured → 422
- [ ] Unverified domain → 422
- [ ] Template not found → 404
- [ ] Invalid variables (schema mismatch) → 400
- [ ] Snapshots guardados (variables, injectors) para auditoría
- [ ] Tests unitarios con todos los resolvers mockeados
- [ ] Integration test con stack real: send → verify email record + job in queue

**Dependencias:** HT-10, HT-11, HT-12, HT-13, HT-14

---

### HT-16: River Workers (Send, Domain Verify, Webhook)

**Objetivo:** Implementar los background workers que procesan jobs de la cola River.

**Spec:** §16 (Background Workers — 16.1 a 16.4)

**Entregables:**
- `internal/adapter/river/client.go` — River client setup, job types registration
- `internal/adapter/river/send_worker.go` — `SendWorker`
  - Toma email record, decrypta adapter credentials
  - Rate limit check (TakeToken) — si no hay token, requeue con delay
  - DKIM sign the message
  - Send via EmailSender port (SES adapter)
  - Update email status (sent/failed)
  - Dispatch email event
  - Retry logic: max 3, exponential backoff
- `internal/adapter/river/verify_worker.go` — `DomainVerifyWorker`
  - DNS lookup para DKIM, SPF, DMARC records
  - Update domain status (pending → verified / error)
  - Schedule recheck
- `internal/adapter/river/webhook_worker.go` — `WebhookWorker`
  - HTTP POST con payload JSON
  - HMAC-SHA256 signature header (`X-Senda-Signature`)
  - Retry: max 5, exponential backoff
  - Track consecutive failures → auto-disable webhook after 10
- `internal/adapter/ses/adapter.go` — `SESAdapter` implementando `port.EmailSender`
  - AWS SDK v2, usa credentials decriptadas del adapter

**Criterios de Aceptación:**
- [ ] SendWorker: envía email via SES, actualiza status, registra event
- [ ] SendWorker: respeta rate limit (requeue si bucket vacío)
- [ ] SendWorker: DKIM firma el mensaje antes de enviar
- [ ] SendWorker: retry con backoff en caso de error transitorio
- [ ] DomainVerifyWorker: DNS check funciona, actualiza status
- [ ] WebhookWorker: HMAC signature es verificable por el receptor
- [ ] WebhookWorker: auto-disable después de 10 failures consecutivos
- [ ] SES adapter: envía email real (test con SES sandbox o mock)
- [ ] Tests unitarios para cada worker; integration test para send flow completo

**Dependencias:** HT-13, HT-14, HT-15

---

## E5 — API Layer

### HT-17: Echo v5 Server + Base Middleware

**Objetivo:** Configurar el servidor HTTP Echo v5 con middleware base y graceful shutdown.

**Spec:** §14 (Middleware Chain), §8.2 (DI en main.go)

**Entregables:**
- `internal/http/server.go` — Echo v5 setup, route registration, graceful shutdown
- `internal/http/middleware/requestid.go` — X-Request-ID generation/propagation
- `internal/http/middleware/logger.go` — Structured access logging con slog
- `internal/http/middleware/recovery.go` — Panic recovery con stack trace logging
- `internal/http/middleware/scope.go` — Extract tenant_code/workspace_code de URL params, set in context
- `cmd/senda/main.go` — DI composition root (wiring de todos los componentes)

**Criterios de Aceptación:**
- [ ] Server arranca, responde a requests, shutdown graceful en SIGTERM
- [ ] Request ID se genera y propaga en headers + logs
- [ ] Access logs incluyen method, path, status, duration, request_id
- [ ] Panic en handler no crashea el server, retorna 500
- [ ] Scope middleware extrae tenant/workspace y los pone en context
- [ ] main.go compone todos los componentes manualmente (sin framework DI)

**Dependencias:** HT-02

---

### HT-18: Auth Middleware (OIDC + API Keys)

**Objetivo:** Implementar autenticación dual: OIDC JWT para management plane, API Keys para data plane.

**Spec:** §14.2 (Auth Middleware), §14.3 (RBAC Middleware)

**Entregables:**
- `internal/http/middleware/auth.go` — `AuthMiddleware`
  - Detecta tipo de auth: `Bearer` → OIDC, `ApiKey` → API Key
  - OIDC: verifica JWT contra discovery URL, extrae email → busca member
  - API Key: hash → lookup → verify active + not expired
  - Set auth context: member, role, scope, auth type
- `internal/http/middleware/rbac.go` — `RequireRole(minRole)`
  - Verifica que el role del member sea >= minRole para el scope actual
  - Hierarchy: superadmin > tenant_admin > workspace_admin > workspace_editor > workspace_viewer

**Criterios de Aceptación:**
- [ ] OIDC token válido → authenticated, member info in context
- [ ] OIDC token inválido → 401
- [ ] API Key válida → authenticated, workspace scope in context
- [ ] API Key revocada → 401
- [ ] Missing auth header → 401
- [ ] RBAC: viewer no puede hacer POST → 403
- [ ] RBAC: superadmin puede acceder a cualquier scope
- [ ] Tests unitarios con OIDC mock y API key fixtures

**Dependencias:** HT-09 (member + apikey stores), HT-17

---

### HT-19: CRUD Handlers — Tenants, Workspaces, Members

**Objetivo:** Implementar handlers HTTP para gestión de tenants, workspaces, y members.

**Spec:** §15.3 (Routes), §15.1 (Pagination), §15.2 (Error Response)

**Entregables:**
- `internal/http/handler/tenant.go` — CRUD tenants (superadmin only)
- `internal/http/handler/workspace.go` — CRUD workspaces (tenant_admin+)
- `internal/http/handler/member.go` — CRUD members + role assignment
- `internal/http/handler/config.go` — Global config get/set (superadmin only)
- `internal/http/request/` — Request DTOs con validación
- `internal/http/response/` — Response DTOs (standardized format)
- Error response contract: `{"error": {"code": "...", "message": "...", "details": [...]}}`
- Cursor pagination: `?cursor=xxx&limit=50` → `{"items": [...], "next_cursor": "...", "has_more": true}`

**Criterios de Aceptación:**
- [ ] Todos los endpoints de tenants/workspaces/members/config de §15.3
- [ ] Pagination funciona con cursor
- [ ] Error responses siguen el contrato de §15.2
- [ ] Validación de request bodies (campos requeridos, formatos)
- [ ] RBAC aplicado: solo roles permitidos pueden acceder
- [ ] Tests de integración: HTTP request → response verification

**Dependencias:** HT-07, HT-17, HT-18

---

### HT-20: CRUD Handlers — Injectors, Adapters, Domains

**Objetivo:** Implementar handlers para recursos de configuración de envío.

**Spec:** §15.3 (Routes para injectors, adapters, domains)

**Entregables:**
- `internal/http/handler/injector.go` — CRUD injector definitions + fields + values
- `internal/http/handler/adapter.go` — CRUD adapters (con encrypt de credentials en create/update)
- `internal/http/handler/domain.go` — CRUD domains + trigger verification
- `internal/service/domain.go` — `DomainService`
  - Create: genera DKIM key pair, encripta private key, genera DNS records
  - Verify: enqueue DomainVerify job
  - GetDNSRecords: retorna records para que el admin configure

**Criterios de Aceptación:**
- [ ] Injector CRUD respeta scope (workspace_id context)
- [ ] Adapter create encripta credentials; list no expone credentials
- [ ] Domain create genera DKIM keys y DNS records
- [ ] Domain verify enqueue job que ejecuta DNS check
- [ ] Tests con assertions de seguridad: credentials nunca en response

**Dependencias:** HT-08, HT-14 (DKIM), HT-17, HT-18

---

### HT-21: CRUD Handlers — Templates, Versions, Locales

**Objetivo:** Implementar handlers para el sistema de templates con versionado y i18n.

**Spec:** §15.3 (Routes para templates)

**Entregables:**
- `internal/http/handler/template_type.go` — CRUD template types (con adapter_id assignment)
- `internal/http/handler/template.go` — CRUD templates + versions + locales
- `internal/service/template.go` — `TemplateService`
  - Create version (draft)
  - Publish version (validates MJML, archives previous published)
  - Archive version
  - CRUD locales per version
- `internal/service/template_type.go` — `TemplateTypeService`
  - CRUD con validación de variable_schema (JSON Schema format)
  - Assign/unassign adapter_id

**Criterios de Aceptación:**
- [ ] Template type CRUD funciona, adapter assignment persiste
- [ ] Version lifecycle: draft → published → archived
- [ ] Solo una versión published por template (constraint)
- [ ] Locales se CRUD dentro de una versión
- [ ] MJML preview endpoint: compila y retorna HTML
- [ ] Tests: crear type → template → version → locale → publish → verify

**Dependencias:** HT-08, HT-14 (MJML), HT-17, HT-18

---

### HT-22: Send Endpoint + Email Query + Tracking

**Objetivo:** Implementar el endpoint de envío, consulta de emails, y lifecycle events.

**Spec:** §15.3 (POST /send, GET /emails), §13 (SendService)

**Entregables:**
- `internal/http/handler/send.go` — `POST /api/v1/send`
  - Auth: API Key only
  - Request: `{to, template_type, variables, locale?, from_email?, from_name?}`
  - Response: `{tracking_id, status}`
  - Calls SendService.Send()
- `internal/http/handler/email.go` — Email query handlers
  - `GET /api/v1/emails` — list by workspace (pagination)
  - `GET /api/v1/emails/:id` — detail with events
  - `GET /api/v1/emails/:id/events` — lifecycle events
- `internal/http/handler/suppression.go` — Suppression list management
  - `GET /api/v1/suppressions` — list (global + workspace)
  - `POST /api/v1/suppressions` — add manual suppression
  - `DELETE /api/v1/suppressions/:id` — remove
- `internal/http/handler/audit.go` — Audit log queries
  - `GET /api/v1/audit-logs` — list by scope (pagination, filters)

**Criterios de Aceptación:**
- [ ] POST /send: happy path retorna tracking_id + 202
- [ ] POST /send: errores de validación retornan 400 con detalles
- [ ] POST /send: suppressed → 422, no adapter → 422, no template → 404
- [ ] GET /emails: pagination, filtros por status/date
- [ ] GET /emails/:id: incluye events timeline
- [ ] Suppression CRUD funciona para global y workspace scope
- [ ] Audit logs queryables por scope, resource, actor
- [ ] E2E test: send → check email created → check event logged → check audit logged

**Dependencias:** HT-15, HT-16, HT-17, HT-18

---

## E6 — Operations

### HT-23: Provider Event Ingestion (SES Webhooks)

**Objetivo:** Recibir eventos de providers (SES bounces, complaints, deliveries) y procesarlos.

**Spec:** §19 (Provider Event Ingestion — 19.1 a 19.5)

**Entregables:**
- `internal/http/handler/provider_webhook.go` — `POST /api/v1/webhooks/ses/inbound`
  - SNS message parsing (SubscriptionConfirmation + Notification)
  - SNS signature verification
  - Extract SES event → normalize to `ProviderEvent`
- `internal/service/event_processor.go` — `EventProcessor`
  - Update email status based on event type
  - Side effects:
    - Hard bounce → add to suppression_global
    - Complaint → add to suppression_workspace
    - Delivered/Opened → update email timestamps
  - Dispatch webhook events to workspace webhooks

**Criterios de Aceptación:**
- [ ] SNS SubscriptionConfirmation auto-confirmed
- [ ] SNS signature verification blocks invalid payloads
- [ ] SES bounce → email status updated + suppression entry created
- [ ] SES complaint → workspace suppression entry created
- [ ] SES delivery → email delivered_at updated
- [ ] SES open → email opened_at updated
- [ ] Events dispatched to active webhooks
- [ ] Unknown provider events logged but not processed (no error)
- [ ] Tests con SNS payload fixtures

**Dependencias:** HT-09, HT-16 (webhook worker), HT-17

---

### HT-24: Webhook System (Dispatch + CRUD)

**Objetivo:** Implementar el sistema de webhooks para notificar a consumidores sobre eventos de email.

**Spec:** §16.3–16.4 (WebhookWorker + WebhookService), §15.3 (Routes)

**Entregables:**
- `internal/http/handler/webhook.go` — CRUD webhooks
  - Create: genera secret automáticamente
  - List/Get/Update/Delete por workspace
  - Test endpoint: envía ping webhook
- `internal/service/webhook.go` — `WebhookService`
  - `Dispatch(ctx, workspaceID, eventType, payload)` → enqueue webhook jobs
  - Filtra webhooks por event type subscription

**Criterios de Aceptación:**
- [ ] CRUD webhooks funciona
- [ ] Secret auto-generated en create (no en response de list)
- [ ] Dispatch enqueue un job por cada webhook activo suscrito al evento
- [ ] WebhookWorker (de HT-16) entrega con firma HMAC verificable
- [ ] Test webhook endpoint envía ping y reporta resultado
- [ ] Tests de integración: create webhook → trigger event → verify delivery

**Dependencias:** HT-16, HT-17, HT-18

---

### HT-25: Onboarding Flow

**Objetivo:** Implementar el wizard de primer uso: primer login OIDC → superadmin → tenant + workspace.

**Spec:** §20 (Onboarding Flow)

**Entregables:**
- `internal/service/onboarding.go` — `OnboardingService`
  - `Status(ctx)` → `{completed: bool, step: string}` (public, no auth)
  - `Setup(ctx, request)` — guard: count(members) == 0
    - Create member from OIDC claims
    - Assign superadmin role (scope=global)
    - Create first tenant + _system workspace
    - Set `onboarding.completed = true` in global_config
- `internal/http/handler/onboarding.go`
  - `GET /api/v1/onboarding/status` — public
  - `POST /api/v1/onboarding/setup` — requires OIDC auth, guard count==0

**Criterios de Aceptación:**
- [ ] Status endpoint es público (no auth)
- [ ] Setup funciona solo cuando no hay members (guard)
- [ ] Setup crea member + superadmin + tenant + _system workspace en una transaction
- [ ] Segundo call a setup → 409 Conflict
- [ ] Después del setup, status retorna completed=true
- [ ] Test E2E: fresh DB → status=false → setup → status=true → second setup fails

**Dependencias:** HT-07, HT-09, HT-17, HT-18

---

### HT-26: Observability (Metrics + Health + Logging)

**Objetivo:** Implementar /metrics Prometheus, health check, y structured logging global.

**Spec:** §21 (Observability — 21.1 a 21.3)

**Entregables:**
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
- slog setup: JSON format en prod, text en dev, level configurable

**Criterios de Aceptación:**
- [ ] GET /metrics retorna métricas en formato Prometheus
- [ ] GET /healthz retorna status con checks
- [ ] Todas las métricas de §21.2 se registran correctamente
- [ ] Logs son structured JSON en producción
- [ ] Request ID se propaga en todos los logs de un request
- [ ] Tests: verify metrics increment after operations

**Dependencias:** HT-17

---

### HT-27: API Keys Service + Management Endpoints

**Objetivo:** Implementar generación, validación, y gestión de API keys.

**Spec:** §3.10 (Schema), §10.2 (APIKeyStore), §15.3 (Routes)

**Entregables:**
- `internal/service/apikey.go` — `APIKeyService`
  - `Generate(ctx, workspaceID, name, createdBy)` → `{key: "senda_live_xxx...", id, key_hint}` (key solo visible una vez)
  - Key format: `senda_live_` + 32 random hex chars
  - Storage: SHA-256 hash of key, last 8 chars as hint
  - `Validate(ctx, rawKey)` → `(APIKey, error)`
  - `Revoke(ctx, keyID)` → soft-revoke (set revoked_at)
  - `ListByWorkspace(ctx, workspaceID)` → list (sin key, solo hint)
- `internal/http/handler/apikey.go`
  - `POST /workspaces/:code/api-keys` — generate
  - `GET /workspaces/:code/api-keys` — list
  - `DELETE /workspaces/:code/api-keys/:id` — revoke

**Criterios de Aceptación:**
- [ ] Generate retorna key completa solo una vez (nunca más recuperable)
- [ ] Key hash lookup funciona para auth
- [ ] Revoked key rechaza auth
- [ ] List no expone key ni hash (solo hint)
- [ ] Tests: generate → validate → revoke → validate fails

**Dependencias:** HT-09, HT-17, HT-18

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

## Suggested Implementation Order (4 tracks parallelizables)

### Track A — Infrastructure (1 dev)
```
HT-01 → HT-02 → HT-03 → HT-04 → HT-13 → HT-14
```
*~4 semanas*

### Track B — Domain + Resolution (1 dev, starts after HT-01)
```
HT-05 → HT-06 → HT-07 → HT-08 → HT-09 → HT-10 → HT-11 → HT-12
```
*~5 semanas*

### Track C — API Layer (1 dev, starts after HT-17+HT-18)
```
HT-17 → HT-18 → HT-19 → HT-20 → HT-21 → HT-27 → HT-25
```
*~5 semanas*

### Track D — Send Flow + Operations (1 dev, starts after dependencies met)
```
HT-15 → HT-16 → HT-22 → HT-23 → HT-24 → HT-26
```
*~4 semanas*

### Timeline estimado (con 2 devs)

| Semana | Dev 1 | Dev 2 |
|--------|-------|-------|
| S1 | HT-01 (Scaffolding) | HT-05 (Domain Models) |
| S2 | HT-02 (Config) + HT-04 (Crypto) | HT-06 (Ports) |
| S3 | HT-03 (DB + Migrations) | HT-07 (Stores: Base) |
| S4 | HT-13 (Cache + Rate Limit) + HT-14 (MJML/DKIM) | HT-08 (Stores: Config) |
| S5 | HT-17 (Echo Server) | HT-09 (Stores: Rest) |
| S6 | HT-18 (Auth) | HT-10 (Chain + Injector Merger) |
| S7 | HT-19 (CRUD: Base) | HT-11 (Template + Adapter Resolver) |
| S8 | HT-20 (CRUD: Config) | HT-12 (Domain + Cache Invalidation) |
| S9 | HT-21 (CRUD: Templates) | HT-15 (SendService) |
| S10 | HT-25 (Onboarding) + HT-27 (API Keys) | HT-16 (Workers) |
| S11 | HT-26 (Observability) | HT-22 (Send Endpoint + Query) |
| S12 | HT-24 (Webhooks) | HT-23 (Provider Events) |

**Total estimado: ~12 semanas con 2 devs.**

---

## Checklist de Cobertura vs Tech Spec

| Sección Spec | HT que la cubre |
|-------------|-----------------|
| §1 Principios | Transversal |
| §3 Schema SQL | HT-03 (migrations) |
| §4 Partitioning | HT-03 |
| §5 Índices | HT-03 |
| §6 Validaciones App | HT-08, HT-11, HT-12 |
| §7 Migrations | HT-03 |
| §8 Arquitectura | HT-17 (server), HT-05/06 (domain/ports) |
| §9 Carpetas | HT-01 |
| §10 Port Interfaces | HT-06 |
| §11 Domain Models | HT-05 |
| §12 Resolution Engine | HT-10, HT-11, HT-12 |
| §13 SendService | HT-15 |
| §14 Middleware | HT-17, HT-18 |
| §15 API Contract | HT-19..22 |
| §16 Workers | HT-16 |
| §17 Config | HT-02 |
| §18 Docker Compose | HT-01 |
| §19 Provider Events | HT-23 |
| §20 Onboarding | HT-25 |
| §21 Observability | HT-26 |
| §22 DKIM | HT-14 |
| §23 PG Cache | HT-13 |
| §24 Rate Limiting | HT-13 |
