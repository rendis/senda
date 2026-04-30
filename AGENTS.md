# Senda — Agent Operating Guide

This file is the **operational entry point** for any agent or developer working on Senda. It tells you how to *operate* the repo (workflow, gates, conventions). It does **not** duplicate architecture, specs, or product context — those live under `docs/`, `docs/specs/`, `stories/`, and `skills/senda/`.

If you need to understand *what* Senda is or *why* a piece works the way it does, follow the links in [Documentation Map](#documentation-map). Do not re-derive that context here.

---

## Project at a glance

- **Module:** `github.com/rendis/senda`
- **Backend:** Go 1.26+ · PostgreSQL 16 + pg_cron · Echo v5 · River · pgx v5 · gomjml · golang-migrate
- **Frontend:** Next.js 16 · React 19 · TypeScript 5 · Tailwind v4 · shadcn/ui (`web/`)
- **Architecture:** Hexagonal (ports & adapters), no Redis, OIDC + API keys, hierarchical resolution.
- **Top-level layout:** `sdk/` (public SDK), `cmd/senda/` (binary), `internal/` (domain · port · service · resolution · adapter · http · app), `pkg/`, `migrations/`, `config/`, `web/`, `docs/`, `stories/`, `skills/senda/`.

For anything deeper (entity model, resolution chain, send pipeline, schema, etc.) → [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`docs/specs/TECH_SPEC_v1.md`](docs/specs/TECH_SPEC_v1.md).

---

## Documentation Map

Treat this table as the routing layer. Always prefer the linked doc over re-explaining things in this file.

| Need | Go to |
| --- | --- |
| System design, layers, resolution flow, ADRs | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) |
| HTTP API (endpoints, auth, RBAC, errors, pagination, webhooks) | [`docs/API.md`](docs/API.md) |
| Local setup, Docker stacks, testing tiers, troubleshooting | [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) |
| Production deployment, env vars, providers, OIDC | [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) |
| End-to-end email flows | [`docs/EMAIL_FLOWS.md`](docs/EMAIL_FLOWS.md) |
| Extending Senda as a Go library (SDK, injectors, init, hooks) | [`docs/extensibility-guide.md`](docs/extensibility-guide.md) |
| Product requirements, user stories, business rules | [`docs/specs/PRD_v5.md`](docs/specs/PRD_v5.md) |
| Authoritative technical spec (~5k lines) | [`docs/specs/TECH_SPEC_v1.md`](docs/specs/TECH_SPEC_v1.md) |
| Story catalogue + dependency graph | [`docs/specs/TECH_STORIES.md`](docs/specs/TECH_STORIES.md) |
| Testing strategy, mocking patterns, coverage targets | [`docs/specs/TESTING_STRATEGY.md`](docs/specs/TESTING_STRATEGY.md) |
| Security checklist, OWASP mapping, crypto/key model | [`docs/specs/SECURITY_CHECKLIST.md`](docs/specs/SECURITY_CHECKLIST.md) |
| UX/UI brief, design tokens, screen specs | [`docs/specs/DESIGN_BRIEF.md`](docs/specs/DESIGN_BRIEF.md) |
| ADRs (provider-managed email auth, transitional fallback) | [`docs/specs/ADR-0001-...md`](docs/specs/ADR-0001-provider-managed-email-auth.md), [`ADR-0002-...md`](docs/specs/ADR-0002-transitional-email-fallback-unbound-members.md) |
| Postman collection + environments | [`docs/postman/`](docs/postman/) |
| MCP server setup | [`docs/mcp_setup.md`](docs/mcp_setup.md) |
| Story status (backlog / in-progress / done / blocked) | [`stories/MANIFEST.md`](stories/MANIFEST.md) |
| Self-contained agent contract for MCP / external embedders | [`skills/senda/SKILL.md`](skills/senda/SKILL.md) + `skills/senda/references/` |

---

## Stories & roadmap

The HT-driven build phase is closed. Current state lives in [`stories/MANIFEST.md`](stories/MANIFEST.md) — that file owns counters, status, dependency graph, and recommended order. Do not duplicate any of it here.

When you pick up new work:

1. If it maps to an existing story, follow the lifecycle in `stories/MANIFEST.md` (move `backlog → in-progress → done`, update front matter, log progress in the story file).
2. If it is a fix or enhancement outside the HT catalogue, just open a feature/fix branch and PR (see [Branch & PR workflow](#branch--pr-workflow)).

---

## Working agreements (non-negotiable)

These are the rules that survive every change. Spec details and rationales are in `docs/`.

1. **Never iterate on broken code.** Diagnose root cause, refactor or reimplement. No `if err != nil { /* ignore */ }`, no flags to silence failures, no tests rewritten to fit incorrect behavior, no TODO/FIXME deferred past the task.
2. **TDD is mandatory** for backend logic. Test first, make it pass, refactor. Manual mocks only — no mock frameworks. Integration tests use TestContainers (`//go:build integration`); E2E uses `//go:build e2e` and the Docker stack.
3. **Hexagonal boundaries hold.** Domain has zero infra deps. Ports define contracts (`internal/port/`), adapters implement (`internal/adapter/`). Services compose ports.
4. **Soft delete, UUIDv7, cursor pagination, partitioned tables.** These are platform invariants — see `docs/ARCHITECTURE.md` and `docs/specs/TECH_SPEC_v1.md` §3–§5 if you need details.
5. **Frontend reuse first.** Before adding a component, check `web/src/components/shared/` and the design system. Promote any component used by 2+ features into `shared/`.
6. **Do not parse `senda_desing.pen` as JSON.** Always go through the Pencil MCP (`batch_get`, `get_screenshot`, `get_variables`, `batch_design`, `snapshot_layout`). Pencil must be open for the MCP to be reachable.

---

## Task Completion Gate

**Before declaring any task complete**, run the local gate. Do not rely on CI to catch breakage.

Always (every task, all packages — never narrow it to "just what I touched"):

```bash
make lint    # golangci-lint
make vet     # go vet
make test    # go test -race on all repo packages
```

If the change touches `web/`:

```bash
pnpm --dir web typecheck
pnpm --dir web lint
```

For PRs the GitHub-aligned gates are the source of truth — run the one that matches the surface(s) you touched:

```bash
make ci-backend-pr   # backend-only PR
make ci-frontend     # frontend-only PR
make ci-pr           # both surfaces
```

Run `make ci-taxonomy-check` whenever the change touches docs, workflows, the Makefile, or helper scripts that define the public validation contract.

Heavier suites (`make test-integration`, `make test-e2e`, `make test-e2e-ses`, `make system-pr`, `make system-nightly`) are **on-demand**. Trigger them when the change touches infrastructure, providers/adapters, webhooks, queues, auth/RBAC, onboarding, or any cross-layer flow. Do not block every task on them.

To enforce the minimum gate on every push:

```bash
make install-githooks
```

The pre-push hook runs `make ci-pr` and stays Docker-free so pushes are fast.

---

## Branch & PR workflow

`main` is protected — no direct pushes. All work goes through PRs.

Branch naming (lowercase, kebab-case, ≤ ~40 chars, delete after merge):

| Type | Pattern | Example |
| --- | --- | --- |
| Feature / story | `feat/<desc>` or `feat/HT-<nn>-<desc>` | `feat/ses-email-selector`, `feat/HT-17-echo-server` |
| Bug fix | `fix/<desc>` | `fix/identity-grant-count` |
| Refactor | `refactor/<desc>` | `refactor/simplify-from-resolver` |
| CI / Infra | `ci/<desc>` | `ci/harden-workflows` |
| Docs only | `docs/<desc>` | `docs/agents-md-slim` |

Flow:

1. Create branch (or worktree).
2. Implement + run the applicable Task Completion Gate.
3. `git push -u origin <branch>`.
4. `gh pr create --base main` with summary + test plan.
5. Wait for CI (`backend` and/or `frontend` checks must pass).
6. `gh pr merge --squash --delete-branch`.
7. `git checkout main && git pull`.

Conventional Commits. No AI attribution / `Co-Authored-By` lines on commits or PRs.

---

## Skill maintenance (`skills/senda/`)

`skills/senda/` is the **agent-facing contract** for operating Senda from outside this repo (MCP, external embedders, integrators). Stale skill content silently breaks those consumers — treat it as production code, not docs.

**Hard rule — visual builder & variable engine.** If a PR adds, renames, removes, or changes the MJML output of a builder block (`web/src/components/templates/mjml-editor.tsx`, `text-block-mjml.ts`, `video-block.ts`, …) or touches `internal/service/variable_renderer.go`, you MUST update `skills/senda/references/building-a-template.md` and `versions-locales-and-builder.md` in the **same PR**.

**Hard rule — MJML composition rules.** If a PR changes which MJML / HTML tags are allowed or forbidden in `body_mjml` (e.g. adds a new `mj-*` tag the builder emits, relaxes or tightens an HTML-document-tag block, changes `<mj-raw>` semantics), you MUST update `skills/senda/scripts/mjml-check.sh` (rule patterns) AND `skills/senda/scripts/mjml-check.test.sh` (fixtures) in the **same PR**. The skill mandates `mjml-check.sh` as a pre-submit gate; if the script and the docs disagree, agents will produce broken templates.

**General rule.** Before closing any task, ask: *“If a fresh agent reads `skills/senda/` after this PR is merged, will it still get a correct picture of how to operate Senda?”* If the answer is anything other than a confident yes, update the skill in the same PR.

The full decision table (which references to touch when an HTTP route, RBAC matrix, enum, webhook event, SDK type, etc. changes) lives in [`skills/senda/SKILL.md`](skills/senda/SKILL.md). Use it as the checklist; do not re-implement it here.

---

## Dev stack (quick reference)

| Service  | Port(s)              | URL                                              | Credentials   |
| -------- | -------------------- | ------------------------------------------------ | ------------- |
| senda    | 8081                 | http://localhost:8081                            | —             |
| postgres | 5435                 | `postgres://senda:senda@localhost:5435/senda`    | senda / senda |
| keycloak | 9090                 | http://localhost:9090                            | admin / admin |
| mailpit  | 1026 SMTP / 8026 UI  | http://localhost:8026                            | —             |
| caddy    | 443                  | https://localhost (optional)                     | —             |

Keycloak test users (per role): `admin@senda.dev`, `tenant-admin@senda.dev`, `workspace-admin@senda.dev`, `workspace-editor@senda.dev`, `workspace-viewer@senda.dev`. Password = local-part of the email. Full setup, Docker stacks, and troubleshooting in [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md).

---

## Make targets

`make help` lists every target with its description. The day-to-day shortcuts:

| Purpose | Targets |
| --- | --- |
| Dev stack | `make dev`, `make dev-down`, `make dev-clean` |
| Build | `make build` |
| Local gates | `make lint`, `make vet`, `make test` |
| PR gates | `make ci-backend-pr`, `make ci-frontend`, `make ci-pr`, `make ci-taxonomy-check` |
| Heavy suites | `make test-integration`, `make test-e2e`, `make test-e2e-ses`, `make system-pr`, `make system-nightly` |
| DB | `make migrate-up`, `make migrate-down` |
| Frontend | `pnpm --dir web dev`, `pnpm --dir web typecheck`, `pnpm --dir web lint` |

For the full inventory (E2E variants, system gates, cleanup, etc.) run `make help` or read the `Makefile` directly.
