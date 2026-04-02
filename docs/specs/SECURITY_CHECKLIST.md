# Security Checklist — Senda P1

**Referencia:** TECH_SPEC v1.4 | OWASP Top 10 (2021)

---

## 1. Authentication and Authorization

### 1.1. OIDC / JWT

- [ ] JWT tokens validated against the provider discovery URL (not hardcoded)
- [ ] Verificar `iss`, `aud`, `exp`, `nbf` claims
- [ ] Reject expired tokens without exception
- [ ] Do not store JWT tokens in the backend (stateless validation)
- [ ] Store the OIDC client_secret encrypted in global_config (AES-256-GCM)
- [ ] Discovery URL must be HTTPS only

### 1.2. API Keys

- [ ] Keys generated with a CSPRNG (crypto/rand), at least 32 bytes of entropy
- [ ] Store only the SHA-256 hash in the DB (never the raw key)
- [ ] Full key visible only once (in the creation response)
- [ ] Immediate revocation (check revoked_at on every request)
- [ ] Rate limit per API key (future, not P1 — but design it to be extensible)
- [ ] Key prefix identifies the type: `senda_live_` (helps detect leaks)
- [ ] Audit log on creation and revocation

### 1.3. RBAC

- [ ] Roles validated against scope (DB CHECK constraint + app validation)
- [ ] Least privilege principle: viewers cannot mutate data
- [ ] Superadmin is the only global role — it cannot be self-assigned
- [ ] Verify scope on every request (do not rely only on the role)
- [ ] member_roles enforced in middleware, not in individual handlers

---

## 2. Sensitive Data

### 2.1. Encryption at Rest

- [ ] Adapter credentials encrypted with AES-256-GCM
- [ ] DKIM private keys encrypted with AES-256-GCM
- [ ] OIDC client_secret encrypted in global_config
- [ ] Master key loaded from env var (never in a config file or the repo)
- [ ] Master key at least 32 bytes
- [ ] Unique nonce per encryption operation (GCM generates a random nonce)
- [ ] Key derivation: HKDF or similar to derive sub-keys from the master key

### 2.2. Data in Transit

- [ ] HTTPS mandatory in production (TLS 1.2+) 
- [ ] Caddy as reverse proxy handles automatic TLS
- [ ] Outbound webhooks only to HTTPS URLs
- [ ] SES communication via AWS SDK (TLS by default)

### 2.3. Data in Logs

- [ ] Never log: credentials, API keys, email bodies, DKIM keys, master key
- [ ] Log safely: key_hint (last 8 chars), email hashes, adapter IDs
- [ ] slog with sanitization: use custom LogValuer for sensitive types
- [ ] Request/response logging does not include bodies in production

### 2.4. Data in Responses

- [ ] API never returns: decrypted credentials, key_hash, DKIM private keys
- [ ] Adapter list returns: name, type, is_default, workspace — no credentials
- [ ] API Key list returns: name, key_hint, created_at — no key or hash
- [ ] Error messages do not expose stack traces or SQL queries in production

---

## 3. Input Validation (OWASP A03:2021 — Injection)

### 3.1. SQL Injection

- [ ] Use pgx named parameters (`$1`, `$2`) — never concatenate SQL
- [ ] Do not use `fmt.Sprintf` to build queries
- [ ] Prepared statements for frequent queries
- [ ] Validate input types before the query layer

### 3.2. Request Validation

- [ ] Validate all request body inputs (required fields, types, lengths)
- [ ] Email format: validate with regex + MX record check (optional in P1)
- [ ] Slugs: regex `^[a-z][a-z0-9-]*$`, length 2-50, no reserved words
- [ ] UUIDs: validate format before using them in queries
- [ ] JSONB inputs (variable_schema, variables): validar contra JSON Schema
- [ ] Limit request body size (e.g. 1 MB max)
- [ ] Limit string lengths (subject: 998 chars per RFC 2822, body: 10 MB)

### 3.3. Path Traversal

- [ ] Tenant/workspace codes validated with regex in middleware (do not allow `../`, `..`, `/`)
- [ ] Do not use user input to build filesystem paths

### 3.4. XSS in Templates

- [ ] MJML compiles to HTML — rendered HTML comes from admin-controlled templates
- [ ] Injected variables in templates must be HTML-escaped by default
- [ ] Template variables do NOT allow raw HTML (unless field_type = 'html' and the admin explicitly configures it)

---

## 4. OWASP Top 10 Coverage

### A01:2021 — Broken Access Control

- [ ] RBAC middleware applied to all protected routes
- [ ] API keys scoped to a workspace — they cannot access other workspaces
- [ ] Tenant isolation: queries always filter by tenant/workspace context
- [ ] No IDOR: verify that the resource belongs to the request scope
- [ ] Onboarding endpoint protected by a guard (count==0)

### A02:2021 — Cryptographic Failures

- [ ] AES-256-GCM for encryption at rest (see §2.1)
- [ ] HTTPS in transit (see §2.2)
- [ ] Passwords/keys never in plaintext in the DB, logs, or responses
- [ ] UUIDs v7 (no secuenciales predecibles)

### A03:2021 — Injection

- [ ] SQL injection prevented with parametrized queries (see §3.1)
- [ ] No shell execution from user input
- [ ] MJML compilation in a sandbox (gomjml is pure Go, no shell)

### A04:2021 — Insecure Design

- [ ] Threat model: multi-tenant isolation is core to the design
- [ ] Immutable audit log by design (append-only, no UPDATE/DELETE)
- [ ] Soft delete prevents accidental data loss
- [ ] DKIM key rotation: design for regeneration without downtime

### A05:2021 — Security Misconfiguration

- [ ] Docker containers run as a non-root user
- [ ] PostgreSQL: unique credentials per environment, do not use defaults in production
- [ ] Example config does not contain real secrets
- [ ] pg_cron database_name configured explicitly
- [ ] CORS: configure allowed origins (no `*` in production)
- [ ] Remove debug endpoints in production

### A06:2021 — Vulnerable and Outdated Components

- [ ] Dependabot or Renovate for automatic updates
- [ ] `go mod tidy` + `govulncheck` in CI
- [ ] Audit dependencies quarterly (see audit in TECH_SPEC v1.4)
- [ ] All current dependencies verified as active (pgx v5, River, Echo v5, go-msgauth, gomjml, golang-migrate)

### A07:2021 — Identification and Authentication Failures

- [ ] API Key brute force: SHA-256 hash lookup (no timing side-channel)
- [ ] OIDC: validar issuer contra whitelist
- [ ] No credential stuffing concern (no passwords, only OIDC + API keys)
- [ ] Session management delegated to the OIDC provider

### A08:2021 — Software and Data Integrity Failures

- [ ] SNS webhook: verify the message signature before processing
- [ ] Webhook dispatch: sign with HMAC-SHA256 so the receiver can verify it
- [ ] Migrations checksummed by golang-migrate
- [ ] Docker images pinned by digest in production

### A09:2021 — Security Logging and Monitoring Failures

- [ ] Audit log for all write operations
- [ ] Structured logging (slog JSON) with correlation IDs (request_id)
- [ ] Log auth failures (401/403) with IP + user agent
- [ ] Prometheus metrics for: auth failures, error rates, bounce rates
- [ ] Alerting (futuro): bounce rate > 5%, complaint rate > 0.1%

### A10:2021 — Server-Side Request Forgery (SSRF)

- [ ] Webhook URLs: validar que son HTTPS + no son IPs privadas (10.x, 172.16.x, 192.168.x, 127.x, ::1)
- [ ] SNS subscription confirmation: only confirm URLs from the *.amazonaws.com domain
- [ ] Domain verification DNS lookup: no permite redireccion a internal hosts

---

## 5. HTTP Security Headers

```go
// Security headers middleware
e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        h := c.Response().Header()
        h.Set("X-Content-Type-Options", "nosniff")
        h.Set("X-Frame-Options", "DENY")
        h.Set("X-XSS-Protection", "0") // disabled, CSP handles this
        h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
        h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        return next(c)
    }
})
```

---

## 6. CORS Policy

```go
e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins:     cfg.Server.CORSAllowedOrigins, // configurable per env
    AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowHeaders:     []string{"Authorization", "Content-Type", "X-Request-ID"},
    ExposeHeaders:    []string{"X-Request-ID", "X-Total-Count"},
    AllowCredentials: true,
    MaxAge:           3600,
}))
```

---

## 7. Rate Limiting (API)

Not implemented in P1 for API consumption (architectural decision). Consider for P2:
- [ ] Per-workspace rate limit (leaky bucket or token bucket)
- [ ] Per-IP rate limit for public endpoints (onboarding status, health)
- [ ] 429 Too Many Requests with `Retry-After` header

---

## 8. Multi-Tenancy Isolation

- [ ] Queries ALWAYS filter by tenant_id and/or workspace_id from the authenticated context
- [ ] API keys only access their workspace (enforced in middleware, not in the handler)
- [ ] OIDC members: access determined by member_roles (scope check)
- [ ] No cross-tenant data leaks in pagination (cursor includes workspace_id)
- [ ] Audit logs segregated by scope
- [ ] Global suppression is cross-tenant by design (hard bounces affect everyone)

---

## 9. Secrets Management in Production

| Secret | Where | How |
|--------|-------|------|
| Master key (AES) | Env var `SENDA_MASTER_KEY` | 32+ bytes, rotatable |
| DB password | Env var `SENDA_DATABASE_URL` | Managed PG credentials |
| OIDC client secret | Encrypted in global_config | AES-256-GCM |
| Adapter credentials | Encrypted in adapters.config_encrypted | AES-256-GCM |
| DKIM private keys | Encrypted in domains.dkim_private_key_encrypted | AES-256-GCM |
| Webhook secrets | Plain in webhooks.secret | Generated per webhook |
| API Keys | SHA-256 hash in `api_keys.key_hash` | Never stored raw |

**Master key rotation:**
- Requires re-encrypting all credentials and DKIM keys
- Implement as a migration or admin command (future)
- Support dual master keys temporarily during rotation

---

## 10. Pre-Release Checklist

- [ ] All dependencies scanned with `govulncheck`
- [ ] Example config does not contain real secrets
- [ ] Docker image runs as non-root
- [ ] HTTPS configured (Caddy or load balancer)
- [ ] CORS origins configured (no `*`)
- [ ] Logs in JSON, INFO level in production
- [ ] Master key generated with `openssl rand -hex 32`
- [ ] DB password is not the default
- [ ] SNS webhook endpoint protected with signature verification
- [ ] Audit logging enabled
- [ ] Health check responds
- [ ] Metrics endpoint accessible (but not public)
