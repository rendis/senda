# Senda — Code Agents Entry Point

## Project Overview

Senda is an open-source email orchestration platform built with Go + PostgreSQL (no Redis). It implements a 3-level hierarchy (Global → Tenant → Workspace) with inheritance chain resolution for templates, injectors, and adapters.

- **Module:** `github.com/rendis/senda`
- **Backend:** Go 1.25+ / PostgreSQL 16 + pg_cron / Echo v5 / River (PG-native queue) / pgx v5 / gomjml / golang-migrate
- **Frontend:** Next.js 16 / React 19 / TypeScript 5 / Tailwind v4 / shadcn/ui (in `web/`)
- **Architecture:** Hexagonal (Ports & Adapters) — domain has zero infra dependencies
- **No Redis.** PG handles cache (UNLOGGED tables), rate limiting (PL/pgSQL token bucket), and job queue (River).
- **Config priority:** env vars > YAML > defaults. Prefix: `SENDA_`
- **Auth:** OIDC (humans, management plane) + API Keys (machines, data plane). 5-tier RBAC.
- **Data patterns:** UUIDs v7, cursor-based pagination, soft delete (`deleted_at`), partitioned tables (emails, audit_logs)

### Key Paths

```
sdk/                    PUBLIC SDK — Engine, Injector interface, InjectorContext, InitFunc
                        Users `go get` this to extend Senda as a library.
cmd/senda/              Entry point (uses sdk.NewWithConfig + engine.Run)
internal/
  domain/               Entities, value objects, domain errors
  port/                 Interface contracts (stores, senders, crypto, cache, queue, CodeInjector)
  service/              Business logic (SendService, TemplateService, EventProcessor, WebhookService, APIKeyService, IdentityService)
  resolution/           Hierarchy chain resolution (chain, template, adapter, injector merger + code injectors)
  adapter/
    postgres/           PG stores (pgx v5) + rate limiter
    pgcache/            PG UNLOGGED cache
    ses/                AWS SES sender + identity provider
    gmail/              Gmail sender + identity provider
    smtp/               SMTP sender (dev/Mailpit)
    river/              Background workers (send + webhook)
    mjml/               MJML -> HTML compiler (gomjml)
    crypto/             AES-256-GCM encryption (HKDF)
  http/
    handler/            HTTP handlers (26+)
    middleware/          Auth, RBAC, scope, logger, metrics, recovery, requestid
  app/                  Bootstrap + DI wiring + Extensions bridge
pkg/                    apperr (error mapping), slug (validation), tracking (ID gen)
migrations/             24 SQL migrations (golang-migrate)
config/                 YAML config + env overrides (SENDA_* prefix)
web/src/                Frontend app (Next.js App Router)
```

### Rules

- **TDD mandatory** — write test first, make it pass, refactor
- **Manual mocks** — no mock frameworks; implement interfaces by hand
- **Integration tests** — use TestContainers, tag with `//go:build integration`
- **E2E tests** — tag with `//go:build e2e`, require Docker stack running

---

## Session Workflow

### At the start of each session

1. **Read this file** (`AGENTS.md`) — you are already doing that
2. **Review the current state:**

   ```bash
   ls stories/in-progress/    # What's currently in progress?
   ls stories/done/           # What's already completed?
   ```

3. **If there is an HT in progress** → continue from where it stopped (read that HT's "Progress Log" section)
4. **If nothing is in progress** → check the `stories/MANIFEST.md` "Ready to Start" section and pick the next story according to the recommended order

### To implement an HT:

1. Read the full HT (`stories/backlog/HT-XX.md`)
2. Read the referenced TECH_SPEC sections listed in `spec_sections`
3. Move the HT to `stories/in-progress/` and update its front matter
4. Implement with TDD (test → code → refactor)
5. Record decisions and progress in the HT ("Implementation Notes" and "Progress Log")
6. When all acceptance criteria are met:
   - Run the quality gates (`make test`, `make lint`, `make vet`)
   - Move the HT to `stories/done/`
   - Update `stories/MANIFEST.md` (status, counters, "Ready to Start")

### If a session stops in the middle of an HT

- Leave the HT in `stories/in-progress/` with its "Progress Log" updated
- The next session resumes from there — do not repeat work that is already done

### Example prompt to start a session

```
Review the project state and continue with the next available HT.
```

Or, if you want to be specific:

```
Implement HT-01. Read the story and sections §9 and §18 of the TECH_SPEC.
```

---

## Working Teams (parallelization)

The project is organized around **specialized teams** working in parallel. Each team has a profile and optimized context for its track.

### Team Definitions

**Team Infra** — Track A (Foundation + Infrastructure)

- **Profile:** DevOps / Platform Engineer
- **Expertise:** Docker, PostgreSQL, migrations, crypto, caching, rate limiting
- **HTs:** HT-01 → HT-02 → HT-03 → HT-04 → HT-13 → HT-14
- **Spec focus:** §3, §4, §5, §7, §9, §17, §18, §23, §24

**Team Domain** — Track B (Core Domain + Resolution Engine)

- **Profile:** Domain Engineer / DDD Specialist
- **Expertise:** Domain modeling, hexagonal architecture, ports & adapters, resolution algorithms
- **HTs:** HT-05 → HT-06 → HT-07 → HT-08 → HT-09 → HT-10 → HT-11 → HT-12
- **Spec focus:** §10, §11, §12, §6

**Team API** — Track C (API Layer + Auth)

- **Profile:** Backend API Engineer
- **Expertise:** HTTP handlers, middleware, OIDC/JWT, RBAC, REST API design
- **HTs:** HT-17 → HT-18 → HT-19 → HT-20 → HT-21 → HT-27 → HT-25
- **Spec focus:** §8, §14, §15, §20

**Team SendOps** — Track D (Send Flow + Operations)

- **Profile:** Integration / Systems Engineer
- **Expertise:** Message queues, workers, webhooks, provider integrations, observability
- **HTs:** HT-15 → HT-16 → HT-22 → HT-23 → HT-24 → HT-26
- **Spec focus:** §13, §16, §19, §21

**Team Frontend** — Track E (Frontend + Design-to-Code)

- **Profile:** Frontend Engineer / Design Systems Specialist
- **Expertise:** Next.js, TypeScript, Tailwind CSS, React, component architecture, Pencil MCP
- **HTs:** HT-28 → HT-29 → HT-30 → HT-31 → HT-32 → HT-33 → HT-34 → HT-35 → HT-36
- **Spec focus:** DESIGN_BRIEF (§3 to §8), PRD (§5 User Stories US-36 to US-45)
- **Stack:** Next.js 16, TypeScript 5, Tailwind v4, shadcn/ui, TanStack Query 5, TanStack Table 9, React Hook Form 7, Zod 4, Auth.js v5, ky, Monaco Editor, Lucide React, Sileo
- **Design:** `senda_desing.pen` — ALWAYS read it through Pencil MCP; NEVER parse the `.pen` file as JSON
- **Flow:** Pencil MCP reads the frame → Agent generates the React + Tailwind component → verify pixel-perfect output → iterate in Pencil if there is drift
- **Blocked by:** HT-37 (QA Gate) — frontend work does NOT start until the backend is 100% tested

**Team QA** — Track F (Quality Assurance + Security)

- **Profile:** QA Engineer / Security Tester / Pentester
- **Expertise:** E2E testing, API testing, fuzzing, OWASP Top 10, race conditions, load testing, Postman, Go testing, TestContainers
- **HTs:** HT-37
- **Spec focus:** §6, §14, §15, §19, §20, §21, §24
- **Mindset:** Adversarial — try to break the system, not merely confirm that it works
- **Infrastructure:** TestContainers (PostgreSQL 16 + Mailpit + Senda Server + River workers)
- **Deliverables:** E2E test suite, OWASP pentesting, chaos tests, Postman collection, coverage reports, and findings
- **Gate:** Frontend (Track E) does NOT begin until HT-37 is in `done/`

### Parallelization Matrix

```
Week     Team Infra        Team Domain         Team API          Team SendOps
─────────────────────────────────────────────────────────────────────────────
S1       HT-01             HT-05               —                 —
S2       HT-02 + HT-04     HT-06               —                 —
S3       HT-03             HT-07               HT-17             —
S4       HT-13 + HT-14     HT-08               HT-18             —
S5       ✅ done            HT-09               HT-19             —
S6       —                 HT-10               HT-20 + HT-21     —
S7       —                 HT-11               HT-27 + HT-25     —
S8       —                 HT-12               ✅ done            HT-15
S9       —                 ✅ done              —                 HT-16
S10      —                 —                   —                 HT-22 + HT-23
S11      —                 —                   —                 HT-24 + HT-26

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BACKEND COMPLETE ↑   |   QA GATE ↓   |   FRONTEND ↓↓
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Week     Team QA
─────────────────────────
S12      HT-37 (E2E QA + Pentesting + Postman — waits for the full backend)

Week     Team Frontend (after the QA Gate)
─────────────────────────
S13      HT-28 (scaffolding + Pencil MCP + Design System)
S14      HT-29 (auth + scope switcher)
S15      HT-30 (onboarding wizard)
S15      HT-31 (dashboard + metrics)
S16      HT-33 (templates)
S16      HT-34 (injectors + adapters + domains)
S17      HT-36 (audit log + settings + empty states)
S17      HT-32 (emails)
S18      HT-35 (webhooks + API keys + members)
```

### Parallelization Rules

1. **HT-01 is globally blocking** — all teams wait for it to finish (project scaffolding)
2. **Team Infra and Team Domain start together** in S1 (HT-01 and HT-05 depend only on HT-01)
3. **Team API starts in S3** — it needs HT-02 (config) for HT-17 (Echo server)
4. **Team SendOps starts in S8** — it needs resolvers (HT-10..12) + infra (HT-13, HT-14)
5. **Team QA starts in S12** — it needs the FULL backend complete (Tracks A+B+C+D). HT-37 blocks frontend work
6. **Team Frontend starts in S13** — after the QA Gate (HT-37 in `done/`). Frontend work does NOT begin until the backend is 100% tested and closed
7. **Cross-team dependencies:** when an HT depends on another HT owned by another team, verify that it is in `stories/done/` before starting
8. **Each team keeps its own session** — `MANIFEST.md` is the shared synchronization point
9. **Pencil MCP is mandatory for Team Frontend** — ALWAYS read the design through the Pencil MCP server; NEVER parse the `.pen` file directly as JSON
10. **Pipeline:** `Backend (Tracks A+B+C+D) → QA Gate (Track F) → Frontend (Track E)`

### Example prompt to start a team:

```
You are Team Infra. Your profile is DevOps / Platform Engineer.
Read AGENTS.md, review `stories/done/` and `stories/in-progress/`,
and continue with the next HT in your track (A).
Only work on HTs assigned to your team.
```

```
You are Team QA. Your profile is QA Engineer / Pentester.
Read AGENTS.md and HT-37. Verify that ALL backend HTs are in `done/`.
Your job is to break the system, not to confirm that it works.
Bring up the full E2E stack and execute the test battery.
```

---

## Documentation Map

Documentation lives in `docs/`:

| Document | What You'll Find | Read When |
| --- | --- | --- |
| `docs/ARCHITECTURE.md` | Hexagonal layer diagrams, domain entities, port interfaces, resolution engine flow (chain → template → injector → adapter), send pipeline sequence, River workers, cache/rate-limit/encryption infrastructure, ADR summary | Understanding system design or adding new adapters/services |
| `docs/API.md` | All endpoints (health, data-plane, management, global), auth schemes (OIDC + API Key), RBAC roles, error codes, pagination (cursor-based), curl examples, webhook events + HMAC signatures | Building handlers, integrations, or debugging API calls |
| `docs/DEVELOPMENT.md` | First-time setup, Docker stacks (dev vs e2e comparison), all 27 Makefile targets, frontend npm scripts, DB migrations, Keycloak test users/credentials, service URLs/ports, testing guide (unit → system), troubleshooting | First session, env setup, or when something breaks |
| `docs/DEPLOYMENT.md` | Production Dockerfile, required/optional env vars, PG + pg_cron setup, OIDC provider config, email provider setup (SES/Gmail/SMTP), health endpoints, reverse proxy examples, docker run example | Deploying to production |
| `docs/specs/PRD_v5.md` | Product requirements, user stories, business rules, scope hierarchy rules | Understanding "why" and "what" |
| `docs/specs/TECH_SPEC_v1.md` | Complete technical spec (~5000 lines): SQL schema (16+ tables), migrations, folder structure, port interfaces, domain models, resolution engine, send flow, middleware, API contract, workers, config, Docker, observability, PG cache, rate limiting | **Primary reference for implementation** |
| `docs/specs/TECH_STORIES.md` | All 38 HTs with dependency graph, timeline, tracks | Understanding scope and order |
| `docs/specs/TESTING_STRATEGY.md` | Test pyramid, manual mock patterns, TestContainers usage, coverage targets | Writing tests |
| `docs/specs/SECURITY_CHECKLIST.md` | OWASP Top 10 mapping, AES-256-GCM encryption spec, auth requirements, API key security model | Security-sensitive code |
| `docs/specs/DESIGN_BRIEF.md` | UX/UI screens, component specs, responsive breakpoints, Design System tokens | Frontend implementation |
| `docs/specs/ADR-0001-...md` | **Why no DKIM/SPF/DMARC in app** — provider-managed email auth decision, consequences for send flow and identity validation | When questioning email auth approach |
| `docs/extensibility-guide.md` | SDK extension guide: Engine, Injectors, InitFunc, InjectorContext, lifecycle hooks, merge flow, consumer project structure, troubleshooting | Extending Senda as a Go library |
| `skills/senda/SKILL.md` | MCP tool reference, API groups, auth schemes, RBAC, SES lifecycle (provision + deprovision), AWS permission matrix | Working with Senda API via MCP, or understanding SES adapter lifecycle |
| `docs/postman/` | Postman collection (116KB, all endpoints) + local/staging environments | API testing and exploration |

### UI/UX Design — Pencil MCP (MANDATORY)

The base application design lives in `senda_desing.pen` (project root), created with [Pencil](https://www.pencil.dev/). Official documentation: https://docs.pencil.dev/

**RULE: ALWAYS use Pencil MCP to interact with the design. NEVER parse the `.pen` file as JSON.**

Pencil exposes a local MCP server while Pencil is open. The code agent connects automatically and has access to all available tools.

#### Available MCP tools (USE ALL RELEVANT ONES)

**Design — `batch_design`:**

- Create, modify, and manipulate design elements
- Operations: insert, copy, update, replace, move, delete
- Generate and place images
- **Mandatory** for any modification to the `.pen` file

**Read — `batch_get`:**

- Read design components and hierarchy
- Search for elements by pattern
- Inspect component structure
- **Mandatory** before implementing any screen (read the frame first)

**Screenshots — `get_screenshot`:**

- Render previews from Pencil
- **Mandatory** for DoD Gate 1: compare the app screenshot against the Pencil screenshot

**Layout — `snapshot_layout`:**

- Analyze layout structure
- Detect positioning issues
- Find overlapping elements
- **Use it** to verify pixel-perfect output after implementation

**Editor — `get_editor_state`:**

- Current editor context
- Selection information
- Details about the active file

**Variables — `get_variables` / `set_variables`:**

- Read design tokens (colors, spacing, typography)
- Update theme values
- Sync with CSS/Tailwind
- **Mandatory** to extract Design System tokens and map them to Tailwind config

#### Pencil MCP workflow

```
1. READ the design:
   batch_get → read the frame for the screen being implemented
   get_variables → extract tokens (colors, spacing, typography)
   get_screenshot → capture the design reference image

2. IMPLEMENT:
   Generate React + Tailwind components aligned with the design
   Use the tokens extracted in step 1 for exact colors and spacing

3. VALIDATE (DoD Gate 1):
   get_screenshot → capture the Pencil design
   Compare it against a screenshot of the running app (`pnpm --dir web dev`)
   snapshot_layout → verify structure and positioning
   If there is drift → fix it in CODE, not in Pencil

4. ITERATE if needed:
   batch_design → adjust the design only if the designer approves it
   set_variables → sync tokens if they changed
```

#### Advanced operations

**Batch operations** for consistency:

```
"Verify that all buttons use the primary color variable"
"Update all headings to use the typography scale"
"Apply an 8px grid to all elements"
```

**Code ↔ design synchronization:**

```
"Import the Design System from Tailwind config into Pencil"
"Update React components to match the Pencil designs"
"Sync typography variables between CSS and Pencil"
```

**Code generation from design:**

```
"Generate React code for this component"
"Generate Tailwind config from these Pencil variables"
```

#### Troubleshooting

- **MCP does not connect:** Verify that Pencil is running and that the `.pen` file is open
- **Unexpected changes:** Be more specific in prompts and ask for an explanation before applying changes

---

## How Stories Work

### Directory Structure

```
senda/
├── AGENTS.md              ← You are here (entry point)
├── stories/
│   ├── MANIFEST.md        ← Dependency graph + status overview
│   ├── backlog/           ← Stories not yet started
│   ├── in-progress/       ← Currently being worked on
│   ├── done/              ← Completed stories
│   └── blocked/           ← Stories blocked by external dependency
├── docs/
│   ├── ARCHITECTURE.md    ← Architecture deep-dive + diagrams
│   ├── API.md             ← Full API reference
│   ├── DEVELOPMENT.md     ← Developer setup guide
│   ├── DEPLOYMENT.md      ← Production deployment
│   ├── postman/           ← Postman collection + environments
│   └── specs/             ← PRD, Tech Spec, Stories, Security, Design
└── ...
```

### Story Lifecycle

1. **Pick a story** from `backlog/` — check MANIFEST.md for dependencies
2. **Move it** to `in-progress/` — update front matter `status: in-progress`
3. **Implement** following TDD: write test → make it pass → refactor
4. **Document** decisions in the "Implementation Notes" section
5. **Log progress** in the "Progress Log" section
6. **Move to `done/`** when all acceptance criteria are met — update `status: done`

### Moving a Story

```bash
# Start working on HT-01
mv stories/backlog/HT-01.md stories/in-progress/
# Update status in front matter: status: in-progress

# Complete HT-01
mv stories/in-progress/HT-01.md stories/done/
# Update status in front matter: status: done

# Block a story
mv stories/in-progress/HT-03.md stories/blocked/
# Update status in front matter: status: blocked
# Add the reason in "Implementation Notes"
```

---

## Picking the Next Story

### Rules

1. **Never start a story whose dependencies aren't in `done/`**
2. **Prefer stories on the critical path** (Track A or B first)
3. **Only one story in-progress at a time** (per developer)
4. **Read the relevant TECH_SPEC sections** before writing any code

### Recommended Start Order (Solo Developer)

```
HT-01 → HT-05 → HT-02 → HT-06 → HT-03 → HT-04 → HT-07 → HT-14 →
HT-08 → HT-09 → HT-17 → HT-13 → HT-10 → HT-18 → HT-11 → HT-12 →
HT-15 → HT-16 → HT-19 → HT-20 → HT-21 → HT-25 → HT-27 → HT-22 →
HT-23 → HT-24 → HT-26
```

### Quick Dependency Check

Before starting HT-XX, verify:

```bash
# List completed stories
ls stories/done/

# Check required dependencies in the story's front matter
grep "dependencies:" stories/backlog/HT-XX.md
```

---

## Branch & PR Workflow

**Main is protected.** Direct pushes to main are blocked (enforce_admins). ALL work goes through PRs with required CI checks.

### Branch conventions

Always create a branch before writing code. Use a worktree (`isolation: "worktree"`) or a regular branch — user's preference.

| Type | Pattern | Example |
|------|---------|---------|
| Feature / Story | `feat/<short-desc>` | `feat/ses-email-selector` |
| Bug fix | `fix/<short-desc>` | `fix/identity-grant-count` |
| Refactor | `refactor/<short-desc>` | `refactor/simplify-from-resolver` |
| CI / Infra | `ci/<short-desc>` | `ci/harden-workflows` |
| Docs only | `docs/<short-desc>` | `docs/task-completion-gate` |
| Story-specific | `feat/HT-<nn>-<short-desc>` | `feat/HT-17-echo-server` |

Rules:
- Lowercase, kebab-case, no spaces
- Short and descriptive (max ~40 chars)
- Delete branch after merge (PRs use `--delete-branch`)

### PR flow

1. `git checkout -b <type>/<desc>` (or use worktree)
2. Implement + run Task Completion Gate (lint, vet, test on ALL packages)
3. `git push -u origin <branch>`
4. `gh pr create --base main` with summary + test plan
5. Wait for CI (`backend` + `frontend` checks must pass)
6. `gh pr merge --squash --delete-branch`
7. `git checkout main && git pull`

## Implementation Protocol

### For Each Story:

1. **Read the story file** completely (objective, deliverables, acceptance criteria)
2. **Read the referenced TECH_SPEC sections** (listed in `spec_sections` front matter)
3. **Write tests first** (TDD — Red → Green → Refactor)
4. **Follow project conventions:**
   - Go module: `github.com/rendis/senda`
   - Folder structure per §9 of TECH_SPEC
   - Interfaces in `internal/port/`
   - Domain models in `internal/domain/`
   - Implementations in `internal/adapter/`
   - Services in `internal/service/`
   - HTTP handlers in `internal/http/handler/`
   - Middleware in `internal/http/middleware/`
5. **Use manual mocks** (no mock frameworks) — see TESTING_STRATEGY.md
6. **Integration tests** use TestContainers (tagged `//go:build integration`)
7. **Update the story** with implementation notes and progress log
8. **Run all tests** before marking done

### Task Completion Gate (mandatory)

**Before declaring ANY task complete**, run the fast local validation gate. Do NOT wait for the pre-push hook — catch errors immediately after finishing the code.

**Always run (every task, ALL packages — never skip to "just what I touched"):**

```bash
make lint                 # golangci-lint on repo Go packages only
make vet                  # Go vet on repo Go packages only
make test                 # go test on repo Go packages with race detector — validates nothing existing broke
```

If the task touches frontend (`web/`), also run:

```bash
pnpm --dir web typecheck         # tsc --noEmit (full project)
pnpm --dir web lint              # ESLint with sonarjs (--max-warnings=0, full project)
```

**On-demand only (not part of the standard gate):**

```bash
make test-integration     # Integration tests (requires Docker/TestContainers)
make test-e2e             # E2E deterministic gate (requires Docker/TestContainers)
make test-e2e-ses         # SES lifecycle E2E (requires MiniStack)
```

Run these heavier suites when the change touches infrastructure, adapters, or cross-layer flows — but do NOT block every task on them.

### Code Quality Gates

```bash
make test                 # Unit tests pass
make test-integration     # Integration tests pass (if applicable)
make test-e2e             # E2E deterministic gate (if applicable)
make lint                 # golangci-lint passes
make vet                  # No issues
```

### PR Local Gate (mandatory before opening or updating a PR)

For this repo, the primary validation happens locally before pushing changes. Use the GitHub-aligned gate targets so local validation and CI do not drift:

```bash
make ci-backend-pr
```

If the change also touches `web/`, run:

```bash
make ci-frontend
```

If the branch touches both backend and frontend, run:

```bash
make ci-pr
```

Do NOT wait for GitHub Actions to catch basic breakage. If you are pushing a branch that will back a PR, you MUST run the applicable local gate first. A PR must not be opened or updated with unverified changes.

And also run:

```bash
make ci-main
```

when the change touches end-to-end flows or systemic behavior, including:

- infrastructure and Docker
- adapters/providers (SES, Gmail, SMTP)
- webhooks and notifications
- workers / queues / River
- auth / RBAC / API keys
- onboarding
- UI/API flows that cross multiple layers

If the change modifies a critical part of the system and you did not run the relevant local battery, the PR is NOT ready.

To enforce this automatically before every push:

```bash
make install-githooks
```

The versioned pre-push hook runs the minimal required gate (`make ci-pr`) on every branch. Validation gates must stay Docker-free so pushes stay fast. Keep `make test-integration` and `make test-e2e` as explicit deeper suites when you want them, but they must not be part of the default validation path.

---

## Fundamental Code Rules

### Rule #0: NEVER iterate on broken code

**This is the most important rule in the project.** You do not iterate on errors or build on top of broken code. If something is wrong, the correct response is:

1. **STOP** — do not try to "make it work" with patches
2. **DIAGNOSE** — understand the root cause, not the symptom
3. **REFACTOR** — fix the design/approach, do not patch over the error
4. **REIMPLEMENT** — if the approach is fundamentally incorrect, rewrite it from scratch

**Forbidden anti-patterns:**

- Adding `if err != nil { // ignore }` to "skip over" an error
- Wrapping broken code in try/catch or recover just so it "does not fail"
- Copy-pasting code that is not fully understood
- Adding flags/booleans to "disable" the failing part
- Changing tests so they pass with incorrect behavior
- Accumulating TODO/FIXME items without resolving them before marking work as done

**The correct approach:**

- If a test fails → production code is wrong, not the test (unless the test itself is wrong)
- If an approach does not work after 2 attempts → rethink the design
- If you do not understand why something fails → read the spec again before touching code
- If you detect technical debt while implementing → refactor NOW, not "later"

### Frontend Methodology: Feature-First + Explicit Reuse

**Feature-first:** Each frontend HT is a complete vertical feature (UI + hooks + API calls + state + empty states). Do not build isolated horizontal layers.

**Mandatory reuse:**

1. **Before creating a component** → verify whether it already exists in `components/shared/` or in the Design System (HT-28)
2. **If a component is used in 2+ features** → extract it into `components/shared/` with generic props
3. **Predefined shared patterns** (created in HT-28):
   - `PageShell` — layout with sidebar + header + breadcrumbs
   - `DataTable` — table with sort, filter, and cursor-based pagination
   - `FormDialog` — modal with form + Zod validation
   - `EmptyState` — empty state with icon + message + CTA
   - `ConfirmDialog` — destructive confirmation dialog
   - `StatusBadge` — status badge with semantic colors
   - `ScopeIndicator` — level indicator (Global/Tenant/Workspace)
4. **Each HT documents** which components it reuses and which new ones it extracts

---

## Key Architecture Decisions

These are non-negotiable decisions documented in TECH_SPEC v1.4:

1. **No Redis** — PG UNLOGGED table for cache, PL/pgSQL for rate limiting
2. **Adapter assigned per template_type** — not per workspace or template
3. **Resolution chain** via scope hierarchy (workspace → _system → global)
4. **River** for job queue (Go + PG native, no external broker)
5. **OIDC for humans, API Keys for machines** — dual auth. Keys are SHA-256 hashed, workspace-scoped, raw shown once at creation. Blast radius of a compromised key = 1 workspace (data-plane only, no management access)
6. **Hexagonal architecture** — ports define contracts, adapters implement
7. **UUIDs v7** — time-ordered, non-sequential
8. **Partitioned tables** — emails and audit_logs by month
9. **Soft delete** — `deleted_at` column, never physical delete
10. **Cursor-based pagination** — no offset, UUIDv7 as cursor
11. **Provider-managed email auth** — SPF/DKIM/DMARC are the provider's responsibility (SES/Gmail), NOT the app. Senda validates sender capability via adapter identities (sync from provider + default identity). No DKIM signing, no DNS record management in app code. See [ADR-0001](docs/specs/ADR-0001-provider-managed-email-auth.md)
12. **No app-level email address validation** — `from_email` is verified by the provider's identity system. If an identity isn't verified, the provider rejects the send. Senda tracks identity status (`verified`/`pending`/`failed`) via `AdapterIdentity` but doesn't duplicate provider checks
13. **SDK extensibility model** — Senda exposes a public `sdk/` package. Users extend via code injectors (implement `sdk.Injector`), init functions (`sdk.InitFunc`), and lifecycle hooks (`OnStart`/`OnShutdown`). Built-in adapters (SES, Gmail, SMTP, PG stores, River, cache, crypto) stay internal, managed by YAML config. The `sdk.Engine` wraps `internal/app.Bootstrap` with an extensions bridge. Code injectors resolve alongside DB injectors in the `InjectorMerger`; on name collision, code wins with a warning. Pattern follows pdf-forge/doc-assembly SDK model.
14. **SES adapter lifecycle** — Provision (6 steps) and Deprovision (4 steps), both tracked in `adapter_provisioning_steps`. Delete permissions validated at creation. Full reference in `skills/senda/SKILL.md` "SES Adapter Lifecycle" and `docs/DEPLOYMENT.md`.

---

## SDK — Extending Senda as a Library

Senda exposes `sdk/` as a public Go package. External projects import it to add business-specific logic without forking.

### Public API (`sdk/`)

| Type | Purpose |
|---|---|
| `sdk.Engine` | Builder entry point. `New()` / `NewWithConfig(path)` → register extensions → `Run()` |
| `sdk.Injector` | Interface: `Code()`, `Resolve() (ResolveFunc, deps)`, `IsCritical()`, `Timeout()` |
| `sdk.ResolveFunc` | `func(ctx, *InjectorContext) (map[string]any, error)` — returns field values |
| `sdk.InitFunc` | `func(ctx, *InjectorContext) (any, error)` — runs once per request before injectors |
| `sdk.InjectorContext` | Read-only context: headers, variables, init data, tenant/workspace IDs, resolved values |

### How code injectors merge with DB injectors

1. `SendService.Send()` builds an `InjectorContext` with HTTP headers, send request data, and resolved tenant/workspace
2. `InjectorMerger.ResolveWithContext()` resolves DB injectors first → seeds context → runs `InitFunc` → resolves code injectors in dependency order
3. Code injector values merge into the same `map[string]map[string]any` as DB injectors
4. Templates access both via `{{ injector.<name>.<field> }}`
5. On name collision: code injector wins, warning logged

### Key files

| File | Role |
|---|---|
| `sdk/engine.go` | Engine struct + `Run()` (wraps `app.Bootstrap`) |
| `sdk/interfaces.go` | Type aliases → `internal/port.CodeInjector`, `CodeResolveFunc`, `CodeInitFunc` |
| `sdk/types.go` | Type alias → `internal/port.InjectorContext` |
| `internal/port/code_injector.go` | Real implementations: `InjectorContext`, `CodeInjector` interface |
| `internal/app/extensions.go` | `Extensions` struct bridging SDK → bootstrap |
| `internal/resolution/injector_merger.go` | `ResolveWithContext()` — merges DB + code injectors |

### Consumer project structure (like tools-pdf-forge)

```
my-senda-app/
├── main.go                  sdk.NewWithConfig("config.yaml") + Register(engine) + engine.Run()
├── extensions/
│   ├── register.go          Register(engine) — single entry point for all extensions
│   ├── init.go              InitFunc — load shared data per request
│   └── injectors/
│       ├── student.go       Custom injector: student data
│       └── institution.go   Custom injector: institution data
├── internal/                Private data sources (MongoDB, APIs, etc.)
├── config.yaml              Senda config (DB, OIDC, providers)
└── go.mod                   requires github.com/rendis/senda
```

---

## Available Commands

### Makefile Targets

| Category    | Command                         | Description                                               |
| ----------- | ------------------------------- | --------------------------------------------------------- |
| **Dev**     | `make dev`                      | Start full Docker stack (senda + PG + Keycloak + Mailpit) |
|             | `make dev-down`                 | Stop the stack                                            |
|             | `make dev-clean`                | Stop stack + remove volumes                               |
| **Build**   | `make build`                    | Build binary to `bin/senda`                               |
| **Test**    | `make test`                     | Unit tests with race detector                             |
|             | `make test-integration`         | Integration tests (TestContainers, requires PG)           |
|             | `make test-e2e`                 | E2E deterministic gate (starts stack, runs, stops)        |
|             | `make test-e2e-run`             | E2E deterministic (assumes stack running)                 |
|             | `make test-e2e-full`            | Full E2E suite incl. chaos (starts stack, stops)          |
|             | `make test-e2e-full-run`        | Full E2E suite (assumes stack running)                    |
|             | `make test-e2e-core`            | E2E core gate only (starts stack, stops)                  |
|             | `make test-e2e-core-run`        | E2E core gate (assumes stack running)                     |
|             | `make test-e2e-chaos`           | Chaos E2E suite (starts stack, stops)                     |
|             | `make test-e2e-chaos-run`       | Chaos E2E suite (assumes stack running)                   |
|             | `make test-e2e-up`              | Start E2E stack with --wait                               |
|             | `make test-e2e-down`            | Stop E2E stack + remove volumes                           |
| **System**  | `make system-validate-manifest` | Validate screen manifest vs app routes                    |
|             | `make system-matrix`            | Generate system coverage matrix CSV                       |
|             | `make system-pr`                | PR system gate (functional + UI + visual)                 |
|             | `make system-nightly`           | Nightly full gate (+ security + a11y)                     |
|             | `make system-down`              | Force-stop system E2E stack                               |
| **DB**      | `make migrate-up`               | Apply all pending migrations                              |
|             | `make migrate-down`             | Rollback last migration                                   |
| **Quality** | `make lint`                     | golangci-lint                                             |
| **Cleanup** | `make clean`                    | Remove build artifacts (bin/, tmp/)                       |
| **Help**    | `make help`                     | Show all targets with descriptions                        |

### Frontend Scripts

```bash
pnpm --dir web dev           # Next.js dev server (localhost:3000)
pnpm --dir web typecheck     # TypeScript validation
pnpm --dir web lint          # ESLint
```

---

## Dev Stack Services

| Service  | Default Port            | Purpose                      | Access URL                                         | Credentials   |
| -------- | ----------------------- | ---------------------------- | -------------------------------------------------- | ------------- |
| senda    | 8081                    | Backend API (Air hot-reload) | http://localhost:8081                              | —             |
| postgres | 5435                    | PostgreSQL 16 + pg_cron      | `psql postgres://senda:senda@localhost:5435/senda` | senda / senda |
| keycloak | 9090                    | OIDC provider                | http://localhost:9090                              | admin / admin |
| mailpit  | 1026 (SMTP) / 8026 (UI) | Email capture                | http://localhost:8026                              | —             |
| caddy    | 443                     | HTTPS proxy (optional)       | https://localhost                                  | —             |

### Keycloak Test Users

| Email                      | Password         | Role            |
| -------------------------- | ---------------- | --------------- |
| admin@senda.dev            | admin            | Superadmin      |
| tenant-admin@senda.dev     | tenant-admin     | TenantAdmin     |
| workspace-admin@senda.dev  | workspace-admin  | WorkspaceAdmin  |
| workspace-editor@senda.dev | workspace-editor | WorkspaceEditor |
| workspace-viewer@senda.dev | workspace-viewer | WorkspaceViewer |

---

## Spec Section Quick Reference

| Section | Content                            |
| ------- | ---------------------------------- |
| §1      | Principles & Glossary              |
| §3      | Complete SQL Schema (16+ tables)   |
| §4      | Partitioning Strategy              |
| §5      | Performance Indices                |
| §6      | App-Level Validations              |
| §7      | Migration Strategy (19 migrations) |
| §8      | Architecture & DI                  |
| §9      | Folder Structure                   |
| §10     | Port Interfaces                    |
| §11     | Domain Models                      |
| §12     | Resolution Engine                  |
| §13     | SendService Flow                   |
| §14     | Middleware Chain                   |
| §15     | API Contract                       |
| §16     | Background Workers                 |
| §17     | Configuration                      |
| §18     | Docker Compose                     |
| §19     | Provider Event Ingestion           |
| §20     | Onboarding Flow                    |
| §21     | Observability                      |
| §23     | PG Cache                           |
| §24     | Token Bucket Rate Limiting         |
