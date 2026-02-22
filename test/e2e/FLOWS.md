# Happy Path Test Flows (F01-F10)

## Quick Reference

All 10 happy path test flows are implemented as real Go test functions using the `testing` package, testify/require, and standard net/http.

| Flow | Test Function | Purpose | Key Assertions |
|------|---------------|---------|-----------------|
| F01 | `TestF01_OnboardingComplete` | Onboarding setup | POST /onboarding → 201, GET /onboarding/status → 200/401 |
| F02 | `TestF02_SetupWorkspace` | Workspace config | Injectors, Adapter, Domain setup → 201/409 |
| F03 | `TestF03_TemplateLifecycle` | Template full lifecycle | Create type/template/version/locales → publish → archive → new version |
| F04 | `TestF04_SendEmailSuccess` | Send with delivery | POST /send → 202, poll email status → delivered, verify in Mailpit |
| F05 | `TestF05_BatchSend` | Batch sending | Send 50 recipients, all delivered, all in Mailpit |
| F06 | `TestF06_QueryByExternalID` | Query by external_id | Send 5 with same external_id, GET /emails?external_id=X → 5+ results |
| F07 | `TestF07_InheritanceChain` | Template inheritance | Workspace template used over global, correct resolution |
| F08 | `TestF08_InjectorMerge` | Injector field merge | Define 3 fields, override 1, merge verified |
| F09 | `TestF09_APIKeyLifecycle` | API Key management | Create → use → revoke → blocked |
| F10 | `TestF10_MemberRoles` | Role-based access | Viewer (R), Editor (R+W draft), Admin (R+W+publish) |

---

## Flow Details

### F01: Onboarding Complete

**Location:** `happy_path_test.go::TestF01_OnboardingComplete()`

**Objective:** Verify end-to-end onboarding process for new tenant setup

**HTTP Requests:**
1. `POST /api/v1/manage/onboarding`
   - Body: tenant code, name, workspace code, name, admin email
   - Expected: 201 Created
   - Verify: response.data.tenant_id exists, status = "pending"

2. `GET /api/v1/manage/onboarding/status`
   - Expected: 200 OK or 401 (if OIDC not mocked)
   - Verify: response structure valid

**Key Code:**
```go
func TestF01_OnboardingComplete(t *testing.T) {
    client := NewTestClient(t)

    req := OnboardingRequest{
        AdminEmail:    SuperadminEmail,
        TenantCode:    TenantCode,
        WorkspaceCode: WorkspaceCode,
        // ...
    }

    resp := client.Post("/api/v1/manage/onboarding", req)
    RequireStatus(t, resp, http.StatusCreated)
    // ...
}
```

**Auth:** Depends on test setup (may require OIDC token or be unauthenticated for first-time setup)

---

### F02: Setup Workspace

**Location:** `happy_path_test.go::TestF02_SetupWorkspace()`

**Objective:** Configure workspace infrastructure (injectors, adapter)

**HTTP Requests:**
1. `POST /api/v1/manage/tenants/test-corp/workspaces/main/injectors`
   - Body: name, description, fields array
   - Expected: 201 Created or 409 Conflict (if exists)

2. `POST /api/v1/manage/tenants/test-corp/workspaces/main/adapters`
   - Body: type="smtp", config with mailpit host/port
   - Expected: 201 Created or 409 Conflict

**Note:** Domain registration and DKIM verification are not part of the application flow. Email authentication (SPF, DKIM, DMARC) is handled natively by the delivery provider. Senda validates sender identity through the provider's verification system (e.g., verified emails/domains in SES, OAuth scopes in Gmail).

**Key Code:**
```go
func TestF02_SetupWorkspace(t *testing.T) {
    tenantPath := "/api/v1/manage/tenants/test-corp/workspaces/main"

    // Create injector with fields
    req := InjectorRequest{Name: "global-vars", Fields: [...]}
    resp := client.Post(tenantPath+"/injectors", req)
    require.True(t, resp.StatusCode == 201 || resp.StatusCode == 409)

    // Create SMTP adapter pointing to Mailpit
    adapterReq := AdapterRequest{Type: "smtp", Config: {"host": "mailpit"}}
    resp = client.Post(tenantPath+"/adapters", adapterReq)
}
```

**Auth:** Bearer token (workspace-admin or tenant-admin role)

---

### F03: Template Lifecycle

**Location:** `happy_path_test.go::TestF03_TemplateLifecycle()`

**Objective:** Complete journey from template type creation through versioning, localization, publishing, and archiving

**HTTP Requests:**
1. `POST /api/v1/manage/tenants/test-corp/workspaces/main/template-types`
   - Body: slug, name, variable_schema
   - Expected: 201 Created or 409

2. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates`
   - Body: slug, template_type_id or template_type_slug
   - Expected: 201 Created or 409

3. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates/welcome-v1/versions`
   - Body: subject, preview_text, from_email, body_mjml
   - Expected: 201 Created
   - Verify: version_number, status = "draft"

4. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates/welcome-v1/versions/1/locales`
   - Body: locale="en", subject, body_mjml
   - Expected: 201 Created
   - Repeat for locale="es"

5. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates/welcome-v1/versions/1/publish`
   - Body: null/empty
   - Expected: 200 OK
   - Verify: status = "published"

6. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates/welcome-v1/versions/1/archive`
   - Body: null/empty
   - Expected: 200 OK
   - Verify: status = "archived"

7. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates/welcome-v1/versions` (new)
   - Expected: 201 with version_number = 2

8. `POST /api/v1/manage/tenants/test-corp/workspaces/main/templates/welcome-v1/versions/2/publish`
   - Expected: 200 OK

**Key Code:**
```go
func TestF03_TemplateLifecycle(t *testing.T) {
    // Create type
    typeReq := TemplateTypeRequest{Slug: "welcome-email", ...}
    resp := client.Post(tenantPath+"/template-types", typeReq)

    // Create template
    tplReq := CreateTemplateRequest{Slug: "welcome-v1", ...}
    resp = client.Post(tenantPath+"/templates", tplReq)

    // Create version
    verReq := CreateVersionRequest{Subject: "Welcome!", ...}
    resp = client.Post(tenantPath+"/templates/welcome-v1/versions", verReq)

    // Add locales
    locReq := CreateLocaleRequest{Locale: "en", ...}
    resp = client.Post(tenantPath+"/templates/welcome-v1/versions/1/locales", locReq)

    // Publish
    resp = client.Post(tenantPath+"/templates/welcome-v1/versions/1/publish", nil)
    RequireStatus(t, resp, http.StatusOK)
}
```

**Auth:** Bearer token (workspace-admin for publish, workspace-editor for draft)

---

### F04: Send Email Success

**Location:** `happy_path_test.go::TestF04_SendEmailSuccess()`

**Objective:** Complete send flow with delivery verification in Mailpit

**HTTP Requests:**
1. `POST /api/v1/send` (Data Plane API)
   - Body: ref="welcome-email/welcome-v1", to=["recipient@test.example.com"], variables, external_id
   - Expected: 202 Accepted
   - Verify: status="accepted", tracking_ids[0].tracking_id exists, template_version=1

2. `GET /api/v1/emails/:tracking_id` (polling)
   - Expected: 200 OK
   - Poll until: status = "delivered" (max 5s timeout)

3. `GET http://mailpit:8025/api/v1/messages` (Mailpit API)
   - Expected: 200 OK with messages array
   - Verify: message.To contains "recipient@test.example.com"

4. Verify Mailpit message:
   - Check: From = test from_email
   - Check: Subject contains expected text
   - Check: HTML body not empty

**Key Code:**
```go
func TestF04_SendEmailSuccess(t *testing.T) {
    client := NewTestClient(t)
    mailpit := NewMailpitClient(t)
    mailpit.ClearMessages()

    // Send
    req := SendRequest{
        Ref: "welcome-email/welcome-v1",
        To: []string{"recipient@test.example.com"},
        Variables: map[string]interface{}{
            "first_name": "John",
            "company_name": "Test Corp",
        },
    }
    resp := client.Post("/api/v1/send", req)
    RequireStatus(t, resp, http.StatusAccepted)

    var sendResp struct {
        Data struct {
            TrackingIDs []struct{ TrackingID string }
        }
    }
    ParseJSONResponse(t, resp, &sendResp)
    trackingID := sendResp.Data.TrackingIDs[0].TrackingID

    // Poll email status
    client.WaitForEmailStatus(trackingID, "delivered", 5*time.Second)

    // Verify in Mailpit
    msg := mailpit.AssertMessageExists("recipient@test.example.com")
    require.Equal(t, TestFromEmail, msg.From)
    require.NotEmpty(t, msg.HTML)
}
```

**Auth:** API Key or Bearer token

---

### F05: Batch Send

**Location:** `happy_path_test.go::TestF05_BatchSend()`

**Objective:** Send 50 emails in single request, verify all deliver and arrive

**HTTP Requests:**
1. `POST /api/v1/send` with 50 recipients
   - Body: to=["user0@...", "user1@...", ..., "user49@..."]
   - Expected: 202 Accepted
   - Verify: tracking_ids array length = 50, all non-empty

2. Poll all 50 tracking_ids for status = "delivered"
   - Each: `GET /api/v1/emails/:tracking_id` until delivered
   - Timeout: 10 seconds

3. `GET http://mailpit:8025/api/v1/messages`
   - Verify: >= 50 messages received
   - Timeout: 5 seconds

**Key Code:**
```go
func TestF05_BatchSend(t *testing.T) {
    recipients := make([]string, 50)
    for i := 0; i < 50; i++ {
        recipients[i] = fmt.Sprintf("user%d@test.example.com", i)
    }

    req := SendRequest{
        Ref: "welcome-email/welcome-v1",
        To: recipients,
        Variables: map[string]interface{}{...},
    }
    resp := client.Post("/api/v1/send", req)

    // Extract and poll all tracking IDs
    var trackingIDs []string
    for _, entry := range respBody.Data.TrackingIDs {
        trackingIDs = append(trackingIDs, entry.TrackingID)
        client.WaitForEmailStatus(entry.TrackingID, "delivered", 10*time.Second)
    }

    // Verify all in Mailpit
    mailpit.WaitForMessages(50, 5*time.Second)
}
```

**Auth:** API Key or Bearer token

---

### F06: Query by External ID

**Location:** `happy_path_test.go::TestF06_QueryByExternalID()`

**Objective:** Send 5 emails with same external_id, query and verify all returned

**HTTP Requests:**
1. `POST /api/v1/send` with 5 recipients, external_id="batch-xyz"
   - Expected: 202 Accepted

2. `GET /api/v1/emails?external_id=batch-xyz`
   - Expected: 200 OK
   - Verify: items array length >= 5
   - Verify: all items have external_id = "batch-xyz"

**Key Code:**
```go
func TestF06_QueryByExternalID(t *testing.T) {
    externalID := fmt.Sprintf("batch-%d", time.Now().Unix())

    // Send 5 with same external_id
    req := SendRequest{
        Ref: "welcome-email/welcome-v1",
        To: []string{...5 emails...},
        ExternalID: externalID,
    }
    resp := client.Post("/api/v1/send", req)
    RequireStatus(t, resp, http.StatusAccepted)

    time.Sleep(500 * time.Millisecond)

    // Query by external_id
    resp = client.Get(fmt.Sprintf("/api/v1/emails?external_id=%s", externalID))
    RequireStatus(t, resp, http.StatusOK)

    var queryResp struct {
        Data struct {
            Items []struct {
                ExternalID string `json:"external_id"`
            } `json:"items"`
        }
    }
    ParseJSONResponse(t, resp, &queryResp)

    require.GreaterOrEqual(t, len(queryResp.Data.Items), 5)
    for _, item := range queryResp.Data.Items {
        require.Equal(t, externalID, item.ExternalID)
    }
}
```

**Auth:** API Key or Bearer token

---

### F07: Inheritance Chain

**Location:** `happy_path_test.go::TestF07_InheritanceChain()`

**Objective:** Verify that workspace template is used over global template

**HTTP Requests:**
1. `POST /api/v1/send` with template in workspace
   - Expected: 202 Accepted
   - Verify: template_resolved matches workspace template slug
   - Verify: template_version is workspace version

**Key Code:**
```go
func TestF07_InheritanceChain(t *testing.T) {
    req := SendRequest{
        Ref: "welcome-email/welcome-v1",
        To: []string{"inherit-test@test.example.com"},
        Variables: map[string]interface{}{...},
    }

    resp := client.Post("/api/v1/send", req)
    RequireStatus(t, resp, http.StatusAccepted)

    var sendResp struct {
        Data struct {
            TemplateResolved string `json:"template_resolved"`
            TemplateVersion int `json:"template_version"`
        }
    }
    ParseJSONResponse(t, resp, &sendResp)

    require.Contains(t, sendResp.Data.TemplateResolved, "welcome-v1")
}
```

**Auth:** API Key or Bearer token

---

### F08: Injector Merge

**Location:** `happy_path_test.go::TestF08_InjectorMerge()`

**Objective:** Define injector with 3 fields, override 1, verify merge

**HTTP Requests:**
1. `POST /api/v1/manage/tenants/test-corp/workspaces/main/injectors`
   - Body: 3 fields (field1, field2, field3)
   - Expected: 201 or 409

2. `PUT /api/v1/manage/tenants/test-corp/workspaces/main/injectors/merge-test-injector/values`
   - Body: {"field1": "value1", "field2": "value2", "field3": "override"}
   - Expected: 200 OK or 201 Created

3. `GET /api/v1/manage/tenants/test-corp/workspaces/main/injectors/merge-test-injector`
   - Expected: 200 OK
   - Verify: fields populated with correct values

**Key Code:**
```go
func TestF08_InjectorMerge(t *testing.T) {
    // Create injector with 3 fields
    req := InjectorRequest{
        Name: "merge-test-injector",
        Fields: []InjectorFieldRequest{
            {FieldName: "field1", FieldType: "text"},
            {FieldName: "field2", FieldType: "text"},
            {FieldName: "field3", FieldType: "text"},
        },
    }
    resp := client.Post(tenantPath+"/injectors", req)

    // Set values
    values := SetInjectorValuesRequest{
        Values: map[string]interface{}{
            "field1": "value1",
            "field2": "value2",
            "field3": "override",
        },
    }
    resp = client.Put(tenantPath+"/injectors/merge-test-injector/values", values)
    RequireStatus(t, resp, http.StatusOK)

    // Verify
    resp = client.Get(tenantPath + "/injectors/merge-test-injector")
    RequireStatus(t, resp, http.StatusOK)
}
```

**Auth:** Bearer token (workspace-admin)

---

### F09: API Key Lifecycle

**Location:** `happy_path_test.go::TestF09_APIKeyLifecycle()`

**Objective:** Create, use, revoke API key with proper access control

**HTTP Requests:**
1. `POST /api/v1/manage/tenants/test-corp/workspaces/main/api-keys`
   - Body: name="test-key-xyz"
   - Expected: 201 Created
   - Verify: response.data.key or response.data.token exists

2. `POST /api/v1/send` with X-API-Key header
   - Auth: X-API-Key=<key_from_step_1>
   - Expected: 202 Accepted

3. `DELETE /api/v1/manage/tenants/test-corp/workspaces/main/api-keys/:id`
   - Expected: 204 No Content

4. `POST /api/v1/send` with revoked key
   - Auth: X-API-Key=<revoked_key>
   - Expected: 401 Unauthorized

**Key Code:**
```go
func TestF09_APIKeyLifecycle(t *testing.T) {
    // Create
    req := APIKeyRequest{Name: "test-key-" + fmt.Sprintf("%d", time.Now().Unix())}
    resp := client.Post(tenantPath+"/api-keys", req)
    RequireStatus(t, resp, http.StatusCreated)

    var createResp struct {
        Data struct {
            ID string `json:"id"`
            Key string `json:"key"`
        }
    }
    ParseJSONResponse(t, resp, &createResp)
    apiKey := createResp.Data.Key

    // Use
    sendClient := NewTestClient(t)
    sendClient.SetAPIKey(apiKey)
    sendResp := sendClient.Post("/api/v1/send", SendRequest{...})
    RequireStatus(t, sendResp, http.StatusAccepted)

    // Revoke
    deleteResp := client.Delete(tenantPath + "/api-keys/" + createResp.Data.ID)
    RequireStatus(t, deleteResp, http.StatusNoContent)

    // Verify revoked
    sendResp = sendClient.Post("/api/v1/send", SendRequest{...})
    RequireStatus(t, sendResp, http.StatusUnauthorized)
}
```

**Auth:** Bearer token (workspace-admin) for API key management, API Key for send

---

### F10: Member Roles

**Location:** `happy_path_test.go::TestF10_MemberRoles()`

**Objective:** Verify role-based access control for workspace-viewer, workspace-editor, workspace-admin

**Test Cases:**

**Case 1: Workspace-Viewer**
- `GET /template-types` → 200 OK (can read)
- `POST /template-types` → 403 Forbidden (cannot write)

**Case 2: Workspace-Editor**
- `POST /templates/:slug/versions` (draft) → 201 Created (can create draft)
- `POST /templates/:slug/versions/:num/publish` → 403 Forbidden (cannot publish)

**Case 3: Workspace-Admin**
- `POST /template-types` → 201 Created (can create)
- `POST /templates` → 201 Created (can create)
- `POST /templates/:slug/versions/:num/publish` → 200 OK (can publish)

**Key Code:**
```go
func TestF10_MemberRoles(t *testing.T) {
    tenantPath := "/api/v1/manage/tenants/test-corp/workspaces/main"

    // Viewer: can GET, cannot POST
    getResp := client.Get(tenantPath + "/template-types")
    require.Equal(t, http.StatusOK, getResp.StatusCode)

    postResp := client.Post(tenantPath+"/template-types", TemplateTypeRequest{...})
    require.Equal(t, http.StatusForbidden, postResp.StatusCode)

    // Editor: can create draft, cannot publish
    verResp := client.Post(tenantPath+"/templates/welcome-v1/versions", CreateVersionRequest{...})
    require.Equal(t, http.StatusCreated, verResp.StatusCode)

    pubResp := client.Post(tenantPath+"/templates/welcome-v1/versions/1/publish", nil)
    require.Equal(t, http.StatusForbidden, pubResp.StatusCode)

    // Admin: can do everything
    typeResp := client.Post(tenantPath+"/template-types", TemplateTypeRequest{...})
    require.Equal(t, http.StatusCreated, typeResp.StatusCode)
}
```

**Auth:** Bearer token with appropriate role

---

## Implementation Notes

### General Patterns

1. **Subtests:** Each flow uses `t.Run()` for logical grouping
2. **Status Assertions:** Always check HTTP status first with `RequireStatus()`
3. **JSON Parsing:** Use generic `ParseJSON[T]()` or specific struct with `ParseJSONResponse()`
4. **Polling:** Use `WaitForEmailStatus()` or `WaitForMessages()` for async operations
5. **Error Handling:** Use `require.*()` which fails test immediately on assertion failure
6. **Idempotency:** Tests handle 409 Conflict responses (resource already exists)

### Test Isolation

- **Mailpit Cleanup:** Call `mailpit.ClearMessages()` at start of tests that verify emails
- **Unique Names:** Use timestamp or random suffix for resource names to avoid conflicts
- **External IDs:** Use timestamp-based external_id for batch and query tests

### Assertion Pattern

```go
resp := client.Post(path, body)
RequireStatus(t, resp, http.StatusExpected)

var respBody struct {
    Data struct {
        // fields
    } `json:"data"`
}
ParseJSONResponse(t, resp, &respBody)

// Assert response fields
require.NotEmpty(t, respBody.Data.ID)
require.Equal(t, "expected", respBody.Data.Field)
```

---

## Running Tests

```bash
# Run all happy path tests
go test -tags=e2e -v ./test/e2e/ -run "^TestF"

# Run specific flow
go test -tags=e2e -v ./test/e2e/ -run "TestF04_SendEmailSuccess"

# Run with race detection
go test -tags=e2e -race ./test/e2e/...

# Run with timeout
go test -tags=e2e -timeout 60s ./test/e2e/...
```

---

## Dependencies

- `testing` — Go standard testing package
- `github.com/stretchr/testify/require` — Assertion helpers
- `net/http` — HTTP client and status constants
- `encoding/json` — JSON marshaling/unmarshaling
- `os` — Environment variable access
- `time` — Timeouts and durations

No external mocking frameworks. All mocks are manual.

---

## API Response Contracts

### Success Response (200, 201, 202)

```json
{
  "data": {
    "id": "...",
    // resource fields
  }
}
```

### List Response

```json
{
  "data": {
    "items": [...],
    "next_cursor": "...",
    "has_more": true
  }
}
```

### Error Response (4xx, 5xx)

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": [...],
    "request_id": "..."
  }
}
```

---

## Mailpit API

Mailpit exposes a REST API for email verification during tests:

- `GET /api/v1/messages` — List all messages
- `GET /api/v1/message/:id` — Get full message with body
- `DELETE /api/v1/messages` — Delete all messages
- `GET /api/v1/search?query=...` — Search messages

All requests made via `MailpitClient` helper.
