# Security Findings — HT-37

**Date:** 2026-02-17
**Methodology:** OWASP Top 10 automated testing via E2E security suite (S01-S12)
**Severity Scale:** Critical / High / Medium / Low / Info

## Summary

| Severity | Count |
|----------|-------|
| Critical | 0 |
| High | 0 |
| Medium | 1 |
| Low | 1 |
| Info | 2 |
| **Total** | **4** |

**Verdict: No critical or high vulnerabilities. Safe to proceed to frontend.**

---

## Findings

### SEC-01: Cursor Parameter Causes 500 on Malformed Input
- **Severity:** Medium
- **OWASP:** A03:2021 Injection
- **Test:** S01_SQLInjection/emails_query_cursor_injection
- **Description:** The cursor parameter in `GET /emails?cursor=...` is passed to the SQL query without UUIDv7 format validation. SQL injection payloads cause a 500 Internal Server Error instead of 400 Bad Request. No data exfiltration is possible because queries use parameterized statements (`$1`), but the error response is incorrect.
- **Payloads tested:** `' OR '1'='1`, `'; DROP TABLE emails; --`, `' UNION SELECT 1,2,3,4,5 --`, and 6 others.
- **Impact:** Information disclosure via error messages (potential stack traces in non-production). Denial of service if attacker floods with invalid cursors.
- **Remediation:** Validate cursor format as UUIDv7 before passing to query. Return 400 for invalid format.
- **Status:** Open (production bug logged)

### SEC-02: DKIM Not Configured in Test Environment
- **Severity:** Low
- **OWASP:** N/A (infrastructure)
- **Test:** S10_CryptographicValidation/dkim_signature_valid_ed25519
- **Description:** DKIM-Signature header not present on emails sent through Mailpit. Expected in test environment since DKIM requires DNS records. Production deployment must configure DKIM signing.
- **Impact:** None in test. In production, missing DKIM reduces email deliverability and opens phishing risk.
- **Remediation:** Ensure DKIM signing is configured in production deployment checklist.
- **Status:** Expected (test limitation)

### SEC-03: Rate Limiting Not Triggered Under Test Load
- **Severity:** Info
- **OWASP:** N/A (configuration)
- **Test:** E04_RateLimitExceeded
- **Description:** 200 rapid requests did not trigger 429 rate limiting. The test adapter is configured with `RateLimitPerSecond: 100`, and Go's HTTP client + local loopback make 200 requests well within the burst capacity.
- **Impact:** Rate limiting mechanism exists (token bucket in PostgreSQL) but test doesn't reach threshold. Separate load test recommended.
- **Remediation:** Create dedicated load test with lower rate limit or higher request volume.
- **Status:** Accepted (rate limiter verified via C06_RateLimiterUnderLoad)

### SEC-04: Adapter Credentials Encrypted at Rest
- **Severity:** Info (positive finding)
- **OWASP:** A02:2021 Cryptographic Failures
- **Test:** S10_CryptographicValidation/adapter_credentials_not_in_plaintext
- **Description:** Adapter configuration (AWS access keys, secrets) stored encrypted in database via AES-GCM. Direct DB query confirms ciphertext, not plaintext.
- **Impact:** Positive — credentials protected even if database is compromised.
- **Status:** Verified

---

## OWASP Top 10 Coverage

| OWASP Category | Tests | Status |
|----------------|-------|--------|
| A01: Broken Access Control | S02, S03, S07, S08 | PASS |
| A02: Cryptographic Failures | S09, S10 | PASS |
| A03: Injection | S01, S04, S11 | PASS (Medium finding on cursor) |
| A04: Insecure Design | S06 (mass assignment) | PASS |
| A05: Security Misconfiguration | S12 (path traversal) | PASS |
| A06: Vulnerable Components | N/A (manual audit) | — |
| A07: Auth Failures | S02 | PASS |
| A08: Data Integrity | S05 (SSRF) | PASS |
| A09: Logging Failures | (covered by audit log tests) | PASS |
| A10: SSRF | S05 | PASS |

## Tested Attack Vectors

- SQL injection (9 payloads across 6 injection points)
- XSS via template body, subject, from_name, injector values
- SSRF via webhook URLs (localhost, 169.254.x.x, internal IPs)
- DNS rebinding via webhook endpoints
- IDOR with random UUIDs across 5 resource types
- JWT none algorithm attack
- API key timing attack (constant-time validation verified)
- Mass assignment on 4 endpoints
- CRLF header injection in email fields
- Path traversal in URL parameters and resource slugs
- Cross-tenant/cross-workspace access isolation
- Rate limit bypass via User-Agent rotation
