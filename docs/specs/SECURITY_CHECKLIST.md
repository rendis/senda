# Security Checklist — Senda P1

**Referencia:** TECH_SPEC v1.4 | OWASP Top 10 (2021)

---

## 1. Autenticación y Autorización

### 1.1. OIDC / JWT

- [ ] Tokens JWT validados contra discovery URL del provider (no hardcodeado)
- [ ] Verificar `iss`, `aud`, `exp`, `nbf` claims
- [ ] Rechazar tokens expirados sin excepción
- [ ] No almacenar tokens JWT en backend (stateless validation)
- [ ] OIDC client_secret almacenado encriptado en global_config (AES-256-GCM)
- [ ] Discovery URL solo HTTPS

### 1.2. API Keys

- [ ] Keys generadas con CSPRNG (crypto/rand), mínimo 32 bytes de entropía
- [ ] Almacenar solo SHA-256 hash en DB (nunca el key raw)
- [ ] Key completa visible una sola vez (en response de creación)
- [ ] Revocación inmediata (check revoked_at en cada request)
- [ ] Rate limit por API Key (futuro, no P1 — pero diseñar extensible)
- [ ] Key prefix identifica tipo: `senda_live_` (facilita detección en leaks)
- [ ] Audit log en creación y revocación

### 1.3. RBAC

- [ ] Roles validados contra scope (CHECK constraint en DB + app validation)
- [ ] Principio de mínimo privilegio: viewers no pueden mutar
- [ ] Superadmin es el único rol global — no se puede autoasignar
- [ ] Verificar scope en cada request (no confiar solo en el rol)
- [ ] member_roles enforced en middleware, no en handlers individuales

---

## 2. Datos Sensibles

### 2.1. Encryption at Rest

- [ ] Credenciales de adapters encriptadas con AES-256-GCM
- [ ] DKIM private keys encriptadas con AES-256-GCM
- [ ] OIDC client_secret encriptado en global_config
- [ ] Master key cargada desde env var (nunca en config file ni repo)
- [ ] Master key mínimo 32 bytes
- [ ] Nonce único por operación de encriptación (GCM genera nonce aleatorio)
- [ ] Key derivation: HKDF o similar para derivar sub-keys del master key

### 2.2. Datos en Tránsito

- [ ] HTTPS obligatorio en producción (TLS 1.2+)
- [ ] Caddy como reverse proxy maneja TLS automático
- [ ] Webhooks salientes solo a HTTPS URLs
- [ ] SES communication via AWS SDK (TLS by default)

### 2.3. Datos en Logs

- [ ] Nunca loguear: credentials, API keys, email bodies, DKIM keys, master key
- [ ] Loguear de forma segura: key_hint (últimos 8 chars), email hashes, adapter IDs
- [ ] slog con sanitization: usar custom LogValuer para tipos sensibles
- [ ] Request/response logging no incluye body en producción

### 2.4. Datos en Responses

- [ ] API nunca retorna: credentials decriptadas, key_hash, DKIM private keys
- [ ] Adapter list retorna: name, type, is_default, workspace — no credentials
- [ ] API Key list retorna: name, key_hint, created_at — no key ni hash
- [ ] Error messages no exponen stack traces ni SQL queries en producción

---

## 3. Input Validation (OWASP A03:2021 — Injection)

### 3.1. SQL Injection

- [ ] Usar pgx named parameters (`$1`, `$2`) — nunca concatenar SQL
- [ ] No usar `fmt.Sprintf` para construir queries
- [ ] Prepared statements para queries frecuentes
- [ ] Validar tipos de input antes de llegar a la query

### 3.2. Request Validation

- [ ] Validar todos los inputs del request body (campos requeridos, tipos, longitudes)
- [ ] Email format: validar con regex + MX record check (opcional en P1)
- [ ] Slugs: regex `^[a-z][a-z0-9-]*$`, longitud 2-50, sin reserved words
- [ ] UUIDs: validar formato antes de usar en queries
- [ ] JSONB inputs (variable_schema, variables): validar contra JSON Schema
- [ ] Limitar tamaño de request body (ej: 1MB max)
- [ ] Limitar longitud de strings (subject: 998 chars per RFC 2822, body: 10MB)

### 3.3. Path Traversal

- [ ] Tenant/workspace codes validados con regex en middleware (no permitir `../`, `..`, `/`)
- [ ] No usar input del usuario para construir paths de filesystem

### 3.4. XSS en Templates

- [ ] MJML se compila a HTML — el HTML renderizado es de templates controlados por admins
- [ ] Variables inyectadas en templates deben ser HTML-escaped por defecto
- [ ] Template variables NO permiten HTML raw (a menos que field_type = 'html' y admin lo configure explícitamente)

---

## 4. OWASP Top 10 Coverage

### A01:2021 — Broken Access Control

- [ ] RBAC middleware aplicado en todas las rutas protegidas
- [ ] API Keys scoped a workspace — no pueden acceder a otros workspaces
- [ ] Tenant isolation: queries siempre filtran por tenant/workspace context
- [ ] No IDOR: verificar que el recurso pertenece al scope del request
- [ ] Onboarding endpoint protegido por guard (count==0)

### A02:2021 — Cryptographic Failures

- [ ] AES-256-GCM para encryption at rest (ver §2.1)
- [ ] HTTPS en tránsito (ver §2.2)
- [ ] Passwords/keys nunca en plaintext en DB, logs, o responses
- [ ] UUIDs v7 (no secuenciales predecibles)

### A03:2021 — Injection

- [ ] SQL injection prevenido con parametrized queries (ver §3.1)
- [ ] No shell execution desde input del usuario
- [ ] MJML compilation en sandbox (gomjml es Go puro, no shell)

### A04:2021 — Insecure Design

- [ ] Threat model: multi-tenant isolation es core del diseño
- [ ] Audit log inmutable por diseño (append-only, no UPDATE/DELETE)
- [ ] Soft delete previene pérdida accidental de datos
- [ ] DKIM key rotation: diseñar para re-generación sin downtime

### A05:2021 — Security Misconfiguration

- [ ] Docker containers corren como non-root user
- [ ] PostgreSQL: credenciales únicas por entorno, no usar defaults en producción
- [ ] Config ejemplo no contiene secrets reales
- [ ] pg_cron database_name configurado explícitamente
- [ ] CORS: configurar allowed origins (no `*` en producción)
- [ ] Remove debug endpoints en producción

### A06:2021 — Vulnerable and Outdated Components

- [ ] Dependabot o Renovate para actualizaciones automáticas
- [ ] `go mod tidy` + `govulncheck` en CI
- [ ] Auditar dependencias trimestralmente (ver audit en TECH_SPEC v1.4)
- [ ] Todas las dependencias actuales verificadas como activas (pgx v5, River, Echo v5, go-msgauth, gomjml, golang-migrate)

### A07:2021 — Identification and Authentication Failures

- [ ] API Key brute force: SHA-256 hash lookup (no timing side-channel)
- [ ] OIDC: validar issuer contra whitelist
- [ ] No credential stuffing concern (no passwords, solo OIDC + API keys)
- [ ] Session management delegada al OIDC provider

### A08:2021 — Software and Data Integrity Failures

- [ ] SNS webhook: verificar firma del mensaje antes de procesar
- [ ] Webhook dispatch: firmar con HMAC-SHA256 para que receptor verifique
- [ ] Migrations checksummed por golang-migrate
- [ ] Docker images con digest pinning en producción

### A09:2021 — Security Logging and Monitoring Failures

- [ ] Audit log para todas las operaciones de escritura
- [ ] Structured logging (slog JSON) con correlation IDs (request_id)
- [ ] Log auth failures (401/403) con IP + user agent
- [ ] Prometheus metrics para: auth failures, error rates, bounce rates
- [ ] Alerting (futuro): bounce rate > 5%, complaint rate > 0.1%

### A10:2021 — Server-Side Request Forgery (SSRF)

- [ ] Webhook URLs: validar que son HTTPS + no son IPs privadas (10.x, 172.16.x, 192.168.x, 127.x, ::1)
- [ ] SNS subscription confirmation: solo confirmar URLs de dominio *.amazonaws.com
- [ ] Domain verification DNS lookup: no permite redireccion a internal hosts

---

## 5. Headers HTTP de Seguridad

```go
// Middleware de seguridad headers
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

No implementado en P1 para consumo de la API (decisión arquitectónica). Para P2 considerar:
- [ ] Per-workspace rate limit (leaky bucket o token bucket)
- [ ] Per-IP rate limit para endpoints públicos (onboarding status, health)
- [ ] 429 Too Many Requests con `Retry-After` header

---

## 8. Multi-Tenancy Isolation

- [ ] Queries SIEMPRE filtran por tenant_id y/o workspace_id del contexto autenticado
- [ ] API Keys solo acceden a su workspace (enforced en middleware, no en handler)
- [ ] OIDC members: acceso determinado por member_roles (scope check)
- [ ] No cross-tenant data leaks en pagination (cursor incluye workspace_id)
- [ ] Audit logs segregados por scope
- [ ] Suppression global es cross-tenant por diseño (hard bounces afectan a todos)

---

## 9. Secrets Management en Producción

| Secret | Dónde | Cómo |
|--------|-------|------|
| Master key (AES) | Env var `SENDA_MASTER_KEY` | 32+ bytes, rotable |
| DB password | Env var `SENDA_DATABASE_URL` | Managed PG credentials |
| OIDC client secret | Encrypted en global_config | AES-256-GCM |
| Adapter credentials | Encrypted en adapters.config_encrypted | AES-256-GCM |
| DKIM private keys | Encrypted en domains.dkim_private_key_encrypted | AES-256-GCM |
| Webhook secrets | Plain en webhooks.secret | Generated per webhook |
| API Keys | SHA-256 hash en api_keys.key_hash | Nunca almacenado raw |

**Rotación del master key:**
- Requiere re-encriptar todas las credentials y DKIM keys
- Implementar como migration o admin command (futuro)
- Soportar dual master key temporalmente durante rotación

---

## 10. Checklist Pre-Release

- [ ] Todas las dependencias escaneadas con `govulncheck`
- [ ] Config ejemplo no contiene secrets reales
- [ ] Docker image corre como non-root
- [ ] HTTPS configurado (Caddy o load balancer)
- [ ] CORS origins configurados (no `*`)
- [ ] Logs en JSON, nivel INFO en producción
- [ ] Master key generada con `openssl rand -hex 32`
- [ ] DB password no es default
- [ ] SNS webhook endpoint protegido con signature verification
- [ ] Audit logging activo
- [ ] Health check responde
- [ ] Metrics endpoint accesible (pero no público)
