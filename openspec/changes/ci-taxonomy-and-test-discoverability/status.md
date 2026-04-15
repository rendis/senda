# Status

- state: done
- percent: 100%
- dependency: ci-gates-and-doc-alignment
- worktree: `/.worktrees/spec-ci-taxonomy-and-test-discoverability`
- reviewer_final: James
- parallel_with:
  - send-core-rework
  - surface-modularization-and-sdk-hardening
  - security-perimeter-hardening
  - autonomous-e2e-isolation
- notes: taxonomía CI y descubribilidad frontend cerradas; `make ci-taxonomy-check` y `make ci-pr` quedaron verdes; el warning del runner frontend se resolvió con `web/package.json` (`type: module`) y `web/tests/test-root.mjs`
- DoD: taxonomía CI honesta + entrypoint frontend canónico + validación autónoma de drift
