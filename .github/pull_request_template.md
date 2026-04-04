## Summary

- What changed?
- Why did it change?

## Context

- Related issue:
- Related story (if any):
- Risk / tradeoffs:

## Validation

Mark everything you actually ran.

- [ ] `make lint`
- [ ] `go vet ./...`
- [ ] `make test`
- [ ] `make test-integration`
- [ ] `make test-e2e` (required for systemic or cross-layer changes)
- [ ] `npm --prefix web run typecheck`
- [ ] `npm --prefix web run lint -- --max-warnings=0`

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
