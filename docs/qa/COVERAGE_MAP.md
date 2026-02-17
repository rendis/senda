# Endpoint-to-Test Coverage Map — HT-37

**Date:** 2026-02-17

## Legend

- **F** = Happy Path | **E** = Error Flow | **S** = Security | **C** = Chaos

## Coverage Matrix

### Health & Onboarding

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `/health` | GET | — | — | — | C01, C02 |
| `/api/v1/onboarding/status` | GET | F01 | — | — | — |
| `/api/v1/onboarding/setup` | POST | F01 | E10 | S06 | — |

### Tenant Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `/api/v1/manage/tenants/:code` | GET | — | E06 | S01, S03 | — |

### Workspace Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `/api/v1/manage/tenants/:t/workspaces` | POST | F02 | E10 | S03, S12 | — |
| `/api/v1/manage/tenants/:t/workspaces/:w` | GET | — | — | S03 | — |

### Adapter Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../adapters` | POST | F02 | — | S07 | — |
| `.../adapters` | GET | — | — | — | — |
| `.../adapters/:id` | GET | — | — | S07 | — |
| `.../adapters/:id` | DELETE | — | — | S03 | — |

### Domain Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../domains` | POST | F02 | E03 | S07 | — |
| `.../domains/:id` | GET | — | — | S07 | — |

### Template Type Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../template-types` | POST | F03 | E02, E10 | S01, S09 | C05 |
| `.../template-types` | GET | F07, F10 | — | S03 | — |
| `.../template-types/:slug` | GET | — | — | S12 | — |

### Template Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../templates` | POST | F03 | E10 | S04, S07 | C05 |
| `.../templates/:id/versions` | POST | F03 | E03, E05 | S04, S11 | C05 |
| `.../templates/:id/versions/:vid/locales/:l` | PUT | F03 | — | — | — |
| `.../templates/:id/versions/:vid/publish` | POST | F03 | — | — | C05 |

### Injector Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../injectors` | POST | F02, F08 | — | S01, S04, S09 | — |
| `.../injectors/:id/values` | PUT | F08 | — | S04 | — |
| `.../injectors/:id/values` | GET | F08 | — | — | — |

### API Key Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../api-keys` | POST | F09 | — | S06, S07, S10 | — |
| `.../api-keys/:id` | DELETE | F09 | — | S07 | — |

### Send API

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `/api/v1/send` | POST | F04, F05, F06, F09 | E02, E03, E04, E05, E11 | S01, S02, S03, S06, S08 | C04, C06, C08 |

### Email Query

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../emails` | GET | F06 | — | S01 | — |

### Webhook Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../webhooks` | POST | — | — | S05, S07 | C07 |

### Member Management

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../members/invite` | POST | — | — | S06 | — |
| member roles | — | F10 | E08, E09 | S03 | — |

### Suppression

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| `.../suppressions` | POST | — | E11 | — | — |

### Soft Delete

| Endpoint | Method | F | E | S | C |
|----------|--------|---|---|---|---|
| (template soft delete) | DELETE | — | E12 | — | — |

## Uncovered Endpoints

| Endpoint | Method | Reason |
|----------|--------|--------|
| `/api/v1/manage/.../templates/:id` | PATCH | Template update not tested |
| `/api/v1/manage/.../audit-log` | GET | Audit log query not explicitly tested |
| `/api/v1/manage/.../config` | GET/PUT | Workspace config not explicitly tested |
| `/api/v1/manage/.../templates/:id` | DELETE | Disabled endpoint (E01 skipped) |

## Cross-Cutting Concerns

| Concern | Tests |
|---------|-------|
| JWT Authentication | S02 (10 subtests) |
| API Key Authentication | F09, S02, S08, S09 |
| RBAC (viewer/editor/admin) | F10, E08, E09, S03 |
| Tenant isolation | E06, S03, S07 |
| Workspace isolation | E07, S03, S07 |
| Input validation | E05, E10, S01, S04, S06, S11, S12 |
| Error handling | E02-E12 |
| Async processing (River) | F04, F05, C01, C03, C08 |
| Rate limiting | E04, C06, S08 |
| Encryption at rest | S10 |
| Concurrency safety | C04, C05, C09 |
