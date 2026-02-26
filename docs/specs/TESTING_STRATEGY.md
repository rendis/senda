# Testing Strategy — Senda P1

**Referencia:** TECH_SPEC v1.4 | TECH_STORIES.md

---

## 1. Filosofía

TDD (Test-Driven Development) como práctica obligatoria. Todo código de producción nace de un test que falla primero. La pirámide de tests se respeta estrictamente: muchos unit tests, menos integration, pocos E2E.

---

## 2. Pirámide de Tests

```
         ╱╲
        ╱ E2E ╲          ~5%   (flujos completos, browser o HTTP client)
       ╱────────╲
      ╱Integration╲      ~25%  (DB real, TestContainers)
     ╱──────────────╲
    ╱   Unit Tests    ╲   ~70%  (mocks, sin I/O, rápidos)
   ╱────────────────────╲
```

---

## 3. Tipos de Test

### 3.1. Unit Tests

**Scope:** Lógica de dominio, services, resolution engine, middleware, utilities.

**Principios:**
- Sin I/O (no DB, no HTTP, no filesystem)
- Ports mockeados con interfaces (Go interfaces naturales, sin frameworks de mock pesados)
- Ejecución < 1 segundo por archivo
- Naming: `TestNombreFuncion_Escenario_ResultadoEsperado`

**Qué se testea:**
- `internal/domain/` — Validaciones de entidades, parsing de addresses, slug validation
- `internal/service/` — Lógica de negocio con stores/ports mockeados
- `internal/resolution/` — ChainResolver, InjectorMerger, TemplateResolver, AdapterResolver, DomainResolver
- `internal/http/middleware/` — Auth, RBAC, scope extraction
- `pkg/` — slug validation, tracking ID generation, error types

**Patrón de mocks:**

```go
// Ejemplo: mock manual (preferido sobre frameworks)
type mockTenantStore struct {
    getByCodeFn func(ctx context.Context, code string) (*domain.Tenant, error)
}

func (m *mockTenantStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
    return m.getByCodeFn(ctx, code)
}
```

**Ejemplo de test TDD:**

```go
func TestSendService_Send_SuppressedRecipient_Returns422(t *testing.T) {
    // Arrange
    svc := NewSendService(
        &mockSuppressionStore{
            checkFn: func(ctx context.Context, email string) (bool, error) {
                return true, nil // suppressed
            },
        },
        // ... other mocks
    )

    // Act
    _, err := svc.Send(ctx, SendRequest{To: "blocked@example.com", ...})

    // Assert
    assert.ErrorIs(t, err, domain.ErrSuppressed)
}
```

### 3.2. Integration Tests

**Scope:** Store implementations, cache, rate limiter, migrations, queries SQL complejas.

**Herramienta:** TestContainers for Go — levanta PostgreSQL 16 + pg_cron en container efímero por suite.

**Principios:**
- Cada suite crea su schema completo (run migrations)
- Cada test usa transacción con rollback, o truncate tables entre tests
- Tests tagged con `//go:build integration`
- No se ejecutan en `make test` normal, solo con `make test-integration`

**Qué se testea:**
- `internal/adapter/postgres/` — Todos los repos (CRUD, pagination, resolución por chain)
- `internal/adapter/pgcache/` — Cache Get/Set/Delete/DeletePattern con TTL
- Rate limiter — `take_send_token()` PL/pgSQL function
- Migrations — up completo + down completo + idempotencia
- Constraints — CHECKs, UNIQUEs, EXCLUDE, FKs

**Setup compartido:**

```go
// testutil/pgcontainer.go
func SetupTestDB(t *testing.T) *pgxpool.Pool {
    ctx := context.Background()
    container, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("senda_test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready").WithStartupTimeout(30*time.Second),
        ),
    )
    require.NoError(t, err)
    t.Cleanup(func() { container.Terminate(ctx) })

    connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)

    // Run migrations
    runMigrations(t, connStr)

    return pool
}
```

**Qué se verifica:**
- Resolución jerárquica: crear datos en global, _system, workspace → resolver → verificar prioridad
- Soft delete: delete → verify not returned → verify still in DB
- Cursor pagination: insert 100 records → paginate 20 at a time → verify order + completeness
- Constraint violations: duplicate slug → expect error
- Token bucket: take N tokens → verify refill after time

### 3.3. E2E Tests

**Scope:** Flujos completos de usuario vía HTTP API.

**Herramienta:** TestContainers (PG) + HTTP test server (Echo test instance) + httptest.

**Principios:**
- Pocos tests, solo happy paths críticos y edge cases de alto impacto
- Cada test es un flujo completo: setup → action → verify side effects
- Tagged con `//go:build e2e`
- Se ejecutan con `make test-e2e`

**Flujos E2E mínimos:**

| # | Flujo | Verifica |
|---|-------|----------|
| 1 | Onboarding | Fresh DB → GET status (false) → POST setup → GET status (true) |
| 2 | Full CRUD cycle | Create tenant → workspace → template type → template → version → publish |
| 3 | Send email | Setup data → POST /send → verify email record + queued job |
| 4 | Provider event | POST SES webhook → verify email status updated + suppression created |
| 5 | Auth flows | API Key auth → success; revoked key → 401; OIDC → member lookup |
| 6 | Suppression | Add suppression → POST /send → verify 422 |
| 7 | Resolution chain | Global template + workspace override → send → verify workspace version used |

---

## 4. Test Data & Fixtures

### 4.1. Fixtures estáticos

```
testdata/
├── mjml/
│   ├── valid_basic.mjml
│   ├── valid_with_variables.mjml
│   ├── invalid_syntax.mjml
│   └── expected_basic.html
├── sns/
│   ├── bounce_notification.json
│   ├── complaint_notification.json
│   ├── delivery_notification.json
│   └── subscription_confirmation.json
├── dkim/
│   ├── test_private_key.pem
│   └── test_public_key.pem
└── config/
    └── test_config.yaml
```

### 4.2. Test factories

```go
// testutil/factory.go
func NewTenant(overrides ...func(*domain.Tenant)) *domain.Tenant {
    t := &domain.Tenant{
        ID:   uuid.New(),
        Code: "test-tenant",
        Name: "Test Tenant",
    }
    for _, o := range overrides {
        o(t)
    }
    return t
}

func NewWorkspace(tenantID uuid.UUID, overrides ...func(*domain.Workspace)) *domain.Workspace { ... }
func NewAdapter(overrides ...func(*domain.Adapter)) *domain.Adapter { ... }
// ... etc para cada entidad
```

### 4.3. Seeder para integration tests

```go
// testutil/seeder.go
// Crea un escenario completo: tenant + _system + workspace + injectors + adapter + template type + template + version published
func SeedFullScenario(t *testing.T, pool *pgxpool.Pool) *TestScenario {
    // Returns struct with all IDs for assertions
}
```

---

## 5. Cobertura

| Capa | Cobertura mínima | Notas |
|------|-------------------|-------|
| `domain/` | 90% | Lógica pura, fácil de testear |
| `service/` | 85% | Paths críticos + error handling |
| `resolution/` | 90% | Core del producto |
| `adapter/postgres/` | 80% | Integration tests cubren queries |
| `adapter/ses/` | 70% | Mock de AWS SDK |
| `http/middleware/` | 85% | Auth paths críticos |
| `http/handler/` | 75% | Validación + response format |
| `pkg/` | 95% | Utilities puras |

**Enforcement:** `go test -coverprofile=coverage.out ./...` en CI. Build falla si cobertura global < 80%.

---

## 6. Ejecución

### Comandos Make

```makefile
# Unit tests (rápido, sin dependencias externas)
test:
	go test -v -count=1 -race ./internal/... ./pkg/...

# Integration tests (requiere Docker para TestContainers)
test-integration:
	go test -v -count=1 -tags=integration ./internal/adapter/...

# E2E tests (requiere Docker)
test-e2e:
	go test -v -count=1 -tags=e2e ./test/e2e/...

# Coverage report
test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

# All tests
test-all: test test-integration test-e2e
```

### CI Pipeline (cuando se implemente)

```
push → lint (golangci-lint) → unit tests → integration tests → build → (optional: E2E)
```

- Unit tests: run on every push
- Integration tests: run on every push (TestContainers son rápidos)
- E2E tests: run on PR merge to main
- Coverage report: uploaded como artefacto

---

## 7. Principios TDD Aplicados al Proyecto

1. **Red → Green → Refactor** en cada funcionalidad
2. **Un test, un comportamiento.** No testear implementación interna, testear comportamiento observable.
3. **Tests son documentación.** Los nombres de test describen el comportamiento del sistema.
4. **No mockear lo que no te pertenece.** Mockear ports (interfaces propias), no AWS SDK directamente. El SES adapter se testea con integration test o mock HTTP server.
5. **Integration tests > mocks para queries SQL.** Las queries complejas (resolución por chain, pagination) se testean contra PG real, no contra mocks.
6. **Cada HT produce sus tests.** No hay fase separada de "escribir tests". Los tests son parte del entregable de cada HT.
