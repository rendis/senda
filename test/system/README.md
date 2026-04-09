# System Test Harness (Full Coverage)

This directory contains the full-system test harness for Senda:

- Backend deterministic tests + contract verification.
- Security/chaos suites.
- UI flow traversal for all routes/scopes/roles/locales/viewports.
- Visual diff against golden + Pencil baselines (opt-in).
- Accessibility checks (axe + semantic checks).

## Commands

- PR gate:
  - `make system-pr`
- Nightly full gate:
  - `make system-nightly`
- Optional visual baseline pass:
  - `SYSTEM_UI_VISUAL=1 make system-pr`
  - `SYSTEM_UI_VISUAL=1 make system-nightly`
- Validate route coverage manifest:
  - `make system-validate-manifest`
- Generate matrix only:
  - `make system-matrix`

Main orchestrator:

- `test/system/system-runner.sh pr`
- `test/system/system-runner.sh nightly`

The orchestrator now expects a unified stack lifecycle command from `cmd/systemtest`:

- `stack up --mode <pr|nightly> --out <env-report.json>`
- `stack down --out <env-report.json>`

`env-report.json` becomes the source of truth for service base URLs consumed by the subagents. The frontend remains host-side and is still started locally by the UI stages.

## Subagents

- `subagents/infra-orchestrator.sh`
- `subagents/api-contract-tester.sh`
- `subagents/security-chaos-tester.sh`
- `subagents/ui-flow-tester.sh`
- `subagents/ui-workspace-management-tester.sh`
- `subagents/ui-adapter-sharing-tester.sh`
- `subagents/ui-injector-flow-tester.sh`
- `subagents/ui-onboarding-auth-guard-tester.sh`
- `subagents/ui-visual-tester.sh`
- `subagents/ui-a11y-tester.sh`

## Contracts

- `screen-manifest.json`: route/scenario manifest (must cover all `web/src/app/**/page.tsx` routes).
- `visual-baseline-map.json`: route -> Pencil frame + criticality.
- `run-result.schema.json`: unified run-result contract.

Validation is enforced by:

- `go run ./cmd/systemtest validate-manifest ...`

If a new route appears without manifest scenario + baseline map entry, the gate fails.

## Baselines

- Goldens: `test/system/baselines/golden`
- Pencil: `test/system/baselines/pencil`

By default, PR and nightly runs skip the visual baseline diff stage and use functional,
security, accessibility, and manual QA as the release blockers. Set `SYSTEM_UI_VISUAL=1`
when you explicitly want to generate screenshots and compare them against baselines.

Expected naming convention:

- `<routeSlug>.<viewport>.<locale>.png`
- Example: `global__templates__slug__edit.desktop.en.png`

`routeSlug` is generated from route by:

- remove leading `/`
- replace `/` with `__`
- remove `[` and `]`
- `/` root route => `root`

## Artifacts

Each run writes to:

- `artifacts/system/<timestamp>/`

Key outputs:

- `functional-junit.xml`
- `api-contract-report.md`
- `security-chaos-report.md`
- `ui-flow-report.md`
- `ui-workspace-management-report.md`
- `ui-adapter-sharing-report.md`
- `ui-injector-flow-report.md`
- `ui-onboarding-auth-guard-report.md`
- `visual-diff-report.html`
- `a11y-report.md`
- `coverage-matrix.csv`
- `run-result.json`
- `stage-results.tsv`

## Injector UI flow stage

`subagents/ui-injector-flow-tester.sh` is the dedicated browser/system regression for the
new workspace-owned injector model.

It uses the runtime env report from `infra-orchestrator`, starts the frontend locally, logs in
through OIDC, and validates the full UI flow against the running backend + Mailpit stack.

Coverage:

- injector management in the workspace UI
- per-field `default_value`
- per-field `allow_overwrite`
- locked-field validation (`allow_overwrite=false` requires default)
- template editor token palette shows only workspace injectors grouped by field
- test-send modal:
  - locked fields are read-only
  - untouched overwriteable fields fall back to runtime precedence
  - explicit overrides win
  - explicit empty override renders empty string
- bulk-send modal accepts per-item `injectors` JSON overrides
- Mailpit confirms rendered output for:
  - default-only
  - reqBody override
  - code fallback
  - locked field behavior
  - bulk-send partial fallback per field

Useful direct invocation:

```bash
ARTIFACT_DIR=/tmp/senda-ui-injector-flow \
ENV_REPORT_FILE=artifacts/system/<timestamp>/env-report.json \
FRONTEND_BASE_URL=http://localhost:3000 \
test/system/subagents/ui-injector-flow-tester.sh
```

## Onboarding auth guard stage

`subagents/ui-onboarding-auth-guard-tester.sh` verifies that `/onboarding` does not leak
wizard step 2/3 when the browser is unauthenticated but still has stale onboarding state
in `sessionStorage`.

Coverage:

- seeds stale `senda-onboarding` sessionStorage state
- opens `/onboarding` in a fresh unauthenticated browser session
- expects redirect to `/login`
- rejects leaked onboarding content such as “Create your organization” or “Create your workspace”

## Environment defaults

- Backend: `http://localhost:8090`
- Frontend: `http://localhost:3000`
- Keycloak: `http://localhost:9090`
- Mailpit: `http://localhost:9025`

Configurable via env vars in `test/system/subagents/lib.sh`.
