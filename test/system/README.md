# System Test Harness (Full Coverage)

This directory contains the full-system test harness for Senda:

- Backend deterministic tests + contract verification.
- Security/chaos suites.
- UI flow traversal for all routes/scopes/roles/locales/viewports.
- Visual diff against golden + Pencil baselines.
- Accessibility checks (axe + semantic checks).

## Commands

- PR gate:
  - `make system-pr`
- Nightly full gate:
  - `make system-nightly`
- Validate route coverage manifest:
  - `make system-validate-manifest`
- Generate matrix only:
  - `make system-matrix`

Main orchestrator:

- `test/system/system-runner.sh pr`
- `test/system/system-runner.sh nightly`

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
