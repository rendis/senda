# Matriz de Cumplimiento P0 Backend (R-01..R-23)

**Fecha de corte:** 2026-02-25  
**Scope:** Backend P0 + QA gate técnico (unit + integration + e2e deterministic)

| Requisito | Endpoint/Servicio principal | Evidencia automática | Estado |
|---|---|---|---|
| R-01 | `resolution.ChainResolver`, `TemplateResolver`, `InjectorMerger` | `go test ./internal/resolution` | Cumple |
| R-02 | CRUD `tenants/workspaces` | `go test ./internal/http/handler -run TenantHandler\|WorkspaceHandler` | Cumple |
| R-03 | `InjectorHandler`, `InjectorStore` | `go test ./internal/http/handler -run Injector` + integración postgres | Cumple |
| R-04 | `POST /api/v1/send`, `SendService` | `TestSendService_*`, `TestSendHandler_*` (scope, límites, validaciones) | Cumple |
| R-05 | Resolución template + versión + locale | `TemplateResolver` + `TemplateHandler` tests | Cumple |
| R-06 | `TemplateTypeService` + schema en API | `template_type` handler/service tests | Cumple |
| R-07 | Versionado draft/published/archived | `TemplateService` + `TemplateRepo` tests de publish/list | Cumple |
| R-08 | Unicidad por scope | `TemplateRepo_CreateTemplate_Conflict` + `TemplateType` conflicts | Cumple |
| R-09 | Kill switch template (`disable/enable`) | Nuevos tests: `DisableTemplate*`, `SetDisabled*`, `ErrTemplateDisabled->409` | Cumple |
| R-10 | Adapters SES/Gmail + puertos | `adapter` handler tests + `adapter/ses` y `adapter/gmail` tests | Cumple |
| R-11 | Lifecycle + eventos | `EmailRepo` + `EmailHandler` detail/events tests | Cumple |
| R-12 | Trazabilidad `external_id` | `EmailRepo.QueryByWorkspace` filtros + `data-plane /emails` | Cumple |
| R-13 | Auth email provider-managed | ADR-0001 + `DefaultIdentityProviderFactory` tests (sin DKIM in-app) | Cumple |
| R-14 | Sync identities con herencia | `IdentityService.SyncIdentities` + handlers `identities/sync` | Cumple |
| R-15 | Suppression global/workspace | `SuppressionHandler` + `SendService_SuppressedRecipient` | Cumple |
| R-16 | Dashboard métricas | `DashboardHandler` + `DashboardRepo` tests | Cumple |
| R-17 | OIDC + membresía | `middleware/auth` tests (OIDC path), handlers protegidos | Cumple |
| R-18 | RBAC roles/scopes | `middleware/rbac` tests + `member` handler tests | Cumple |
| R-19 | Onboarding inicial | `OnboardingService` + `OnboardingHandler` tests | Cumple |
| R-20 | API Keys data plane | `APIKeyService/Handler` + `/send` scope enforcement (`403`) | Cumple |
| R-21 | API de consulta/búsqueda | Nuevos endpoints `/api/v1/emails*` por API key + tests | Cumple |
| R-22 | Soft delete + dependencias | repos/handlers de tenant/workspace/adapters/templates (soft delete paths) | Cumple |
| R-23 | Audit logging | `AuditHandler` + `AuditRepo` tests | Cumple |

## Gates ejecutados en este cierre

- `make test`
- `make test-integration`
- `make lint`
- `go vet ./...`
- `make test-e2e`
- `make test-e2e-chaos` (observacional, no bloqueante)
