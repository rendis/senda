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
- `visual-diff-report.html`
- `a11y-report.md`
- `coverage-matrix.csv`
- `run-result.json`
- `stage-results.tsv`

## Environment defaults

- Backend: `http://localhost:8090`
- Frontend: `http://localhost:3000`
- Keycloak: `http://localhost:9090`
- Mailpit: `http://localhost:9025`

Configurable via env vars in `test/system/subagents/lib.sh`.
