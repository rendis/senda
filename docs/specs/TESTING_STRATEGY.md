# Testing Strategy — Senda P1

**Reference:** TECH_SPEC v1.4 | TECH_STORIES.md

---

## 1. Philosophy

Test-Driven Development (TDD) is mandatory. Every piece of production code must begin with a failing test. The test pyramid is enforced strictly: many unit tests, fewer integration tests, and a small number of E2E tests.

---

## 2. Test Pyramid

```
         ╱╲
        ╱ E2E ╲          ~5%   (full flows, browser or HTTP client)
       ╱────────╲
      ╱Integration╲      ~25%  (real DB, TestContainers)
     ╱──────────────╲
    ╱   Unit Tests    ╲   ~70%  (mocks, no I/O, fast)
   ╱────────────────────╲
```

---

## 3. Test Types

### 3.1. Unit Tests

**Scope:** Domain logic, services, resolution engine, middleware, utilities.

**Principles:**
- No I/O (no DB, no HTTP, no filesystem)
- Ports mocked with interfaces (plain Go interfaces, no heavy mock frameworks)
- Execution time under 1 second per file
- Naming: `TestFunctionName_Scenario_ExpectedResult`

**What is tested:**
- `internal/domain/` — Entity validations, address parsing, slug validation
- `internal/service/` — Business logic with mocked stores/ports
- `internal/resolution/` — ChainResolver, InjectorMerger, TemplateResolver, AdapterResolver, DomainResolver
- `internal/http/middleware/` — Auth, RBAC, scope extraction
- `pkg/` — slug validation, tracking ID generation, error types

**Mock pattern:**

```go
// Example: manual mock (preferred over frameworks)
type mockTenantStore struct {
    getByCodeFn func(ctx context.Context, code string) (*domain.Tenant, error)
}

func (m *mockTenantStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
    return m.getByCodeFn(ctx, code)
}
```

**TDD test example:**

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

**Scope:** Store implementations, cache, rate limiter, migrations, complex SQL queries.

**Tool:** TestContainers for Go — spins up PostgreSQL 16 + pg_cron in an ephemeral container per suite.

**Principles:**
- Each suite creates its full schema (runs migrations)
- Each test uses a transaction rollback, or truncates tables between tests
- Tests are tagged with `//go:build integration`
- They do not run under the normal `make test`, only with `make test-integration`

**What is tested:**
- `internal/adapter/postgres/` — All repos (CRUD, pagination, chain resolution)
- `internal/adapter/pgcache/` — Cache Get/Set/Delete/DeletePattern with TTL
- Rate limiter — `take_send_token()` PL/pgSQL function
- Migrations — full up + full down + idempotency
- Constraints — CHECKs, UNIQUEs, EXCLUDEs, FKs

**Shared setup:**

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

**What is verified:**
- Hierarchical resolution: create data in global, _system, workspace → resolve → verify priority
- Soft delete: delete → verify not returned → verify still in DB
- Cursor pagination: insert 100 records → paginate 20 at a time → verify order + completeness
- Constraint violations: duplicate slug → expect an error
- Token bucket: take N tokens → verify refill after time

### 3.3. E2E Tests

**Scope:** Complete user flows through the HTTP API.

**Tool:** TestContainers (PG) + HTTP test server (Echo test instance) + `httptest`.

**Principles:**
- Few tests, only critical happy paths and high-impact edge cases
- Each test is a full flow: setup → action → verify side effects
- Tagged with `//go:build e2e`
- Run with `make test-e2e`

**Minimum E2E flows:**

| # | Flow | Verifies |
|---|-------|----------|
| 1 | Onboarding | Fresh DB → GET status (false) → POST setup → GET status (true) |
| 2 | Full CRUD cycle | Create tenant → workspace → template type → template → version → publish |
| 3 | Send email | Set up data → POST /send → verify email record + queued job |
| 4 | Provider event | POST SES webhook → verify updated email status + created suppression |
| 5 | Auth flows | API Key auth → success; revoked key → 401; OIDC → member lookup |
| 6 | Suppression | Add suppression → POST /send → verify 422 |
| 7 | Resolution chain | Global template + workspace override → send → verify workspace version used |

### 3.4. System UI DoD for new management features

New management-plane functionality is not done with API coverage alone.
Every new UI feature must include a browser-based system flow that:

- boots the required environment with Docker/Testcontainers
- exercises the real UI with `agent-browser`
- captures screenshot evidence for the key happy-path and protected/error states
- writes a report under `artifacts/system/<timestamp>/`

At minimum, the system UI flow must prove:

- the feature is discoverable from the intended navigation entrypoint
- the main CRUD or state-transition path works end-to-end
- protected or destructive cases show the expected UI/behavior
- the resulting scope/route navigation matches the product flow

---

## 4. Test Data & Fixtures

### 4.1. Static fixtures

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
// ... etc. for every entity
```

### 4.3. Seeder for integration tests

```go
// testutil/seeder.go
// Creates a full scenario: tenant + _system + workspace + injectors + adapter + template type + template + published version
func SeedFullScenario(t *testing.T, pool *pgxpool.Pool) *TestScenario {
    // Returns a struct with all IDs for assertions
}
```

---

## 5. Coverage

| Layer | Minimum coverage | Notes |
|------|-------------------|-------|
| `domain/` | 90% | Pure logic, easy to test |
| `service/` | 85% | Critical paths + error handling |
| `resolution/` | 90% | Core product logic |
| `adapter/postgres/` | 80% | Integration tests cover queries |
| `adapter/ses/` | 70% | Mock AWS SDK |
| `http/middleware/` | 85% | Critical auth paths |
| `http/handler/` | 75% | Validation + response format |
| `pkg/` | 95% | Pure utilities |

**Enforcement:** `go test -coverprofile=coverage.out ./...` in CI. Build fails if global coverage is below 80%.

---

## 6. Execution

### Make commands

```makefile
# Unit tests (fast, no external dependencies)
test:
	go test -v -count=1 -race ./internal/... ./pkg/...

# Integration tests (requires Docker for TestContainers)
test-integration:
	go test -v -count=1 -tags=integration ./internal/adapter/...

# E2E tests (requires Docker)
test-e2e:
	go test -v -count=1 -tags=e2e ./test/e2e/...

# Coverage report
test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

# All tests
test-all: test test-integration test-e2e
```

### CI Pipeline (when implemented)

```
pull_request → backend gate + frontend gate + taxonomy check
workflow_dispatch → manual system gate(s) and observational checks
```

- Unit tests: run inside the automatic backend PR gate
- Frontend tests: run inside the automatic frontend PR gate via `pnpm --dir web test`
- Taxonomy drift checks: run as part of PR validation so docs and workflows stay honest
- Manual / observational system gates: run through `workflow_dispatch`, not as automatic PR blockers
- Coverage report: uploaded as an artifact when the relevant CI path includes it

---

## 7. TDD Principles Applied to the Project

1. **Red → Green → Refactor** for every feature
2. **One test, one behavior.** Do not test internal implementation, test observable behavior.
3. **Tests are documentation.** Test names describe system behavior.
4. **Do not mock what you do not own.** Mock ports (your own interfaces), not the AWS SDK directly. The SES adapter is tested with an integration test or a mock HTTP server.
5. **Integration tests > mocks for SQL queries.** Complex queries (chain resolution, pagination) are tested against a real PostgreSQL instance, not mocks.
6. **Each HT produces its own tests.** There is no separate "write tests" phase. Tests are part of the deliverable for each HT.
