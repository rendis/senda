
# System Test Harness (Full Coverage)

This directory contains the full-system test harness for Senda:

- backend deterministic and contract coverage
- chaos / recovery validation
- UI route and flow traversal
- environment-mode validation for `prod` / `test`
- accessibility and optional visual diff stages

## Commands

- `make system-pr`
- `make system-nightly`
- `make system-validate-manifest`
- `make system-matrix`
- `make system-down`

## Main orchestrator

- `test/system/system-runner.sh pr`
- `test/system/system-runner.sh nightly`

The harness expects the unified stack lifecycle via `cmd/systemtest` and consumes the generated env report as the source of truth for base URLs.

## Subagents

- `subagents/infra-orchestrator.sh`
- `subagents/api-contract-tester.sh`
- `subagents/security-chaos-tester.sh`
- `subagents/ui-flow-tester.sh`
- `subagents/ui-workspace-management-tester.sh`
- `subagents/ui-environment-mode-tester.sh`
- `subagents/ui-adapter-sharing-tester.sh`
- `subagents/ui-injector-flow-tester.sh`
- `subagents/ui-onboarding-auth-guard-tester.sh`
- `subagents/ui-visual-tester.sh`
- `subagents/ui-a11y-tester.sh`

## Environment-mode stage

`ui-environment-mode-tester.sh` validates the browser-facing environment model:

- switch between `prod` and `test`
- environment-aware navigation
- environment-scoped workspace state
- test-only workspace controls
- template-type test recipient override controls

## Outputs

The harness writes reports such as:

- `api-contract-report.md`
- `security-chaos-report.md`
- `ui-flow-report.md`
- `ui-workspace-management-report.md`
- `ui-environment-mode-report.md`
- `ui-adapter-sharing-report.md`
- `ui-injector-flow-report.md`
- `ui-onboarding-auth-guard-report.md`
- `a11y-report.md`
- `run-result.json`
