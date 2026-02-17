# Senda E2E Test Suite

This directory contains end-to-end (E2E) tests for the Senda email orchestration platform. Tests are organized by flow type and verify the complete happy path scenarios.

## Build Tag

All test files in this directory use the `//go:build e2e` build tag. To run these tests:

```bash
go test -tags=e2e ./test/e2e/...
```

## File Structure

- **helpers.go** — Shared HTTP client, auth utilities, Mailpit client, assertion helpers
- **seed.go** — Constants and data structures for test scenarios
- **happy_path_test.go** — All 10 happy path test flows (F01-F10)

## Test Files Overview

### helpers.go

Provides utilities for writing E2E tests:

- **TestClient** — HTTP client wrapper with auth header management
  - `NewTestClient(t *testing.T)` — Creates a test client
  - `Get/Post/Put/Delete` methods for HTTP operations
  - `SetBearerToken()` / `SetAPIKey()` — Set authentication
  - `WaitForEmailStatus()` — Poll email status with timeout

- **MailpitClient** — REST client for Mailpit email verification
  - `NewMailpitClient(t *testing.T)` — Creates a Mailpit client
  - `GetMessages()` — Fetch all messages
  - `GetMessage(id)` — Get a specific message
  - `SearchMessages(query)` — Search by query
  - `ClearMessages()` — Delete all messages
  - `AssertMessageExists(recipient)` — Find message by recipient
  - `WaitForMessages(count, timeout)` — Wait for N messages to arrive

- **Assertion Helpers**
  - `RequireStatus(t, resp, expectedStatus)` — Assert HTTP status code
  - `ParseJSON[T](t, resp)` — Unmarshal JSON response
  - `ParseError(t, resp)` — Extract error details
  - `AssertError(t, resp, expectedCode)` — Assert error code

### seed.go

Test data constants and DTOs:

- **Tenant/Workspace Constants**
  - TenantCode = "test-corp"
  - WorkspaceCode = "main"
  - SystemWorkspaceCode = "_system"

- **User Credentials**
  - SuperadminEmail
  - TenantAdminEmail
  - WorkspaceAdminEmail
  - WorkspaceEditorEmail
  - WorkspaceViewerEmail

- **Template Resources**
  - TemplateTypeSlug = "welcome-email"
  - TemplateSlug = "welcome-v1"

- **Infrastructure**
  - TestDomain = "mail.test.example.com"
  - MailpitSMTPHost = "mailpit"
  - MailpitSMTPPort = 1025

- **Request DTOs**
  - OnboardingRequest
  - TemplateTypeRequest
  - CreateTemplateRequest
  - SendRequest
  - APIKeyRequest
  - MemberRequest
  - WebhookRequest
  - And more...

## Happy Path Flows (F01-F10)

### F01: Onboarding Complete
**Objective:** Verify complete onboarding flow

**Steps:**
1. POST `/api/v1/manage/onboarding` with tenant + workspace + admin data → 201 Created
2. Verify response contains tenant ID and status = "pending"
3. GET `/api/v1/manage/onboarding/status` → 200 or 401 (depending on OIDC config)

**Files:** `happy_path_test.go::TestF01_OnboardingComplete()`

---

### F02: Setup Workspace
**Objective:** Configure workspace with injectors, adapter, and domain

**Steps:**
1. POST `/api/v1/manage/tenants/test-corp/workspaces/main/injectors` → 201 or 409
2. POST `/api/v1/manage/tenants/test-corp/workspaces/main/adapters` (SMTP → Mailpit) → 201 or 409
3. POST `/api/v1/manage/tenants/test-corp/workspaces/main/domains` → 201 or 409
4. POST `/api/v1/manage/tenants/test-corp/workspaces/main/domains/:id/verify` → 200 or 422

**Files:** `happy_path_test.go::TestF02_SetupWorkspace()`

---

### F03: Template Lifecycle
**Objective:** Complete template creation, versioning, and localization flow

**Steps:**
1. POST `/template-types` create type with variable schema → 201 or 409
2. POST `/templates` create template → 201 or 409
3. POST `/templates/:slug/versions` create draft → 201
4. POST `/templates/:slug/versions/:num/locales` add en locale → 201
5. POST `/templates/:slug/versions/:num/locales` add es locale → 201
6. POST `/templates/:slug/versions/:num/publish` publish → 200
7. POST `/templates/:slug/versions/:num/archive` archive → 200
8. POST `/templates/:slug/versions` create new version → 201
9. POST `/templates/:slug/versions/:num/publish` publish new → 200

**Files:** `happy_path_test.go::TestF03_TemplateLifecycle()`

---

### F04: Send Email Success
**Objective:** Full send flow with processing and delivery verification

**Steps:**
1. POST `/api/v1/send` with valid template + recipient + variables → 202 Accepted
2. Response contains tracking_id, status = "accepted"
3. Poll GET `/api/v1/emails/:tracking_id` until status = "delivered" (max 5s)
4. Query Mailpit `/api/v1/messages` → email arrived with correct recipient/subject
5. Verify DKIM-Signature header present in message

**Files:** `happy_path_test.go::TestF04_SendEmailSuccess()`

---

### F05: Batch Send
**Objective:** Send 50 emails in single request and verify all delivered

**Steps:**
1. POST `/api/v1/send` with 50 recipients → 202 Accepted
2. Response contains 50 tracking_ids in array
3. Poll all 50 tracking_ids until each reaches status = "delivered"
4. Query Mailpit → verify all 50 emails received (max 5s wait)

**Files:** `happy_path_test.go::TestF05_BatchSend()`

---

### F06: Query by External ID
**Objective:** Send 5 emails with same external_id and query them

**Steps:**
1. POST `/api/v1/send` with 5 recipients + external_id = "batch-xyz" → 202
2. GET `/api/v1/emails?external_id=batch-xyz` → 200 OK
3. Response items has >= 5 results
4. All items have external_id = "batch-xyz"

**Files:** `happy_path_test.go::TestF06_QueryByExternalID()`

---

### F07: Inheritance Chain
**Objective:** Verify template inheritance from workspace > _system > global

**Steps:**
1. Send email using workspace template
2. Verify response.template_resolved = workspace template slug
3. Email rendered with workspace-level content

**Files:** `happy_path_test.go::TestF07_InheritanceChain()`

---

### F08: Injector Merge
**Objective:** Verify injector field merging (global + workspace override)

**Steps:**
1. POST `/injectors` with 3 fields → 201 or 409
2. PUT `/injectors/:name/values` set values for field1, field2, field3 → 200
3. GET `/injectors/:name` → verify fields populated
4. (Optional: send email and verify merged values in body)

**Files:** `happy_path_test.go::TestF08_InjectorMerge()`

---

### F09: API Key Lifecycle
**Objective:** Create, use, revoke API Key with proper access control

**Steps:**
1. POST `/api-keys` → 201 Created, response includes key/token
2. POST `/api/v1/send` with API Key header → 202 Accepted
3. DELETE `/api-keys/:id` → 204 No Content
4. POST `/api/v1/send` with revoked key → 401 Unauthorized

**Files:** `happy_path_test.go::TestF09_APIKeyLifecycle()`

---

### F10: Member Roles
**Objective:** Verify role-based access control (viewer, editor, admin)

**Steps:**
1. Workspace-viewer: GET `/template-types` → 200 ✓, POST `/template-types` → 403 ✗
2. Workspace-editor: POST `/templates/:slug/versions` (draft) → 200 ✓, POST `/publish` → 403 ✗
3. Workspace-admin: POST `/template-types` → 201 ✓, POST `/templates` → 201 ✓, POST `/publish` → 200 ✓

**Files:** `happy_path_test.go::TestF10_MemberRoles()`

---

## Running the Tests

### Prerequisites

1. **Senda Backend Running**
   ```bash
   docker-compose up -d  # Assuming docker-compose.yml in root
   ```

2. **Environment Variables**
   ```bash
   export SENDA_BASE_URL=http://localhost:8080
   export MAILPIT_URL=http://localhost:8025
   ```

3. **Dependencies**
   ```bash
   go mod download
   ```

### Run All E2E Tests

```bash
go test -tags=e2e -v ./test/e2e/...
```

### Run Specific Test

```bash
go test -tags=e2e -v -run TestF04_SendEmailSuccess ./test/e2e/...
```

### Run with Verbose Output

```bash
go test -tags=e2e -v -race ./test/e2e/...
```

### Run with Timeout

```bash
go test -tags=e2e -timeout 60s ./test/e2e/...
```

## Test Assertions

All tests use `testify/require` for assertions, which immediately fail the test on assertion failure:

```go
require.Equal(t, expected, actual)
require.NoError(t, err)
require.True(t, condition)
require.Contains(t, actual, substring)
```

## Response Parsing

JSON responses are parsed using generic helper:

```go
var resp struct {
    Data struct {
        ID string `json:"id"`
    } `json:"data"`
}
ParseJSONResponse(t, httpResp, &resp)
```

Or using generic type:

```go
data := ParseJSON[MyType](t, httpResp)
```

## Mailpit Integration

Mailpit acts as a fake SMTP server for testing. All emails sent during tests are captured and queryable via REST API:

```go
mailpit := NewMailpitClient(t)
mailpit.ClearMessages()  // Start fresh

// ... send emails ...

messages := mailpit.GetMessages()
msg := mailpit.AssertMessageExists("recipient@example.com")
```

## Error Handling

Error responses follow the standard contract:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "...",
    "details": [...],
    "request_id": "..."
  }
}
```

Extract with helper:

```go
errResp := ParseError(t, resp)
AssertError(t, resp, "VALIDATION_ERROR")
```

## Best Practices

1. **Use constants** from `seed.go` for all test data
2. **Clear Mailpit** at start of test to avoid cross-test pollution
3. **Use subtests** for logical groupings within a flow
4. **Add descriptive comments** explaining what each step verifies
5. **Poll with timeout** for async operations (email delivery, domain verification)
6. **Assert HTTP status codes** first before parsing body
7. **Use `t.Run()`** for nested test scopes
8. **Set auth** via `SetBearerToken()` or `SetAPIKey()` before making requests

## Extending Tests

To add new test flows:

1. Create a new test function in `happy_path_test.go`
2. Use naming convention `TestFXX_DescriptiveName()`
3. Add seed data constants to `seed.go` if needed
4. Use helpers from `helpers.go` for HTTP and Mailpit operations
5. Include subtests with `t.Run()` for each logical step
6. Document the flow in this README

## Troubleshooting

### Tests timeout waiting for email

**Cause:** River workers not processing, or email status not updating

**Solution:**
1. Check docker-compose logs for worker errors
2. Verify adapter (SMTP) is configured correctly
3. Ensure domain is "verified" (or skip verification in test env)

### 401 Unauthorized on management endpoints

**Cause:** OIDC token not set or expired

**Solution:**
1. Mock OIDC token in test setup
2. Verify `SENDA_OIDC_DISCOVERY_URL` is configured
3. Use `client.SetBearerToken()` before management API calls

### Mailpit messages not found

**Cause:** Wrong Mailpit URL or adapter not using Mailpit SMTP

**Solution:**
1. Verify `MAILPIT_URL` environment variable
2. Check adapter config points to `mailpit:1025`
3. Clear previous messages with `mailpit.ClearMessages()` at test start

### "Email not found by recipient"

**Cause:** Email address doesn't match or recipient lookup is exact

**Solution:**
1. Use exact email address from SendRequest
2. Check Mailpit has the message (GetMessages())
3. Verify To[] array contains the recipient
