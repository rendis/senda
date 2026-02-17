# Security Test Suite (S01-S12)

## Overview

This document describes the comprehensive security test suite for the Senda email orchestration platform. All tests are contained in `security_test.go` and follow OWASP Top 10 (2021) vulnerabilities.

**Build tag:** `//go:build e2e` (requires E2E test environment with running Senda server, PostgreSQL 16, and Mailpit)

**Philosophy:** Adversarial testing — tests are written like a real attacker trying to exploit the API, not as confirmation of expected behavior.

---

## Test Summary

| Test ID | Name | OWASP | Severity | Sub-Tests | Payloads |
|---------|------|-------|----------|-----------|----------|
| S01 | SQL Injection | A03:2021 – Injection | CRITICAL | 6 | 10 SQL payloads × 6 endpoints = 60 cases |
| S02 | Broken Authentication | A07:2021 – Auth Failures | CRITICAL | 9 | 9 auth bypass scenarios |
| S03 | Broken Access Control | A01:2021 – Access Control | CRITICAL | 7 | 7 RBAC/IDOR scenarios |
| S04 | XSS via Templates | A03:2021 – Injection (XSS) | HIGH | 5 | 13 XSS payloads × 4 fields = 52 cases |
| S05 | SSRF via Webhooks | A10:2021 – SSRF | HIGH | 2 | 19 SSRF payloads |
| S06 | Mass Assignment | A01:2021 – Access Control | MEDIUM | 4 | 4 mass assignment scenarios |
| S07 | IDOR | A01:2021 – Access Control | CRITICAL | 6 | 5 UUIDs × 6 resources = 30 enumeration cases |
| S08 | Rate Limit Bypass | A05:2021 – Resource Exhaustion | MEDIUM | 3 | 3 bypass techniques |
| S09 | API Key Timing Attack | A02:2021 – Crypto Failures | MEDIUM | 1 | 3 timing measurements |
| S10 | Cryptographic Validation | A02:2021 – Crypto Failures | CRITICAL | 4 | 4 crypto checks |
| S11 | SMTP Header Injection | A03:2021 – Injection | HIGH | 2 | 6 header injection payloads |
| S12 | Path Traversal | A01:2021 – Access Control | MEDIUM | 5 | 13 path traversal payloads × 5 fields = 65 cases |
| **TOTAL** | | | | **55 sub-tests** | **~400+ attack payloads** |

---

## Test Details

### TestS01_SQLInjection
**Target:** SQL injection protection
**Severity:** CRITICAL — can leak entire database, create/drop tables, escalate privileges

**What it tests:**
- Query parameter filtering: `?external_id=`, `?recipient=`, `?cursor=`
- JSON field validation: tenant `code`, injector `name`, send `variables`
- All endpoints reject SQL injection payloads with either:
  - `400 Bad Request` (validation rejects the input)
  - `200 OK` (normal response with empty/safe result)
  - **Never** `500 Internal Server Error` (would indicate SQL reached the database)

**SQL Payloads tested:** (10 variations)
```
' OR '1'='1
'; DROP TABLE emails; --
1; SELECT * FROM pg_catalog.pg_tables --
' UNION SELECT 1,2,3,4,5 --
\\'; DELETE FROM templates WHERE ''='
admin' --
' OR 1=1 --
'; EXEC xp_cmdshell('dir'); --
' AND '1'='1
' AND 1=2 UNION SELECT NULL,NULL,NULL --
```

**Endpoints tested:**
1. `GET /api/v1/emails?external_id=<payload>`
2. `GET /api/v1/emails?recipient=<payload>`
3. `GET /api/v1/emails?cursor=<payload>`
4. `POST /api/v1/manage/tenants` with `code: <payload>`
5. `POST /tenants/:code/workspaces/:ws/injectors` with `name: <payload>`
6. `POST /api/v1/send` with variables containing `<payload>`

**Protection mechanism expected:** Parameterized queries (pgx v5), input validation

---

### TestS02_BrokenAuthentication
**Target:** Authentication middleware
**Severity:** CRITICAL — can bypass auth entirely, impersonate any user

**What it tests:**
1. **Missing Authorization header** → `401 Unauthorized`
2. **Invalid JWT token format** → `401`
3. **Malformed JWT** (not even close to valid structure) → `401`
4. **JWT signed with wrong key** → `401`
5. **Empty bearer token** → `401`
6. **Invalid API Key format** (doesn't match snd_live_/snd_test_) → `401`
7. **API Key with extra characters** (e.g., special chars) → `401`
8. **API Key on management endpoints** (should reject, OIDC only) → `401`/`403`
9. **JWT with "none" algorithm** (security anti-pattern) → `401`

**Protection mechanisms expected:**
- Mandatory Authorization header or X-API-Key
- JWT signature validation
- API Key format validation (snd_live_* or snd_test_*)
- API keys rejected from `/api/v1/manage/*` endpoints

---

### TestS03_BrokenAccessControl
**Target:** Role-Based Access Control (RBAC) and object ownership
**Severity:** CRITICAL — can escalate privileges, access other users' data

**What it tests:**
1. **workspace-viewer calling admin-only endpoint** (POST /injectors) → `403`/`401`
2. **workspace-editor calling admin-only endpoint** (DELETE /templates) → `403`/`401`
3. **tenant-admin calling superadmin-only endpoint** (POST /tenants) → `403`/`401`
4. **Cross-tenant access** (accessing other tenant) → `404` (not `403`, to prevent enumeration)
5. **Cross-workspace access** (accessing other workspace) → `404`
6. **API Key workspace isolation** (key scoped to ws-A, accessing ws-B) → `401`/`403`
7. **Path traversal in URL** (e.g., `/tenants/../../../admin`) → `404`/`400`
8. **Direct UUID access to non-existent resource** → `404` (not `403`)

**Protection mechanisms expected:**
- Middleware enforcing `RequireRole(role)` on all routes
- Object ownership checks before returning resources
- 404 for non-existent resources (prevents enumeration)
- API Key workspace scoping at repository layer

---

### TestS04_XSSViaTemplates
**Target:** XSS injection through template content storage/output
**Severity:** HIGH — can steal cookies, sessions, credentials

**What it tests:**
- Template body, subject, from_name, injector values all tested with XSS payloads
- Content should be stored safely and sanitized on output
- Responses should not contain executable JavaScript

**XSS Payloads tested:** (13 variations)
```
<script>alert('xss')</script>
"><img src=x onerror=alert(1)>
<svg onload=alert('xss')>
<iframe src="javascript:alert('xss')"></iframe>
javascript:alert('xss')
<body onload=alert('xss')>
<input onfocus=alert('xss') autofocus>
<select onfocus=alert('xss') autofocus>
<textarea onfocus=alert('xss') autofocus>
<marquee onstart=alert('xss')>
<details open ontoggle=alert('xss')>
<video src=x onerror=alert('xss')>
<!--<img src=x onerror=alert('xss')>-->
```

**Fields tested:**
1. Template body (MJML)
2. Template subject
3. Template from_name
4. Injector field values

**Protection mechanisms expected:**
- HTML sanitization library (e.g., bluemonday)
- Content Security Policy headers
- Input validation rejecting dangerous patterns

---

### TestS05_SSRFViaWebhooks
**Target:** Server-Side Request Forgery (SSRF) via webhook URLs
**Severity:** HIGH — can access internal services, AWS/GCP metadata endpoints, private networks

**What it tests:**
Attempts to register webhooks with URLs pointing to:
1. **Loopback addresses** (block)
   - `http://127.0.0.1:8080`
   - `http://localhost:3000`
   - `http://[::1]:8080` (IPv6)
   - `http://0.0.0.0:8080`
   - `http://localhost.localdomain`

2. **Private RFC 1918 networks** (block)
   - `http://10.0.0.1` – `http://10.255.255.255`
   - `http://172.16.0.1` – `http://172.31.255.255`
   - `http://192.168.0.1` – `http://192.168.255.255`

3. **Cloud metadata endpoints** (block)
   - `http://169.254.169.254/latest/meta-data` (AWS)
   - `http://metadata.google.internal` (GCP)
   - `http://169.254.169.254/metadata/instance` (Azure)

4. **Valid external URLs** (allow)
   - `http://webhook.site/abc123def`
   - `https://api.service.com/webhook`

5. **DNS rebinding attempts** (should not crash)
   - Domain resolving to localhost should be rejected or handled gracefully

**Protection mechanisms expected:**
- DNS resolution followed by IP range validation
- Block private IP ranges and metadata endpoints before making request
- Reject SSRF attempts with `400 Bad Request`

---

### TestS06_MassAssignment
**Target:** Mass assignment (setting unintended fields via request body)
**Severity:** MEDIUM — can set administrative flags, bypass business logic

**What it tests:**
Sending extra fields in POST/PUT requests that should be ignored:
1. **POST /tenants** with `is_superadmin: true` → ignored
2. **POST /send** with `bypass_rate_limit: true` → ignored
3. **POST /api-keys** with extra `workspace_id: "other"` → scoped to current workspace
4. **POST /members** with `is_superadmin: true` → ignored

**Protection mechanisms expected:**
- Strict request parsing (only unmarshal expected fields)
- Extra fields silently ignored (not causing 400)
- No privilege escalation via mass assignment

---

### TestS07_IDOR
**Target:** Insecure Direct Object Reference (IDOR) — accessing resources by guessing UUIDs
**Severity:** CRITICAL — can enumerate and access other users' resources

**What it tests:**
- Attempt to access templates, webhooks, members, API keys, adapters, domains with random UUIDs
- Should return `404 Not Found` (not `403 Forbidden`) to prevent enumeration
- Tests 5 random UUIDs against 6 resource types = 30 enumeration attempts

**Random UUIDs tested:**
```
550e8400-e29b-41d4-a716-446655440000
550e8400-e29b-41d4-a716-446655440001
550e8400-e29b-41d4-a716-446655440002
ffffffff-ffff-ffff-ffff-ffffffffffff
00000000-0000-0000-0000-000000000000
```

**Protection mechanisms expected:**
- Return `404` for non-existent resources (not `403`)
- Verify object ownership before returning any data

---

### TestS08_RateLimitBypass
**Target:** Rate limiting evasion
**Severity:** MEDIUM — enables brute force attacks, DOS

**What it tests:**
1. **IP rotation via X-Forwarded-For** — changing source IP should not reset rate limit
2. **User-Agent rotation** — changing UA should not bypass rate limit
3. **Auth method switching** — switching JWT ↔ API Key should not reset rate limit (should be per-workspace)

**Protection mechanisms expected:**
- Rate limit keyed by workspace (not by IP/UA/auth method)
- Use real IP (ignore X-Forwarded-For unless behind trusted proxy)
- Token bucket in PostgreSQL (no Redis), persists across requests

---

### TestS09_APIKeyTimingAttack
**Target:** Constant-time API Key comparison
**Severity:** MEDIUM — enables brute force attacks

**What it tests:**
- Measure response time for completely wrong API key
- Measure response time for key with correct prefix
- Measure response time for key with 90% hash match
- All three should have response times within 50ms variance (constant-time comparison)

**What failure looks like:**
- Wrong key: 1ms
- Correct prefix: 10ms
- 90% match: 45ms
- → Attacker can brute force by measuring timing

**Protection mechanisms expected:**
- Use `subtle.ConstantTimeCompare()` (Go standard library)
- All key comparisons constant-time

---

### TestS10_CryptographicValidation
**Target:** Cryptographic implementations
**Severity:** CRITICAL — can forge emails, leak credentials, spoof identities

**What it tests:**
1. **API Key format** — all keys start with `snd_live_` or `snd_test_`
2. **Adapter credentials not in plaintext** — GET /adapters should NOT return password/username in response body
3. **Request ID unpredictable** — request IDs should not follow a sequential pattern
4. **DKIM signature valid** — email signed with Ed25519, signature present in Mailpit

**Protection mechanisms expected:**
- All API Keys stored as SHA256 hash (raw never persisted)
- Adapter credentials encrypted with AES-256-GCM
- Request ID generation using crypto/rand (UUIDv7)
- DKIM signing on all outgoing emails using Ed25519 keys

---

### TestS11_HeaderInjection
**Target:** SMTP header injection (email header manipulation)
**Severity:** HIGH — can manipulate BCC, CC, inject headers, spoof sender

**What it tests:**
- Injecting CRLF characters (`\r\n`) in email fields to add additional headers
- Attempting to inject BCC, CC, custom headers via:
  - `from_name: "Admin\r\nBcc: attacker@evil.com"`
  - `subject: "Test\r\nX-Injected: true"`
  - `to: "victim@test.com\r\nBcc: attacker@evil.com"`

**Payloads tested:** (6 variations)
```
Admin\r\nBcc: attacker@evil.com
Admin\nBcc: attacker@evil.com
Test\r\nX-Injected: true
Test\r\nCc: attacker@evil.com
Test\nCc: attacker@evil.com
Preview\r\nBcc: attacker@evil.com
```

**Protection mechanisms expected:**
- Strip/reject CRLF characters from all email fields
- Validation rejects `\r`, `\n` in headers
- Verify final email in Mailpit has no injected headers

---

### TestS12_PathTraversal
**Target:** Path traversal in codes/slugs
**Severity:** MEDIUM — can bypass authorization, access unintended resources

**What it tests:**
- All code and slug fields should reject path traversal attempts
- Fields: tenant code, workspace code, template slug, injector name
- All should return `400 Bad Request`

**Payloads tested:** (13 variations)
```
../admin
..\\admin
../../etc/passwd
..%2fadmin
..%252fadmin
....//admin
..%c0%afadmin
admin/../../../etc/passwd
%2e%2e%2fadmin
admin/..%2fpasswd
test/../../admin
../../system
admin%3a%3apasswd
admin%0a%0dpasswd
```

**Protection mechanisms expected:**
- Slug/code validation regex: `^[a-z0-9_-]+$` (no dots, slashes, special chars)
- Reject anything with `.`, `/`, `\`, or encoded variants
- Return `400` on invalid format

---

## Running the Tests

### Prerequisites
```bash
# Install dependencies
go get -u github.com/stretchr/testify

# Ensure E2E test environment is running
docker-compose -f docker-compose.test.yml up -d
```

### Run All Security Tests
```bash
go test -v ./test/e2e -run "TestS" -timeout 300s
```

### Run Specific Test
```bash
# Run only SQL injection tests
go test -v ./test/e2e -run "TestS01" -timeout 60s

# Run only authentication tests
go test -v ./test/e2e -run "TestS02" -timeout 60s
```

### Run with Coverage
```bash
go test -v ./test/e2e -run "TestS" -coverprofile=coverage.out -timeout 300s
go tool cover -html=coverage.out
```

---

## Test Methodology

### Adversarial Approach
Each test is written like a real attacker trying to exploit the API:
- **Try to inject SQL** — validate that parameterized queries work
- **Try to bypass auth** — validate that auth is mandatory
- **Try to escalate privileges** — validate RBAC is enforced
- **Try to enumerate UUIDs** — validate 404 is returned, not 403

### Assertions
Tests use `require` (not `assert`) — any failure stops the test immediately, providing clear diagnostics:
```go
require.NotEqual(t, 500, resp.StatusCode, "SQL injection payload reached DB: %s", payload)
require.Equal(t, 401, resp.StatusCode, "missing auth should return 401, got %d", resp.StatusCode)
require.Equal(t, 404, resp.StatusCode, "enumeration attack should return 404, got %d", resp.StatusCode)
```

### Error Messages
Each assertion includes context about what was tested and why it matters:
```go
// ✓ Good
require.Equal(t, 400, resp.StatusCode,
    "path traversal in tenant code should return 400, got %d for payload: %s",
    resp.StatusCode, payload)

// ✗ Bad
require.Equal(t, 400, resp.StatusCode)
```

---

## Expected Test Results

### All Tests Pass ✅
- No SQL injection possible
- No auth bypasses
- RBAC strictly enforced
- XSS payloads sanitized
- SSRF blocked
- Rate limits persist
- Timing attack impossible
- Cryptography correct
- Header injection blocked
- Path traversal blocked

### Investigation Required 🔍
Any test failure indicates a security vulnerability. Examples:
- **TestS01 failure** → SQL injection leak (CRITICAL)
- **TestS02 failure** → Auth bypass possible (CRITICAL)
- **TestS07 failure** → Can enumerate UUIDs / access other users' data (CRITICAL)
- **TestS10 failure** → Cryptographic issues (CRITICAL)
- **TestS05 failure** → SSRF possible, can access internal services (HIGH)

---

## Maintenance

### Adding New Tests
1. Create `func TestSXX_Description(t *testing.T)`
2. Use `t.Run()` for sub-tests
3. Document OWASP category and severity
4. Include 3-5 concrete payload examples
5. Use `require.*` for assertions with context

### Updating Payloads
- SQL injection payloads: update `sqlPayloads` slice
- XSS payloads: update `xssPayloads` slice
- SSRF payloads: update `ssrfPayloads` struct
- Path traversal: update `pathTraversalPayloads` slice

### False Positives
If a test incorrectly flags a legitimate feature:
1. Review the assertion logic
2. Check if the feature should really be rejected
3. Update the test expectation or the code being tested
4. **Never** disable a security test without explicit approval

---

## References

- OWASP Top 10 2021: https://owasp.org/Top10/
- Testing Guide: https://owasp.org/www-project-web-security-testing-guide/
- Go Security Best Practices: https://golang.org/doc/security
- Senda Threat Model: `docs/specs/SECURITY_CHECKLIST.md`

---

## Contact

For questions about these tests, refer to:
- Team QA (HT-37) — owns security test suite
- Senda Security Review — annually or on request
