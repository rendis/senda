# E2E Test Report — HT-37 (Refresco Operativo)

**Date:** 2026-02-25  
**Environment:** `docker/docker-compose.e2e.yml` (PostgreSQL 16 + Keycloak + Mailpit + Senda + River)  
**Commands executed:**

- `make test-e2e` (deterministic release gate)
- `make test-e2e-chaos` (observational, non-blocking release)

## Summary

| Suite | Scope | Result |
|---|---|---|
| Deterministic Gate | `TestCore|TestCRUD|TestE|TestF|TestS` | PASS |
| Chaos / Resilience | `TestC01..TestC09` | PASS |

## Deterministic Gate Status

- Deterministic suite passed end-to-end with no functional skips.
- `POST /api/v1/send` exercised in happy/error/security paths including:
  - workspace scope checks
  - `_system` block
  - `to <= 50`
  - template disabled -> `409`
  - rate limit behavior (`429`) in dedicated scenarios
- Data-plane queries and RBAC checks executed under deterministic suite.

## Chaos Suite Status (Non-Blocking Policy)

- `C01` provider restart recovery passed with deterministic recovery assertion.
- `C02` DB pause/unpause recovery passed.
- `C03` worker crash recovery passed.
- `C04` concurrent send race passed.
- `C05` concurrent publish race passed.
- `C06` under load observed `429 RATE_LIMITED` and accepted traffic coexistence.
- `C07` slow webhook endpoint did not block send API acceptance path.
- `C08` large payload returned controlled non-crash behavior.
- `C09` cache invalidation race passed.

## Notes

- Release gate remains deterministic-only (`make test-e2e`).
- Chaos remains observational (`make test-e2e-chaos`) and does not block release by policy.
- Historical `PRODUCTION BUG` branches were removed from deterministic flow handling.

## Final QA Gate Verdict

- Backend P0 deterministic E2E gate: **GREEN**.
- Chaos suite execution: **GREEN** (observational).
- No open critical functional blockers in E2E core gate.
