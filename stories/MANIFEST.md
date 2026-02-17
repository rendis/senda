# Senda — Story Manifest

> Auto-actualizar este archivo al mover stories entre directorios.
> Última actualización: 2026-02-17

---

## Status Overview

| Status | Count | Stories |
|--------|-------|---------|
| Backlog | 20 | HT-12, HT-15, HT-16, HT-20 to HT-25, HT-27, HT-28 to HT-37 |
| In Progress | 0 | — |
| Done | 17 | HT-01 to HT-11, HT-13, HT-14, HT-17 to HT-19, HT-26 |
| Blocked | 0 | — |

**Progress: 17 / 37 (46%)**

---

## Dependency Matrix

| HT | Title | Dependencies | Track | Status |
|----|-------|-------------|-------|--------|
| HT-01 | Project Scaffolding + Docker Compose | — | A | done |
| HT-02 | Configuration + Secrets Management | HT-01 | A | done |
| HT-03 | Database Connection + Migrations Runner | HT-01, HT-02 | A | done |
| HT-04 | Encryption Module (AES-256-GCM) | HT-02 | A | done |
| HT-05 | Domain Models + Error Types | HT-01 | B | done |
| HT-06 | Port Interfaces (Contratos) | HT-05 | B | done |
| HT-07 | PG Stores — Tenants, Workspaces, Config | HT-03, HT-05, HT-06 | B | done |
| HT-08 | PG Stores — Injectors, Adapters, Domains, Templates | HT-04, HT-07 | B | done |
| HT-09 | PG Stores — Members, API Keys, Emails, Audit, etc. | HT-07 | B | done |
| HT-10 | ChainResolver + InjectorMerger | HT-06, HT-08 | B | done |
| HT-11 | TemplateResolver + AdapterResolver | HT-10 | B | done |
| HT-12 | DomainResolver + Cache Invalidation | HT-10, HT-11 | B | backlog |
| HT-13 | PG Cache + Token Bucket Rate Limiter | HT-03 | A | done |
| HT-14 | MJML Compiler + DKIM Signer | HT-01 | A | done |
| HT-15 | SendService — Orchestration Core | HT-10, HT-11, HT-12, HT-13, HT-14 | D | backlog |
| HT-16 | River Workers (Send, Verify, Webhook) | HT-13, HT-14, HT-15 | D | backlog |
| HT-17 | Echo v5 Server + Base Middleware | HT-02 | C | done |
| HT-18 | Auth Middleware (OIDC + API Keys) | HT-09, HT-17 | C | done |
| HT-19 | CRUD Handlers — Tenants, Workspaces, Members | HT-07, HT-17, HT-18 | C | done |
| HT-20 | CRUD Handlers — Injectors, Adapters, Domains | HT-08, HT-14, HT-17, HT-18 | C | backlog |
| HT-21 | CRUD Handlers — Templates, Versions, Locales | HT-08, HT-14, HT-17, HT-18 | C | backlog |
| HT-22 | Send Endpoint + Email Query + Tracking | HT-15, HT-16, HT-17, HT-18 | D | backlog |
| HT-23 | Provider Event Ingestion (SES Webhooks) | HT-09, HT-16, HT-17 | D | backlog |
| HT-24 | Webhook System (Dispatch + CRUD) | HT-16, HT-17, HT-18 | D | backlog |
| HT-25 | Onboarding Flow | HT-07, HT-09, HT-17, HT-18 | C | backlog |
| HT-26 | Observability (Metrics + Health + Logging) | HT-17 | D | done |
| HT-27 | API Keys Service + Management Endpoints | HT-09, HT-17, HT-18 | C | backlog |
| **HT-37** | **E2E Backend QA + Pentesting + Colección Postman** | **HT-11..12, HT-15..16, HT-19..25, HT-27** | **F** | **backlog** |
| HT-28 | Frontend Scaffolding + Pencil MCP + Design System | HT-17, **HT-37** | E | backlog |
| HT-29 | Auth (OIDC) + Scope Switcher + Protected Routes | HT-18, HT-25, HT-28 | E | backlog |
| HT-30 | Onboarding Wizard (3 pasos) | HT-25, HT-29 | E | backlog |
| HT-31 | Dashboard + Métricas | HT-29 | E | backlog |
| HT-32 | Emails — Lista + Detalle | HT-22, HT-29 | E | backlog |
| HT-33 | Templates — Types + Lista + Editor MJML | HT-21, HT-29 | E | backlog |
| HT-34 | CRUD Views — Injectors + Adapters + Domains | HT-20, HT-29 | E | backlog |
| HT-35 | CRUD Views — Webhooks + API Keys + Members | HT-19, HT-24, HT-27, HT-29 | E | backlog |
| HT-36 | Audit Log + Settings + Empty States | HT-29 | E | backlog |

---

## Dependency Graph (Visual)

```
Level 0 (no deps):
  HT-01 ─────────────────────────────────────────────

Level 1 (depends on HT-01):
  HT-02   HT-05   HT-14

Level 2:
  HT-03 (01,02)   HT-04 (02)   HT-06 (05)   HT-17 (02)

Level 3:
  HT-07 (03,05,06)   HT-13 (03)   HT-26 (17)

Level 4:
  HT-08 (04,07)   HT-09 (07)

Level 5:
  HT-10 (06,08)   HT-18 (09,17)

Level 6:
  HT-11 (10)   HT-19 (07,17,18)   HT-20 (08,14,17,18)
  HT-21 (08,14,17,18)   HT-25 (07,09,17,18)   HT-27 (09,17,18)

Level 7:
  HT-12 (10,11)

Level 8:
  HT-15 (10,11,12,13,14)

Level 9:
  HT-16 (13,14,15)

Level 10:
  HT-22 (15,16,17,18)   HT-23 (09,16,17)   HT-24 (16,17,18)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BACKEND COMPLETE ↑  |  QA GATE ↓  |  FRONTEND ↓↓
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Level 11 — QA GATE (bloqueante para frontend):
  HT-37 (ALL backend: HT-11,12,15,16,19-25,27)

Level 12:
  HT-28 (17,37)

Level 13:
  HT-29 (18,25,28)

Level 14:
  HT-30 (25,29)   HT-31 (29)   HT-36 (29)

Level 15:
  HT-33 (21,29)   HT-34 (20,29)

Level 16:
  HT-32 (22,29)   HT-35 (19,24,27,29)
```

---

## Tracks (Parallel Execution)

### Track A — Infrastructure ✅ DONE
```
HT-01 → HT-02 → HT-03 → HT-04 → HT-13 → HT-14
```

### Track B — Domain + Resolution
```
HT-05 → HT-06 → HT-07 → HT-08 → HT-09 → HT-10 → HT-11 → HT-12
```

### Track C — API Layer
```
HT-17 → HT-18 → HT-19 → HT-20 → HT-21 → HT-27 → HT-25
```

### Track D — Send Flow + Operations
```
HT-15 → HT-16 → HT-22 → HT-23 → HT-24 → HT-26
```

### Track F — QA Gate (after all backend)
```
HT-37 (depends on ALL of Track B + C + D completing)
```

### Track E — Frontend (after QA Gate)
```
HT-28 → HT-29 → HT-30 → HT-31 → HT-32 → HT-33 → HT-34 → HT-35 → HT-36
```

---

## Epic Summary

| Epic | Stories | Done | % |
|------|---------|------|---|
| E1 — Foundation | HT-01, HT-02, HT-03, HT-04 | 4 | 100% |
| E2 — Core Domain | HT-05, HT-06, HT-07, HT-08, HT-09 | 5 | 100% |
| E3 — Resolution Engine | HT-10, HT-11, HT-12 | 2 | 67% |
| E4 — Send Flow | HT-13, HT-14, HT-15, HT-16 | 2 | 50% |
| E5 — API Layer | HT-17, HT-18, HT-19, HT-20, HT-21, HT-22 | 3 | 50% |
| E6 — Operations | HT-23, HT-24, HT-25, HT-26, HT-27 | 1 | 20% |
| **E8 — QA Gate** | **HT-37** | **0** | **0%** |
| E7 — Frontend | HT-28, HT-29, HT-30, HT-31, HT-32, HT-33, HT-34, HT-35, HT-36 | 0 | 0% |

---

## Pipeline Flow

```
Backend (Tracks A+B+C+D)  →  QA Gate (Track F)  →  Frontend (Track E)
     HT-01 to HT-27            HT-37                HT-28 to HT-36
```

**El frontend NO comienza hasta que HT-37 esté en `done/`.**

---

## Ready to Start (dependencies met)

> Stories whose ALL dependencies are in `done/`:

- **HT-12** — DomainResolver + Cache Invalidation *(deps: HT-10, HT-11 done)*
- **HT-20** — CRUD Handlers: Injectors, Adapters, Domains *(deps: HT-08, HT-14, HT-17, HT-18 done)*
- **HT-21** — CRUD Handlers: Templates, Versions, Locales *(deps: HT-08, HT-14, HT-17, HT-18 done)*
- **HT-25** — Onboarding Flow *(deps: HT-07, HT-09, HT-17, HT-18 done)*
- **HT-27** — API Keys Service + Management *(deps: HT-09, HT-17, HT-18 done)*
