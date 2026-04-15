# Verify Report — ci-gates-and-doc-alignment

## Summary

The repository now exposes an honest frontend test entrypoint, and the `main` taxonomy is documented as local convenience wrappers rather than GitHub CI gates.

## Verified commands

- `corepack pnpm --dir web typecheck` ✅
- `corepack pnpm --dir web lint --max-warnings=0` ✅
- `corepack pnpm --dir web test` ✅
- `make ci-frontend` ✅
- Focused taxonomy check over `README.md`, `docs/DEVELOPMENT.md`, `docs/specs/TESTING_STRATEGY.md`, `Makefile`, and `scripts/run-github-gates.sh` ✅

## Result

The frontend gate is still green, and the `main` wrappers are now explicitly local-only.

Reviewer final: `James` → **APPROVED**

### Failing assertions

None for this batch. The stale assertions were already rebaselined; this batch fixed the CI/main taxonomy drift.

## What changed

- Added `web/package.json` script `test`.
- Implemented `web/scripts/run-tests.mjs` so the canonical frontend test entrypoint runs from the repo root, which is required by the existing path-based tests.
- Aligned `scripts/run-github-gates.sh`, `Makefile`, and the GitHub workflows to the same frontend gate sequence.
- Reworded the system gate as manual/observational.
- Updated README and testing strategy docs so they no longer promise gates that do not exist.
- Rebaselined the stale frontend UI assertions so the suite reflects the current markup, including the display-code toggle label, the Bulk Send enablement condition, the `_system` workspace default, and the scope indicator translation key.
- Reworded `ci-backend-main` and `ci-main` as local convenience wrappers and removed any implication that GitHub CI runs push-to-main workflows for them.

## Remaining work

None for this stream. The frontend gate is green and the change is ready for review.

## Final signoff

James approved the stream because:

1. `ci-backend-main` and `ci-main` are now described consistently as local convenience wrappers, not GitHub CI gates.
2. The CI surface is aligned with the real workflows: backend/frontend on `pull_request`, `system-pr` as manual/observational.
3. The frontend contract is honest and executable through `web/package.json` + `web/scripts/run-tests.mjs`, and `make ci-frontend` stays green.
