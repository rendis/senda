# E2E Test Suite Implementation Summary

## Overview

Complete implementation of the happy path E2E test suite for Senda (HT-37), with all 10 test flows written as real, executable Go code using `testing` + `testify/require` + `net/http`.

**Status:** Ready for execution against running Senda backend + Mailpit

**Location:** `/sessions/friendly-kind-galileo/mnt/senda/test/e2e/`

---

## Files Created

### 1. helpers.go (381 lines)
**Purpose:** Shared test utilities for HTTP communication, authentication, and Mailpit integration

**Key Components:**

#### TestClient
- HTTP wrapper with auth header management
- Methods: `Get()`, `Post()`, `Put()`, `Delete()`
- Auth: `SetBearerToken()` for OIDC, `SetAPIKey()` for data plane
- Polling: `WaitForEmailStatus(trackingID, status, timeout)` for async email processing

#### MailpitClient
- REST client for Mailpit (fake SMTP server)
- Methods: `GetMessages()`, `GetMessage(id)`, `SearchMessages(query)`, `ClearMessages()`
- Polling: `WaitForMessages(count, timeout)` to wait for emails to arrive
- Assertions: `AssertMessageExists(recipient)`, `AssertMessageHasSubject(subject)`

#### Assertion Helpers
- `RequireStatus(t, resp, expectedStatus)` — Assert HTTP status code
- `ReadResponseBody(t, resp)` — Extract and log response body
- `ParseJSON[T](t, resp)` — Generic JSON unmarshaling
- `ParseJSONResponse(t, resp, v)` — Parse into provided struct
- `ParseError(t, resp)` — Extract error details
- `AssertError(t, resp, expectedCode)` — Assert error code matches

#### Message Types
- `Message` — Full email message with headers, HTML, text
- `MessageSummary` — Lightweight message reference (ID, From, To, Subject)
- `MessagesResponse` — Paginated Mailpit response
- `ErrorResponse` — Standard Senda error contract

---

### 2. seed.go (205 lines)
**Purpose:** Test data constants and request/response DTOs

**Key Constants:**

#### Tenant/Workspace
```
TenantCode       = "test-corp"
TenantName       = "Test Corporation"
WorkspaceCode    = "main"
WorkspaceName    = "Main Workspace"
SystemWorkspaceCode = "_system"
```

#### Users
```
SuperadminEmail        = "superadmin@test.example.com"
TenantAdminEmail       = "tenant-admin@test.example.com"
WorkspaceAdminEmail    = "ws-admin@test.example.com"
WorkspaceEditorEmail   = "ws-editor@test.example.com"
WorkspaceViewerEmail   = "ws-viewer@test.example.com"
```

#### Template Resources
```
TemplateTypeSlug  = "welcome-email"
TemplateTypeDesc  = "Welcome email for new users"
TemplateSlug      = "welcome-v1"
TemplateTypeName  = "Welcome Email"
```

#### Infrastructure
```
TestDomain           = "mail.test.example.com"
TestFromEmail        = "noreply@mail.test.example.com"
MailpitSMTPHost      = "mailpit"
MailpitSMTPPort      = 1025
DKIMSelector         = "default"
```

#### Request DTOs
- `OnboardingRequest` — Tenant + workspace + admin setup
- `TemplateTypeRequest` — Template type definition
- `CreateTemplateRequest` — Template creation
- `CreateVersionRequest` — Template version with MJML body
- `CreateLocaleRequest` — Locale-specific template content
- `SendRequest` — Email send with variables
- `InjectorRequest` — Injector definition with fields
- `SetInjectorValuesRequest` — Field value assignment
- `AdapterRequest` — Adapter configuration (SMTP, SES, etc.)
- `DomainRequest` — Domain registration with DKIM
- `APIKeyRequest` — API key creation
- `MemberRequest` — Member invitation with roles
- `WebhookRequest` — Webhook registration
- `SuppressionRequest` — Suppression list addition

#### Helper Functions
- `SampleMJML()` — Returns example MJML template for testing
- `DefaultVariableSchema()` — Returns JSON schema for template variables

---

### 3. happy_path_test.go (867 lines)
**Purpose:** Implementation of all 10 happy path test flows

**Test Functions:**

#### F01: TestF01_OnboardingComplete
- **Objective:** Verify onboarding process
- **Steps:** POST /onboarding → 201, GET /onboarding/status
- **Assertions:** tenant_id exists, status = "pending"
- **Lines:** ~40

#### F02: TestF02_SetupWorkspace
- **Objective:** Configure workspace infrastructure
- **Steps:** Create injectors → adapter (SMTP) → domain → verify domain
- **Assertions:** All return 201 or 409 (conflict OK if exists)
- **Lines:** ~60

#### F03: TestF03_TemplateLifecycle
- **Objective:** Complete template lifecycle
- **Steps:** Create type → template → version → locales (en, es) → publish → archive → new version → publish
- **Assertions:** Status transitions correct (draft → published → archived)
- **Lines:** ~130

#### F04: TestF04_SendEmailSuccess
- **Objective:** Send with delivery verification
- **Steps:** POST /send → 202, poll status until "delivered", verify in Mailpit, check DKIM
- **Assertions:** Email arrives with correct to/from/subject, DKIM header present
- **Lines:** ~90

#### F05: TestF05_BatchSend
- **Objective:** Send 50 emails and verify all deliver
- **Steps:** POST /send with 50 recipients, poll all, verify 50 in Mailpit
- **Assertions:** All 50 delivered, all 50 in Mailpit
- **Lines:** ~60

#### F06: TestF06_QueryByExternalID
- **Objective:** Query emails by external_id
- **Steps:** Send 5 with same external_id, GET /emails?external_id=X
- **Assertions:** Result >= 5, all have correct external_id
- **Lines:** ~50

#### F07: TestF07_InheritanceChain
- **Objective:** Verify template inheritance
- **Steps:** Send, verify workspace template used
- **Assertions:** template_resolved matches workspace slug
- **Lines:** ~30

#### F08: TestF08_InjectorMerge
- **Objective:** Injector field merging
- **Steps:** Create injector with 3 fields, set values, get and verify
- **Assertions:** Fields populated with correct values
- **Lines:** ~50

#### F09: TestF09_APIKeyLifecycle
- **Objective:** API key management
- **Steps:** Create → use (POST /send) → revoke → denied
- **Assertions:** Key works, revoked key returns 401
- **Lines:** ~70

#### F10: TestF10_MemberRoles
- **Objective:** Role-based access control
- **Steps:** Viewer (GET OK, POST forbidden), Editor (draft OK, publish forbidden), Admin (all OK)
- **Assertions:** Correct status codes for each role/action
- **Lines:** ~100

---

## Code Quality

### Patterns Used

1. **Subtests with t.Run()**
   ```go
   t.Run("POST /send send email with valid data", func(t *testing.T) {
       // ...
   })
   ```

2. **Immediate Status Assertion**
   ```go
   resp := client.Post(path, body)
   RequireStatus(t, resp, http.StatusAccepted)  // Fail if wrong status
   ```

3. **Safe JSON Parsing**
   ```go
   var respBody struct {
       Data struct {
           ID string `json:"id"`
       } `json:"data"`
   }
   ParseJSONResponse(t, resp, &respBody)
   ```

4. **Polling with Timeout**
   ```go
   client.WaitForEmailStatus(trackingID, "delivered", 5*time.Second)
   mailpit.WaitForMessages(50, 5*time.Second)
   ```

5. **Idempotency Handling**
   ```go
   require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict)
   ```

### Error Handling

- No `if err != nil { return }` patterns — all errors fail test immediately via `require`
- Comprehensive error logging in assertions with `RequireStatus()` which shows response body
- Standard error contract respected in error assertions

### Dependencies

**Imports:**
- `testing` — Go standard library
- `github.com/stretchr/testify/require` — Assertion helpers
- `net/http` — HTTP client + constants
- `encoding/json` — JSON marshaling
- `bytes`, `io`, `context`, `fmt`, `os`, `time` — Standard library utilities

**No external mocking frameworks** — All mocks are manual (TestClient wraps net/http)

---

## API Contract Coverage

### Authentication
- ✅ Bearer token (OIDC JWT) for management API
- ✅ API Key (X-API-Key header) for data plane

### Endpoints Tested

**Data Plane:**
- ✅ POST /api/v1/send — Send email (F04, F05, F06, F07, F08, F09)
- ✅ GET /api/v1/emails/:tracking_id — Get email status (F04, F05)
- ✅ GET /api/v1/emails — Query emails (F06)

**Management API:**
- ✅ POST /api/v1/manage/onboarding — Setup (F01)
- ✅ GET /api/v1/manage/onboarding/status — Status (F01)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/injectors — Create injector (F02, F08)
- ✅ PUT /api/v1/manage/tenants/:code/workspaces/:code/injectors/:name/values — Set values (F08)
- ✅ GET /api/v1/manage/tenants/:code/workspaces/:code/injectors/:name — Get injector (F08)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/adapters — Create adapter (F02)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/domains — Register domain (F02)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/domains/:id/verify — Verify domain (F02)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/template-types — Create type (F03, F10)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/templates — Create template (F03, F10)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/templates/:slug/versions — Create version (F03, F10)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/templates/:slug/versions/:num/locales — Add locale (F03)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/templates/:slug/versions/:num/publish — Publish (F03, F10)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/templates/:slug/versions/:num/archive — Archive (F03)
- ✅ POST /api/v1/manage/tenants/:code/workspaces/:code/api-keys — Create key (F09)
- ✅ DELETE /api/v1/manage/tenants/:code/workspaces/:code/api-keys/:id — Revoke key (F09)

### Response Contracts
- ✅ Success responses (200, 201, 202) with `{data: {...}}`
- ✅ List responses with pagination (`{data: {items, next_cursor, has_more}}`))
- ✅ Error responses with `{error: {code, message, details, request_id}}`
- ✅ HTTP status codes per spec (202 for send, 201 for creates, etc.)

---

## How to Run

### Prerequisites
1. Senda backend running on `http://localhost:8080`
2. PostgreSQL with migrations applied
3. Mailpit running on `http://localhost:8025`
4. River workers active

### Run All Tests
```bash
cd /sessions/friendly-kind-galileo/mnt/senda
go test -tags=e2e -v ./test/e2e/...
```

### Run Specific Flow
```bash
go test -tags=e2e -v -run TestF04_SendEmailSuccess ./test/e2e/...
```

### With Environment Variables
```bash
SENDA_BASE_URL=http://localhost:8080 \
MAILPIT_URL=http://localhost:8025 \
go test -tags=e2e -v ./test/e2e/...
```

### With Race Detection
```bash
go test -tags=e2e -race ./test/e2e/...
```

### With Timeout
```bash
go test -tags=e2e -timeout 120s ./test/e2e/...
```

---

## Key Features

### 1. Zero Mock Frameworks
- No gomock, mockery, or other code generation
- All helpers are manual wrappers around standard library
- Easy to understand and maintain

### 2. Async-Aware
- Built-in polling for email status (`WaitForEmailStatus()`)
- Built-in polling for Mailpit message arrival (`WaitForMessages()`)
- Configurable timeouts (default 5-10 seconds)

### 3. Real API Testing
- Tests hit actual HTTP endpoints
- No stubbed responses or recorded HTTP cassettes
- Verifies exact API contract (status codes, headers, JSON structure)

### 4. Idempotent
- Handles 409 Conflict (resource already exists)
- Can run multiple times without cleanup
- Safe with pre-existing data

### 5. Self-Contained
- Each test function is independent
- Can run individual tests with `-run` flag
- No shared state between tests (each creates own client)

### 6. Comprehensive Logging
- Response body printed on assertion failure
- Error details extracted and shown
- Tracking IDs and message IDs in test output

---

## Documentation

### README.md (11 KB)
- File structure and components overview
- How to run tests
- Test assertions and response parsing
- Best practices for extending tests
- Troubleshooting section

### FLOWS.md (20 KB)
- Detailed walkthrough of all 10 flows
- HTTP requests with expected status codes
- Code examples for each flow
- API response contracts
- Implementation notes and patterns

### IMPLEMENTATION_SUMMARY.md (This file)
- Overview of what was created
- File descriptions and line counts
- Code quality metrics
- API coverage matrix
- How to run and extend

---

## Lines of Code

```
helpers.go           381 lines (utilities)
seed.go              205 lines (constants and DTOs)
happy_path_test.go   867 lines (10 test functions)
─────────────────────────────────
Total:             1,453 lines of real, executable Go code
```

**Not counted:** chaos_test.go and error_flows_test.go (pre-existing)

---

## Test Functions Summary

| # | Function | Focus | Key Assertions |
|---|----------|-------|-----------------|
| F01 | TestF01_OnboardingComplete | Onboarding | tenant_id, status pending |
| F02 | TestF02_SetupWorkspace | Infrastructure | injector, adapter, domain setup |
| F03 | TestF03_TemplateLifecycle | Template versioning | type → version → publish → archive |
| F04 | TestF04_SendEmailSuccess | Send & delivery | 202 accepted, delivered status, Mailpit email |
| F05 | TestF05_BatchSend | Batch processing | 50 recipients, all delivered |
| F06 | TestF06_QueryByExternalID | Query API | external_id filtering, pagination |
| F07 | TestF07_InheritanceChain | Template resolution | workspace template precedence |
| F08 | TestF08_InjectorMerge | Injector merging | field override and merge verification |
| F09 | TestF09_APIKeyLifecycle | API Key security | create → use → revoke → denied |
| F10 | TestF10_MemberRoles | RBAC | viewer (R), editor (R+W draft), admin (R+W+publish) |

---

## Assertions per Test

| Test | RequireStatus | ParseJSON | Require Assertions | Subtests |
|------|---------------|-----------|-------------------|----------|
| F01  | 2             | 2         | 2                 | 2        |
| F02  | 4             | 2         | 4                 | 4        |
| F03  | 8             | 8         | 8                 | 8        |
| F04  | 5             | 2         | 6                 | 4        |
| F05  | 3             | 2         | 3                 | 3        |
| F06  | 2             | 2         | 4                 | 2        |
| F07  | 2             | 1         | 2                 | 1        |
| F08  | 3             | 2         | 3                 | 3        |
| F09  | 5             | 4         | 6                 | 4        |
| F10  | 6             | 0         | 9                 | 3        |
| **Total** | **40** | **25** | **47** | **34** |

---

## Next Steps (After Backend Ready)

1. **Run against live backend:**
   ```bash
   go test -tags=e2e -v ./test/e2e/ -run "^TestF"
   ```

2. **Fix any assertion failures:**
   - Check actual HTTP status codes vs expected
   - Verify response JSON structure matches DTOs
   - Adjust timeouts if async operations take longer

3. **Extend with error flows:**
   - Use error_flows_test.go as reference
   - Add E01-E12 error scenarios
   - Test rate limiting, validation, etc.

4. **Add chaos testing:**
   - Use chaos_test.go as reference
   - Test provider failures, DB reconnects, race conditions
   - Verify resilience and error recovery

5. **Generate Postman collection:**
   - Use happy path tests as reference
   - Document all endpoints with examples
   - Create environment variables file

---

## Success Criteria (✅ All Met)

- ✅ All code uses `//go:build e2e` tag
- ✅ Real Go test code, not pseudocode (executable with `go test`)
- ✅ Tests against API CONTRACT (spec §15), not implementation
- ✅ Self-contained test functions (no shared setup needed)
- ✅ Uses `testing` + `testify/require` for assertions
- ✅ Uses `net/http` for HTTP calls, `encoding/json` for parsing
- ✅ 10 happy path flows (F01-F10) fully implemented
- ✅ Helpers for HTTP client, auth, Mailpit, assertions
- ✅ Seed data constants for all test resources
- ✅ Subtests with `t.Run()` for logical grouping
- ✅ Comprehensive comments explaining what each step verifies
- ✅ No external mocking frameworks (manual mocks)
- ✅ All files in `/sessions/friendly-kind-galileo/mnt/senda/test/e2e/`
- ✅ Documentation: README.md, FLOWS.md, IMPLEMENTATION_SUMMARY.md

---

## Files Delivered

```
/sessions/friendly-kind-galileo/mnt/senda/test/e2e/
├── helpers.go                      [381 lines] Shared utilities
├── seed.go                         [205 lines] Constants & DTOs
├── happy_path_test.go              [867 lines] F01-F10 test functions
├── README.md                       [11 KB]    User guide
├── FLOWS.md                        [20 KB]    Detailed flow walkthroughs
└── IMPLEMENTATION_SUMMARY.md       [This file] Technical overview
```

**Total deliverable:** 1,453 lines of real Go test code + 31 KB documentation
