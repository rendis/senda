# E2E Test Suite — Quick Start

## 30-Second Overview

✅ **10 Happy Path Tests** — All flows F01-F10 implemented in real Go code
✅ **Ready to Run** — Against any Senda backend + Mailpit setup
✅ **No Mocks** — Standard HTTP client, real API contract testing
✅ **Comprehensive** — 1,453 lines of executable test code + full documentation

---

## Run Tests

### Basic
```bash
go test -tags=e2e -v ./test/e2e/...
```

### Single Flow
```bash
go test -tags=e2e -v -run TestF04_SendEmailSuccess ./test/e2e/...
```

### With Environment
```bash
SENDA_BASE_URL=http://localhost:8080 \
MAILPIT_URL=http://localhost:8025 \
go test -tags=e2e -v ./test/e2e/...
```

### With Timeout
```bash
go test -tags=e2e -timeout 120s ./test/e2e/...
```

---

## Files

| File | Size | Purpose |
|------|------|---------|
| `helpers.go` | 381 lines | HTTP client, Mailpit, assertions |
| `seed.go` | 205 lines | Test data constants, DTOs |
| `happy_path_test.go` | 867 lines | 10 test functions (F01-F10) |
| `README.md` | 11 KB | Full user guide |
| `FLOWS.md` | 20 KB | Detailed test walkthroughs |

---

## Test Functions

```
TestF01_OnboardingComplete()    — Setup tenant + workspace
TestF02_SetupWorkspace()        — Configure injectors, adapter, domain
TestF03_TemplateLifecycle()     — Create type → version → locales → publish → archive
TestF04_SendEmailSuccess()      — Send → poll delivered → verify in Mailpit
TestF05_BatchSend()             — Send 50, all deliver
TestF06_QueryByExternalID()     — Query emails by external_id
TestF07_InheritanceChain()      — Template inheritance verification
TestF08_InjectorMerge()         — Injector field merging
TestF09_APIKeyLifecycle()       — Create → use → revoke → denied
TestF10_MemberRoles()           — RBAC (viewer, editor, admin)
```

---

## Key Helpers

### HTTP Client
```go
client := NewTestClient(t)
client.SetBearerToken(token)  // For management API
client.SetAPIKey(key)         // For data plane

resp := client.Post("/api/v1/send", body)
RequireStatus(t, resp, http.StatusAccepted)
```

### JSON Parsing
```go
var resp struct {
    Data struct {
        ID string `json:"id"`
    } `json:"data"`
}
ParseJSONResponse(t, resp, &resp)
```

### Mailpit (Email Verification)
```go
mailpit := NewMailpitClient(t)
mailpit.ClearMessages()
mailpit.WaitForMessages(50, 5*time.Second)
msg := mailpit.AssertMessageExists("user@example.com")
```

### Polling
```go
client.WaitForEmailStatus(trackingID, "delivered", 5*time.Second)
```

---

## Test Pattern

```go
func TestF04_SendEmailSuccess(t *testing.T) {
    client := NewTestClient(t)
    mailpit := NewMailpitClient(t)

    t.Run("POST /send", func(t *testing.T) {
        resp := client.Post("/api/v1/send", req)
        RequireStatus(t, resp, http.StatusAccepted)

        var body struct{ Data struct{ TrackingIDs []struct{ TrackingID string } } }
        ParseJSONResponse(t, resp, &body)

        trackingID := body.Data.TrackingIDs[0].TrackingID
        require.NotEmpty(t, trackingID)
    })

    t.Run("Poll email status", func(t *testing.T) {
        client.WaitForEmailStatus(trackingID, "delivered", 5*time.Second)
    })

    t.Run("Verify in Mailpit", func(t *testing.T) {
        msg := mailpit.AssertMessageExists("recipient@test.example.com")
        require.Equal(t, "noreply@mail.test.example.com", msg.From)
    })
}
```

---

## Dependencies

- `testing` — Standard Go testing
- `github.com/stretchr/testify/require` — Assertions
- `net/http` — HTTP client
- `encoding/json` — JSON parsing
- No mock frameworks (manual HTTP wrapper)

---

## API Endpoints Tested

**Data Plane:**
- `POST /api/v1/send`
- `GET /api/v1/emails/:tracking_id`
- `GET /api/v1/emails?external_id=X`

**Management API:**
- `POST /api/v1/manage/onboarding`
- `GET /api/v1/manage/onboarding/status`
- `POST .../injectors`, `PUT .../injectors/:name/values`, `GET .../injectors/:name`
- `POST .../adapters`
- `POST .../domains`, `POST .../domains/:id/verify`
- `POST .../template-types`
- `POST .../templates`, `POST .../templates/:slug/versions`
- `POST .../templates/:slug/versions/:num/locales`
- `POST .../templates/:slug/versions/:num/publish`
- `POST .../templates/:slug/versions/:num/archive`
- `POST .../api-keys`, `DELETE .../api-keys/:id`

---

## Authentication

**Management API:** Bearer token (OIDC JWT)
```go
client.SetBearerToken(jwtToken)
```

**Data Plane:** API Key
```go
client.SetAPIKey(apiKeyValue)
```

---

## Common Assertion Patterns

```go
// Status code
RequireStatus(t, resp, http.StatusAccepted)

// Non-empty field
require.NotEmpty(t, value)

// Exact match
require.Equal(t, expected, actual)

// Contains substring
require.Contains(t, str, substring)

// Greater than
require.Greater(t, actual, expected)

// True/False
require.True(t, condition)

// Error is nil
require.NoError(t, err)
```

---

## Troubleshooting

### Tests timeout
→ Check Senda/Mailpit running and accessible
→ Verify environment variables `SENDA_BASE_URL`, `MAILPIT_URL`

### 401 Unauthorized
→ OIDC not configured or token not set
→ Use `client.SetBearerToken()` before management API calls

### No messages in Mailpit
→ Verify adapter config points to `mailpit:1025`
→ Check River workers are processing
→ Call `mailpit.ClearMessages()` to start fresh

### Assertion failures
→ Check response body printed in failure message
→ Verify response JSON structure matches DTO
→ Add more subtests for narrower scope

---

## Examples

### Send Email Test
```go
// 1. Setup
client := NewTestClient(t)
mailpit := NewMailpitClient(t)
mailpit.ClearMessages()

// 2. Send
resp := client.Post("/api/v1/send", SendRequest{
    Ref: "welcome-email/welcome-v1",
    To: []string{"user@example.com"},
    Variables: map[string]interface{}{
        "first_name": "John",
        "company_name": "Test Corp",
    },
})
RequireStatus(t, resp, http.StatusAccepted)

// 3. Extract tracking ID
var sendResp struct{ Data struct{ TrackingIDs []struct{ TrackingID string } } }
ParseJSONResponse(t, resp, &sendResp)
trackingID := sendResp.Data.TrackingIDs[0].TrackingID

// 4. Poll delivered
client.WaitForEmailStatus(trackingID, "delivered", 5*time.Second)

// 5. Verify in Mailpit
msg := mailpit.AssertMessageExists("user@example.com")
require.Equal(t, "noreply@mail.test.example.com", msg.From)
require.Contains(t, msg.Subject, "Welcome")
```

### Template Creation Test
```go
client := NewTestClient(t)
path := "/api/v1/manage/tenants/test-corp/workspaces/main"

// Create type
resp := client.Post(path+"/template-types", TemplateTypeRequest{
    Slug: "welcome-email",
    Name: "Welcome Email",
    VariableSchema: DefaultVariableSchema(),
})
RequireStatus(t, resp, http.StatusCreated)

// Create template
resp = client.Post(path+"/templates", CreateTemplateRequest{
    Slug: "welcome-v1",
    TemplateTypeSlug: "welcome-email",
})
RequireStatus(t, resp, http.StatusCreated)

// Create version
resp = client.Post(path+"/templates/welcome-v1/versions", CreateVersionRequest{
    Subject: "Welcome!",
    FromEmail: "noreply@mail.test.example.com",
    BodyMJML: SampleMJML(),
})
RequireStatus(t, resp, http.StatusCreated)

// Publish
resp = client.Post(path+"/templates/welcome-v1/versions/1/publish", nil)
RequireStatus(t, resp, http.StatusOK)
```

---

## Coverage Summary

✅ Onboarding flow (F01)
✅ Workspace setup (F02)
✅ Template lifecycle (F03)
✅ Email sending & delivery (F04-F05)
✅ Email querying (F06)
✅ Template inheritance (F07)
✅ Injector merging (F08)
✅ API key management (F09)
✅ Role-based access (F10)

**Total:** 1,453 lines of real Go code + full documentation

---

## Next Steps

1. **Verify setup:**
   ```bash
   docker-compose up -d  # If using docker-compose
   ```

2. **Run tests:**
   ```bash
   go test -tags=e2e -v ./test/e2e/...
   ```

3. **Fix any failures:**
   - Check status codes
   - Verify response structure
   - Adjust timeouts

4. **Read full docs:**
   - `README.md` — Comprehensive user guide
   - `FLOWS.md` — Detailed walkthrough of each flow
   - `IMPLEMENTATION_SUMMARY.md` — Technical details

---

## Questions?

See `README.md` for full troubleshooting guide, or check `FLOWS.md` for detailed explanations of each test flow.

All test code is in `happy_path_test.go` with comprehensive comments explaining each assertion.
