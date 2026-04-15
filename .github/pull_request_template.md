## Summary

- What changed?
- Why did it change?

## Context

- Related issue:
- Related story (if any):
- Risk / tradeoffs:

## Validation

Mark everything you actually ran.

- [ ] `make ci-backend-pr` (if backend, infra, migrations, or tests changed)
- [ ] `make ci-frontend` (if `web/` changed)
- [ ] `make ci-pr` (if both backend and frontend changed)
- [ ] `make ci-taxonomy-check` (if you changed docs, workflows, Makefile targets, or CI helper scripts that define the public validation contract)

Optional deeper checks when you intentionally want Docker-backed coverage:

- [ ] `make test-integration`
- [ ] `make test-e2e`

## Change type

- [ ] Bug fix
- [ ] Feature
- [ ] Refactor
- [ ] Docs
- [ ] Test-only
- [ ] Security-sensitive

## Contributor checklist

- [ ] I kept the change focused and reviewable.
- [ ] I added or updated tests where behavior changed.
- [ ] I updated docs when behavior, setup, API, or workflows changed.
- [ ] I did not add build-only validation as a substitute for the required local gates.
- [ ] I did not add AI attribution or `Co-Authored-By` trailers.

## Compatibility / ops impact

- [ ] No migration
- [ ] Migration included
- [ ] No config changes
- [ ] Config changes included
- [ ] No security impact
- [ ] Security impact described above
