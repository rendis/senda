# Security Findings — HT-37 (Refresco Operativo)

**Date:** 2026-02-25  
**Methodology:** OWASP-focused E2E suite (`TestS01..TestS12`) + deterministic gate evidence  
**Status:** No open critical/high findings.

## Severity Summary

| Severity | Open | Closed |
|---|---:|---:|
| Critical | 0 | 0 |
| High | 0 | 0 |
| Medium | 0 | 1 |
| Low | 0 | 1 |
| Info | 2 | 0 |

## Findings Status

### SEC-01: Cursor malformed payload handling (Injection path)
- **Previous:** malformed cursor payloads could bubble as `500`.
- **Current:** deterministic security suite validates non-500 handling for injection payloads.
- **Status:** **Closed**.
- **Validation:** `TestS01_SQLInjection/emails_query_cursor_injection` in deterministic gate.

### SEC-02: DKIM not present in local Mailpit environment
- **Severity:** Low (environmental limitation)
- **Description:** local Mailpit environment does not represent provider DKIM signing.
- **Status:** Accepted limitation for local test env.
- **Action in production:** enforce provider-managed SPF/DKIM/DMARC checklist in deploy runbook.

### SEC-03: Rate-limit behavior under adversarial load
- **Severity:** Info
- **Description:** rate limiting verified in deterministic (`E04`) and chaos (`C06`) scenarios.
- **Status:** Verified.

### SEC-04: Credentials at rest
- **Severity:** Info
- **Description:** encrypted adapter credentials behavior remains verified by automated tests.
- **Status:** Verified.

## OWASP Coverage (Automated)

| OWASP Category | Coverage | Status |
|---|---|---|
| A01 Broken Access Control | S02, S03, S07 | PASS |
| A02 Cryptographic Failures | S10 | PASS |
| A03 Injection | S01, S04, S11 | PASS |
| A04 Insecure Design | S06 | PASS |
| A05 Security Misconfiguration | S12 | PASS |
| A10 SSRF | S05 | PASS |

## Verdict

Security gate is **operationally green** for backend P0 under deterministic release criteria.
