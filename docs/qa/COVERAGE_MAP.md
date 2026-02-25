# Coverage Map — Backend P0 E2E

**Date:** 2026-02-25  
**Source of truth commands:**

- `make test-e2e` (deterministic release gate)
- `make test-e2e-chaos` (observational)

## Suite-to-Scope Matrix

| Suite | Main Objective | Examples |
|---|---|---|
| `TestCore*` | Hard contractual guardrails in send/data-plane | workspace scope, `_system` block, recipient cap, template disable, data isolation |
| `TestCRUD*` | CRUD and management API behavior across scopes | tenant/workspace/template/adapter/injector/webhook/member/config/audit flows |
| `TestE*` | Deterministic error semantics | 401/403/404/409/422/429 mappings and config error paths |
| `TestF*` | End-to-end happy flows | onboarding, setup, template lifecycle, send, query, API key lifecycle, RBAC smoke |
| `TestS*` | Security and abuse resistance | SQLi, auth, access control, XSS, SSRF, IDOR, rate-limit bypass, header/path attacks |
| `TestC*` | Resilience/chaos (non-blocking) | provider down, DB pause, worker crash, concurrency races, payload stress |

## Deterministic Release Gate Coverage

`make test-e2e` covers these critical backend P0 contracts:

- `/api/v1/send` acceptance/rejection semantics (202/4xx/429)
- workspace API key scoping for data-plane and send
- template kill switch behavior (`409`)
- RBAC for editor/viewer/admin paths
- suppression and lifecycle visibility paths
- cursor/filter query safety in email listing APIs

## Operational Policy

- **Release-blocking:** `TestCore|TestCRUD|TestE|TestF|TestS`
- **Non-blocking but mandatory to monitor:** `TestC*`

## Current Status

- Deterministic gate: **PASS**
- Chaos suite: **PASS**
