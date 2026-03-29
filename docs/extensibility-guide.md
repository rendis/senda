# Extensibility Guide

Senda can be used as a Go library. Import `github.com/senda-app/senda/sdk`, create an Engine, register your extensions, and call `Run()`. Update Senda independently — your extensions keep working.

```bash
go get github.com/senda-app/senda
```

---

## Table of Contents

1. [Engine](#1-engine)
2. [Injectors](#2-injectors)
3. [Init Function](#3-init-function)
4. [Lifecycle Hooks](#4-lifecycle-hooks)
5. [InjectorContext Reference](#5-injectorcontext-reference)
6. [How Code Injectors Merge with DB Injectors](#6-how-code-injectors-merge-with-db-injectors)
7. [Consumer Project Structure](#7-consumer-project-structure)
8. [Complete Example](#8-complete-example)
9. [Error Handling](#9-error-handling)
10. [Troubleshooting](#10-troubleshooting)

---

## 1. Engine

The `sdk.Engine` is the entry point. It wraps Senda's internal bootstrap and lets you register extensions before starting.

### Construction

```go
// Default config path ("config.yaml"):
engine := sdk.New()

// Custom config path:
engine := sdk.NewWithConfig("settings/app.yaml")
```

The config path can be overridden at runtime with the `SENDA_CONFIG` environment variable.

### Fluent API

All registration methods return `*Engine` for chaining:

```go
engine := sdk.NewWithConfig("config.yaml").
    RegisterInjector(&StudentInjector{}).
    RegisterInjector(&InstitutionInjector{}).
    SetInitFunc(myInit()).
    OnStart(connectMongo).
    OnShutdown(closeMongo)
```

### Run

`engine.Run()` performs the following sequence:

1. Load configuration (YAML + `SENDA_*` env vars)
2. Setup structured logging
3. Bootstrap all internal components (DB, repositories, resolvers, HTTP server, River workers)
4. Execute `OnStart` hooks (synchronously, in registration order)
5. Start River workers (background goroutine)
6. Start HTTP server (blocks until signal)
7. On SIGINT/SIGTERM: stop server, execute `OnShutdown` hooks (reverse order), close resources

### Extension Points Summary

| Method | Purpose | Multiplicity |
|--------|---------|-------------|
| `RegisterInjector(inj)` | Add a code injector | Multiple allowed |
| `SetInitFunc(fn)` | Per-request initialization | One (last wins) |
| `OnStart(fn)` | Post-bootstrap hook | Multiple (FIFO) |
| `OnShutdown(fn)` | Pre-exit hook | Multiple (LIFO) |

### What the Engine Does NOT Expose

Built-in components are managed by YAML configuration, not the SDK:

- Email senders (SES, Gmail, SMTP)
- PostgreSQL stores and connection pool
- River job queue and workers
- PG UNLOGGED cache
- AES-256-GCM encryption
- Rate limiter (PL/pgSQL token bucket)
- OIDC / API Key authentication
- HTTP middleware chain
- Resolution engine (chain, template, adapter resolvers)

---

## 2. Injectors

Code injectors are the primary extension point. They resolve dynamic values at send time and merge with DB injectors into templates.

### Interface

```go
type Injector interface {
    // Code returns the unique injector name.
    // Maps to the template namespace: {{ injector.<Code()>.<field> }}
    Code() string

    // Resolve returns the resolution function and optional dependency codes.
    // Dependencies are names of other injectors (code or DB) that must resolve first.
    Resolve() (ResolveFunc, []string)

    // IsCritical returns true if a failure should abort the entire send.
    // When false, the injector is silently skipped on error.
    IsCritical() bool

    // Timeout returns the max duration for resolution.
    // Zero means use the default (30 seconds).
    Timeout() time.Duration
}

// ResolveFunc executes the injector logic.
// Returns field_name -> value pairs.
type ResolveFunc func(ctx context.Context, injCtx *InjectorContext) (map[string]any, error)
```

### Minimal Example

```go
type GreetingInjector struct{}

func (i *GreetingInjector) Code() string { return "greeting" }

func (i *GreetingInjector) Resolve() (sdk.ResolveFunc, []string) {
    return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
        name := injCtx.Variables()["name"]
        return map[string]any{
            "message": fmt.Sprintf("Hello, %v!", name),
        }, nil
    }, nil
}

func (i *GreetingInjector) IsCritical() bool      { return false }
func (i *GreetingInjector) Timeout() time.Duration { return 0 }
```

In the MJML template:
```
<mj-text>{{ injector.greeting.message }}</mj-text>
```

### Reading Headers

Injectors can read HTTP headers from the send request:

```go
func (i *CaseInjector) Resolve() (sdk.ResolveFunc, []string) {
    return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
        caseID := injCtx.Header("X-Case-Id")
        caseData, err := fetchCase(ctx, caseID)
        if err != nil {
            return nil, err
        }
        return map[string]any{
            "title":  caseData.Title,
            "status": caseData.Status,
        }, nil
    }, nil
}
```

### Using Init Data

When an `InitFunc` is registered, its return value is available to all injectors:

```go
func (i *StudentInjector) Resolve() (sdk.ResolveFunc, []string) {
    return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
        // Type-assert the init data to your concrete type.
        data := injCtx.InitData().(*MyInitData)
        return map[string]any{
            "full_name": data.Student.FullName,
            "email":     data.Student.Email,
        }, nil
    }, nil
}
```

### Dependencies Between Injectors

Injectors can declare dependencies on other injectors (code or DB). The merger resolves them in topological order.

```go
type SummaryInjector struct{}

func (i *SummaryInjector) Code() string { return "summary" }

func (i *SummaryInjector) Resolve() (sdk.ResolveFunc, []string) {
    return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
        // Read a DB injector's value.
        company, _ := injCtx.GetResolved("company")
        companyName := ""
        if company != nil {
            companyName, _ = company["name"].(string)
        }

        // Read another code injector's value.
        student, _ := injCtx.GetResolved("student")
        studentName := ""
        if student != nil {
            studentName, _ = student["full_name"].(string)
        }

        return map[string]any{
            "text": fmt.Sprintf("%s enrolled at %s", studentName, companyName),
        }, nil
    }, []string{"company", "student"} // <-- declare dependencies
}
```

**Rules:**
- Dependencies are resolved before the dependent injector runs
- If a dependency is a DB injector, it's already resolved (seeded into context before code injectors run)
- If a dependency code doesn't exist in either DB or code injectors, it's silently skipped
- Circular dependencies are not detected — avoid them

### Critical vs Non-Critical

| `IsCritical()` | On Error | Use When |
|----------------|----------|----------|
| `true` | Send is aborted, error returned to caller | Data is essential (e.g., student name in a certificate) |
| `false` | Injector is skipped, warning logged, send continues | Data is supplementary (e.g., a logo URL) |

### Timeout

Each injector can set a per-resolution timeout:

```go
func (i *SlowInjector) Timeout() time.Duration {
    return 10 * time.Second // max 10s for this injector
}
```

Zero (default) uses the engine default of **30 seconds**.

### Name Collisions

If a code injector has the same `Code()` as a DB injector, the **code injector wins** and a warning is logged:

```
WARN code injector overrides DB injector code=brand
```

If two code injectors have the same `Code()`, the **last registered wins** (map overwrite).

### Registration

```go
engine.RegisterInjector(&StudentInjector{})
engine.RegisterInjector(&InstitutionInjector{})
engine.RegisterInjector(&SummaryInjector{})
```

---

## 3. Init Function

The init function runs **once per send request**, before any code injectors execute. Use it to load shared data that multiple injectors need.

### Signature

```go
type InitFunc func(ctx context.Context, injCtx *InjectorContext) (any, error)
```

### Example

```go
type InitData struct {
    Admission *Admission
    Student   *Student
}

func MyInit() sdk.InitFunc {
    return func(ctx context.Context, injCtx *sdk.InjectorContext) (any, error) {
        caseID := injCtx.Header("X-Case-Id")
        if caseID == "" {
            return nil, nil // no case context, injectors handle gracefully
        }

        admission, err := admissionRepo.FindByID(ctx, caseID)
        if err != nil {
            return nil, fmt.Errorf("load admission: %w", err)
        }

        student, err := studentRepo.FindByID(ctx, admission.StudentID)
        if err != nil {
            return nil, fmt.Errorf("load student: %w", err)
        }

        return &InitData{
            Admission: admission,
            Student:   student,
        }, nil
    }
}
```

### Behavior

- Runs after DB injectors are resolved (so `injCtx.GetResolved("db_injector")` works)
- If it returns an error, the **entire send is aborted**
- The returned value is stored in `injCtx.InitData()` and accessible to all code injectors
- Only one InitFunc is allowed; calling `SetInitFunc` again replaces the previous one

### Registration

```go
engine.SetInitFunc(MyInit())
```

---

## 4. Lifecycle Hooks

### OnStart

Runs after Senda is fully bootstrapped (DB connected, migrations applied, River ready, HTTP server configured) but **before** the server accepts requests.

```go
engine.OnStart(func(ctx context.Context) error {
    mongoClient, err = mongo.Connect(ctx, mongoURI)
    if err != nil {
        return fmt.Errorf("connect mongo: %w", err)
    }
    initRepositories(mongoClient)
    return nil
})
```

- Multiple hooks allowed, executed in **registration order** (FIFO)
- If any hook returns an error, startup is aborted
- Use for: connecting external databases, initializing HTTP clients, loading secrets

### OnShutdown

Runs after the HTTP server stops and before the process exits.

```go
engine.OnShutdown(func(ctx context.Context) error {
    if mongoClient != nil {
        return mongoClient.Disconnect(ctx)
    }
    return nil
})
```

- Multiple hooks allowed, executed in **reverse order** (LIFO)
- Errors are logged but don't prevent shutdown of subsequent hooks
- Shutdown context has a 30-second timeout
- Use for: closing database connections, flushing buffers, stopping background processes

---

## 5. InjectorContext Reference

The `InjectorContext` is passed to both the init function and all code injectors. It's **thread-safe** (uses `sync.RWMutex` internally).

### Available Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Header(key)` | `string` | HTTP request header value. Empty if not found or headers are nil |
| `Headers()` | `map[string]string` | Copy of all HTTP headers. Safe to modify |
| `Ref()` | `string` | Send request addressing ref (e.g., `"tenant:workspace:templateType"`) |
| `Variables()` | `map[string]any` | Caller-provided event variables from the send request |
| `InitData()` | `any` | Value returned by `InitFunc`. Nil if no InitFunc set. Type-assert to your concrete type |
| `TenantID()` | `uuid.UUID` | Resolved tenant UUID |
| `WorkspaceID()` | `uuid.UUID` | Resolved workspace UUID |
| `TemplateType()` | `string` | Resolved template type slug |
| `GetResolved(code)` | `(map[string]any, bool)` | Get resolved fields for a named injector (DB or code). Returns false if not yet resolved |

### Data Flow

```
HTTP Request arrives (POST /api/v1/send)
    │
    ├── Headers extracted → injCtx.Header("X-Case-Id")
    ├── Body parsed → injCtx.Ref(), injCtx.Variables()
    ├── Tenant/Workspace resolved → injCtx.TenantID(), injCtx.WorkspaceID()
    │
    ▼
DB Injectors resolved → seeded into injCtx.GetResolved("db_name")
    │
    ▼
InitFunc runs → result stored in injCtx.InitData()
    │
    ▼
Code Injectors resolve (in dependency order)
    ├── Each reads injCtx as needed
    ├── Each result stored via injCtx.SetResolved(code, fields)
    │
    ▼
All values merged → passed to VariableRenderer → MJML template compiled
```

---

## 6. How Code Injectors Merge with DB Injectors

Senda has two types of injectors that coexist:

| Type | Defined In | Managed By | Resolved At |
|------|-----------|-----------|-------------|
| **DB Injectors** | PostgreSQL tables (`injector_definitions`, `injector_fields`, `injector_values`) | Dashboard / Management API | Per-workspace, scope hierarchy (workspace > \_system > global) |
| **Code Injectors** | Go code (`sdk.Injector` interface) | SDK registration (`engine.RegisterInjector`) | Per-request, by the resolution engine |

### Merge Process

1. **DB injectors resolve first** — the `InjectorMerger` resolves all DB injectors via the scope hierarchy chain (workspace > \_system > global), producing `map[string]map[string]any`
2. **DB values seeded into context** — all DB injector values are available to code injectors via `injCtx.GetResolved("db_injector_name")`
3. **InitFunc runs** — loads shared per-request data
4. **Code injectors resolve** — in dependency order (topological sort). Each injector's result is immediately available to subsequent injectors
5. **Merge** — code injector values are merged into the same map. On name collision, code injector wins

### Template Access

Both types use the same template syntax:

```
{{ injector.company.name }}       ← could be DB or code injector
{{ injector.student.full_name }}  ← could be DB or code injector
{{ event.name }}                  ← caller-provided variable (not an injector)
```

The template doesn't know or care whether a value came from DB or code.

---

## 7. Consumer Project Structure

Following the pattern from pdf-forge and doc-assembly SDKs:

```
my-senda-app/
├── main.go                       Entry point
├── extensions/
│   ├── register.go               Single registration function
│   ├── init.go                   InitFunc implementation
│   └── injectors/
│       ├── student.go            Code injector: student data
│       ├── institution.go        Code injector: institution data
│       └── summary.go            Code injector: derived summary
├── internal/
│   ├── datasource/
│   │   ├── mongodb/              MongoDB repositories
│   │   ├── api/                  External API clients
│   │   └── factory/              Dev/prod factory pattern
│   └── shared/                   JWT utilities, helpers
├── settings/
│   └── app.yaml                  Senda YAML configuration
├── go.mod                        requires github.com/senda-app/senda
└── go.sum
```

### `main.go`

```go
package main

import (
    "log/slog"
    "os"

    "github.com/senda-app/senda/sdk"
    "my-senda-app/extensions"
)

func main() {
    engine := sdk.NewWithConfig("settings/app.yaml")
    extensions.Register(engine)

    if err := engine.Run(); err != nil {
        slog.Error("senda failed", "error", err)
        os.Exit(1)
    }
}
```

### `extensions/register.go`

```go
package extensions

import (
    "context"
    "my-senda-app/extensions/injectors"
    "github.com/senda-app/senda/sdk"
)

var mongoClient *mongo.Client

func Register(engine *sdk.Engine) {
    // Init function: load shared data per request.
    engine.SetInitFunc(TetherInit())

    // Code injectors.
    engine.RegisterInjector(&injectors.StudentInjector{})
    engine.RegisterInjector(&injectors.InstitutionInjector{})
    engine.RegisterInjector(&injectors.SummaryInjector{})

    // Lifecycle: connect/disconnect MongoDB.
    engine.OnStart(func(ctx context.Context) error {
        client, err := connectMongo(ctx)
        if err != nil {
            return err
        }
        mongoClient = client
        injectors.SetMongoClient(client)
        return nil
    })

    engine.OnShutdown(func(ctx context.Context) error {
        if mongoClient != nil {
            return mongoClient.Disconnect(ctx)
        }
        return nil
    })
}
```

---

## 8. Complete Example

A full working injector that loads admission data from MongoDB:

```go
package injectors

import (
    "context"
    "fmt"
    "time"

    "github.com/senda-app/senda/sdk"
)

// AdmissionInjector provides admission-related fields to email templates.
// Templates use: {{ injector.admission.student_name }}, {{ injector.admission.status }}, etc.
type AdmissionInjector struct{}

func (i *AdmissionInjector) Code() string { return "admission" }

func (i *AdmissionInjector) Resolve() (sdk.ResolveFunc, []string) {
    return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
        // Read from init data (loaded once, shared across injectors).
        initData := injCtx.InitData()
        if initData == nil {
            return map[string]any{}, nil // graceful: no admission context
        }

        data, ok := initData.(*InitData)
        if !ok {
            return nil, fmt.Errorf("unexpected init data type: %T", initData)
        }

        adm := data.Admission
        return map[string]any{
            "student_name": adm.Student.FullName,
            "status":       string(adm.Status),
            "created_at":   adm.CreatedAt.Format("02/01/2006"),
            "campus_name":  adm.Campus.Name,
            "program":      adm.Program.Name,
        }, nil
    }, nil // no dependencies
}

func (i *AdmissionInjector) IsCritical() bool      { return true }  // essential for the email
func (i *AdmissionInjector) Timeout() time.Duration { return 0 }    // use default (30s)
```

---

## 9. Error Handling

### InitFunc Errors

If the init function returns an error, the entire send is aborted and the error propagates to the API caller as a 500 Internal Server Error.

### Critical Injector Errors

If a critical injector (`IsCritical() == true`) fails, the send is aborted:

```
critical code injector "admission" failed: db connection timeout
```

### Non-Critical Injector Errors

If a non-critical injector fails, it's skipped and a warning is logged:

```
WARN non-critical code injector failed code=logo error="image service unavailable"
```

The email is still sent — the template fields for that injector resolve to empty values.

### Timeout Errors

If an injector exceeds its timeout, the context is cancelled and the injector fails with a context deadline exceeded error. Whether this aborts the send depends on `IsCritical()`.

### OnStart Hook Errors

If any OnStart hook returns an error, `engine.Run()` returns that error immediately without starting the server.

### OnShutdown Hook Errors

Errors are logged but don't prevent other shutdown hooks from executing.

---

## 10. Troubleshooting

### "code injector overrides DB injector"

A code injector and a DB injector have the same name. The code injector wins. If this is unintentional, rename your code injector's `Code()` to something unique.

### Injector values are empty in the template

1. Verify the injector is registered: `engine.RegisterInjector(&MyInjector{})`
2. Verify the `Code()` matches the template: `{{ injector.<Code()>.<field> }}`
3. Check if `Resolve()` returns the correct field names
4. If using `InitData()`, verify the init function is registered and returns non-nil
5. If depending on other injectors, verify dependencies are spelled correctly

### InitFunc panics

Ensure you handle the case where `InitData()` returns nil (no init function set) or an unexpected type. Always use type assertions with the comma-ok pattern:

```go
data, ok := injCtx.InitData().(*MyType)
if !ok {
    return nil, fmt.Errorf("unexpected init data type")
}
```

### Dependency order seems wrong

Code injectors are resolved in dependency order (topological sort based on the `[]string` returned by `Resolve()`). If the order seems wrong:

1. Check that dependency codes exactly match other injectors' `Code()` values
2. DB injector dependencies are always available (resolved before code injectors)
3. Unknown dependency codes are silently skipped — check for typos

### Headers are empty in InjectorContext

Headers are extracted from the HTTP request by the send handler. If you're testing directly via `SendService.Send()`, ensure you set `req.Headers`:

```go
req := &service.SendRequest{
    Ref:     "tenant:ws:type",
    To:      []string{"user@test.com"},
    Headers: map[string]string{"X-Case-Id": "123"},
}
```
