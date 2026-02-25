# P0 Backend Closure Report (2026-02-25)

## Alcance cerrado

- Endurecimiento funcional de `POST /api/v1/send`:
  - Scope API key vs `ref` (`403 FORBIDDEN`).
  - Bloqueo explícito de workspace `_system` (`422 SYSTEM_WORKSPACE_BLOCKED`).
  - Límite `to` máximo 50 destinatarios (`422 VALIDATION_ERROR`).
  - Template disabled mapeado a `409 TEMPLATE_DISABLED`.
  - No default identity mapeado a `422 NO_DEFAULT_IDENTITY`.
  - Rate limit activo en servicio (`429 RATE_LIMITED`).
- Data-plane por API key:
  - `GET /api/v1/emails`
  - `GET /api/v1/emails/:tracking_id`
  - `GET /api/v1/emails/:tracking_id/events`
  - `GET /api/v1/emails/export` (CSV streaming)
- Kill switch templates (management):
  - Workspace:
    - `POST /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/templates/:template_id/disable`
    - `POST /api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/templates/:template_id/enable`
  - Global:
    - `POST /api/v1/manage/global/templates/:template_id/disable`
    - `POST /api/v1/manage/global/templates/:template_id/enable`
- Identity provider wiring:
  - `DefaultIdentityProviderFactory` con providers SES/Gmail reales.
  - Fallback explícito para adapters sin identity listing.
- Integración/migraciones:
  - Ajuste de test de migraciones para esquema final sin `domains`.
  - Harness TestContainers de postgres compartido por paquete (reseteo determinístico por test).
- QA/E2E:
  - Suite determinística `core/crud/error/happy/security` release-blocking.
  - Suite `chaos` separada como no bloqueante.

## Frontend + Operación CI (cierre operativo de 5 puntos)

- Frontend lint estricto: `npm --prefix web run lint -- --max-warnings=0` ✅
- Frontend build producción: `npm --prefix web run build` ✅
- Workflows CI creados:
  - `.github/workflows/backend-gate.yml`
  - `.github/workflows/frontend-gate.yml`
  - `.github/workflows/chaos-e2e.yml`

## Artefactos de trazabilidad

- ADR: `docs/specs/ADR-0001-provider-managed-email-auth.md`
- Matriz R-01..R-23: `docs/specs/P0_BACKEND_REQUIREMENTS_MATRIX.md`
- QA reportes actualizados:
  - `docs/qa/E2E_REPORT.md`
  - `docs/qa/SECURITY_FINDINGS.md`
  - `docs/qa/COVERAGE_MAP.md`

## Gates ejecutados en este cierre

- `make test` ✅
- `make test-integration` ✅
- `make lint` ✅
- `go vet ./...` ✅
- `make test-e2e` ✅
- `make test-e2e-chaos` ✅
- `npm --prefix web run lint -- --max-warnings=0` ✅
- `npm --prefix web run build` ✅

## Nota de operación

Para gate release E2E determinístico:

- `make test-e2e`

Para pruebas de resiliencia no bloqueantes:

- `make test-e2e-chaos`
