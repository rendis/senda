# Session Handoff — Cycle 3

## Estado global

- **NO está cerrado el programa.**
- Cycle 1: cerrado.
- Cycle 2: cerrado.
- Cycle 3: **abierto**.

Última re-auditoría sobre `main` integrado:

- seguridad: **7.1/10**
- arquitectura: **8.2/10**
- performance: **8.3/10**
- maintainability/DX: **8.2/10**

## Lo ya resuelto en esta sesión

### Seguridad / perímetro

- OIDC management auth ya resuelve miembros por **`issuer + subject`**, no por email.
- `server.metrics_token` ya es obligatorio en producción.
- El logout federado ya no confía en `Host`; usa `AUTH_URL` o `request.nextUrl.origin`.
- El webhook SNS ya es **default-deny** cuando no hay binding/política configurada.
- El validador SES/SNS ya usa **probes únicos no destructivos**.

### DX / cache invalidation

- `InvalidateTenantWorkspaces` ya recorre **todas las páginas** por cursor.

## Lo que queda pendiente

### 1) `perimeter-identity-default-deny`

Estado actual: **in_progress**

Pendiente real:

- rematar el hot path de `/public/video-thumbnail`
  - evitar cliente/transporte nuevo por request
  - reducir copia innecesaria en cache hit
  - mantener SSRF pinning + allowlist + redirect validation + redaction
- correr verify final del stream
- actualizar `verify-report.md`, `status.md`, `state.yaml`

Pistas concretas:

- archivo principal: `internal/http/handler/media.go`
- hoy sigue haciendo:
  - `session := h.newFetchSession()` por request
  - `session.client()` crea `Transport` nuevo
  - `thumbnailCache.Get()` devuelve `append([]byte(nil), entry.value...)`

### 2) `ci-drift-and-cache-pagination`

Estado actual: **in_progress**

Pendiente real:

- ampliar `scripts/ci-taxonomy-check.mjs` para cubrir:
  - `.github/pull_request_template.md`
  - `AGENTS.md`
- correr verify final del stream
- actualizar `verify-report.md`, `status.md`, `state.yaml`

### 3) `sdk-and-http-composition-decoupling`

Estado actual: **planned**

Pendiente real:

- desacoplar el SDK del dominio interno
- revisar:
  - `sdk/environment.go`
  - `sdk/interfaces.go`
  - `sdk/types.go`
- adelgazar composición / wiring HTTP
- revisar:
  - `internal/app/bootstrap.go`
  - `internal/app/http_surfaces.go`
  - `internal/http/server.go`

### 4) `send-context-and-media-hotpath`

Estado actual: **planned**

Pendiente real:

- converger el contexto de envío a una sola fuente de verdad
- revisar:
  - `internal/service/send.go`
  - `internal/service/send_batch.go`
  - `internal/service/send_context.go`
- cerrar optimización del hot path de media/cache/transport
- dejar benchmarks o evidencia de mejora real antes de cierre

## Orden recomendado para retomar

1. terminar `perimeter-identity-default-deny`
2. terminar `ci-drift-and-cache-pagination`
3. ejecutar `sdk-and-http-composition-decoupling`
4. ejecutar `send-context-and-media-hotpath`
5. correr verify final del ciclo 3
6. correr re-auditoría ciega nueva
7. si cualquier eje queda `<= 8.5`, abrir cycle 4

## Verificación ya verde en esta sesión

```bash
go test ./internal/http/middleware ./internal/http/handler ./config ./internal/adapter/ses
go test -tags=integration ./internal/adapter/postgres -run TestMemberRepo_GetByOIDCIdentity
pnpm --dir web exec node --test src/app/api/auth/federated-logout-url/logout-url.test.ts src/app/api/auth/federated-logout-url/origin.test.ts
go test ./internal/resolution -run TestCacheInvalidator_InvalidateTenantWorkspaces
```

## Regla de cierre

No declarar “terminado” hasta que se cumpla TODO:

- todos los streams de cycle 3 en `done/approved`
- verify final de cycle 3
- re-auditoría nueva ejecutada
- todos los ejes **> 8.5**
- sin cambios abiertos nuevos
