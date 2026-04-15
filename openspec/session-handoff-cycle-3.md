# Session Handoff — Cycle 3

## Estado global

- **NO está cerrado el programa.**
- Cycle 1: cerrado.
- Cycle 2: cerrado.
- Cycle 3: **abierto**.
- Punto operativo para retomar: trabajar sobre `main`. En este cierre documental no hay worktrees activos de cycle 3 presentes localmente.

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
- El follow-up `media-thumbnail-hotpath-optimization` ya quedó **cerrado** en `main`:
  - reuse de fetch client por request
  - sin validación duplicada del URL inicial en el primer candidato
  - sin copia extra en cache hit
  - verify focalizado documentado

### DX / cache invalidation

- `InvalidateTenantWorkspaces` ya recorre **todas las páginas** por cursor.
- `ci-drift-and-cache-pagination` ya quedó **cerrado**; no hay trabajo DX pendiente en ese stream.

## Lo que queda pendiente

### 1) `perimeter-identity-default-deny`

Estado actual: **in_progress**

Pendiente real:

- resolver el signoff de policy sobre si el fallback por email para miembros **unbound** se acepta como transición explícita o migra luego a un modelo más estricto
- mantener el stream como **policy/documentation only** salvo que aparezca un hallazgo nuevo; el follow-up de media YA quedó cerrado
- actualizar verify/cierre final solo cuando exista esa decisión de policy

Pistas concretas:

- archivo principal: `internal/http/middleware/auth.go`
- soporte documental/contexto:
  - `internal/http/handler/member.go`
  - `internal/service/onboarding.go`

### 2) `sdk-and-http-composition-decoupling`

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

### 3) `send-context-and-media-hotpath`

Estado actual: **planned**

Pendiente real:

- converger el contexto de envío a una sola fuente de verdad
- revisar:
  - `internal/service/send.go`
  - `internal/service/send_batch.go`
  - `internal/service/send_context.go`
- NO reabrir el hot path de media salvo que aparezca una regresión nueva; ese follow-up ya quedó cerrado en `media-thumbnail-hotpath-optimization`

## Orden recomendado para retomar

1. resolver `perimeter-identity-default-deny` (decisión de policy)
2. ejecutar `sdk-and-http-composition-decoupling`
3. ejecutar `send-context-and-media-hotpath` (solo convergencia de send context)
4. correr verify final del ciclo 3
5. correr re-auditoría ciega nueva
6. si cualquier eje queda `<= 8.5`, abrir cycle 4

## Verificación ya verde en esta sesión

```bash
go test ./internal/http/middleware ./internal/http/handler ./config ./internal/adapter/ses
go test -tags=integration ./internal/adapter/postgres -run TestMemberRepo_GetByOIDCIdentity
pnpm --dir web exec node --test src/app/api/auth/federated-logout-url/logout-url.test.ts src/app/api/auth/federated-logout-url/origin.test.ts
go test ./internal/resolution -run TestCacheInvalidator_InvalidateTenantWorkspaces
go test ./internal/http/handler -run 'TestHandleVideoThumbnail_(CacheHit_PreservesHeadersAndBody|ConcurrentCacheHits_PreserveHeadersAndBody|Pinning_IsScopedPerRequest|InvalidOrOversizedImage_RemainsBadGateway)$'
go test -count=1 ./internal/http/handler -run 'TestHandleVideoThumbnail'
go test -race ./internal/http/handler -run 'TestHandleVideoThumbnail_(ConcurrentCacheHits_PreserveHeadersAndBody|ConcurrentSameURL|Pinning_IsScopedPerRequest)$'
```

## Regla de cierre

No declarar “terminado” hasta que se cumpla TODO:

- todos los streams de cycle 3 en `done/approved`
- verify final de cycle 3
- re-auditoría nueva ejecutada
- todos los ejes **> 8.5**
- sin cambios abiertos nuevos
