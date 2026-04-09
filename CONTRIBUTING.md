# Contributing to Senda

Thanks for contributing to Senda. Here's the thing: good OSS collaboration is not just about sending code — it is about sending changes that are understandable, reviewable, and safe to maintain.

This guide explains the standard contribution flow for this repository.

## Before you start

Please read these documents first:

- [README.md](README.md) — project overview and quick start
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) — local setup, services, and commands
- [SECURITY.md](SECURITY.md) — private reporting for vulnerabilities
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — community expectations

For substantial changes, open an issue before writing code so the maintainers can confirm scope, priority, and fit.

## What to contribute

Contributions are welcome in these areas:

- Bug fixes
- Focused features that fit the roadmap
- Test coverage improvements
- Documentation improvements
- Developer experience and tooling improvements

Please avoid drive-by large refactors without prior discussion. Small, scoped pull requests are much easier to review and merge.

## Standard OSS flow in this repo

1. Fork the repository.
2. Create a focused branch from `main`.
3. Make the smallest change that solves one problem.
4. Add or update tests first when behavior changes. **TDD is mandatory in this repo.**
5. Run the relevant local validation commands — ideally through the same gate targets GitHub uses.
6. Update documentation when behavior, APIs, workflows, or setup change.
7. Open a pull request with context, validation details, and tradeoffs.

## Local setup

### Backend

```bash
git clone https://github.com/rendis/senda.git
cd senda
make dev
```

Verify the stack:

```bash
curl http://localhost:8081/health
```

### Frontend

```bash
corepack enable
(cd web && corepack install)
pnpm --dir web install
pnpm --dir web dev
```

For more detail, see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Validation rules

Run the command that matches the surface you changed. These targets are the same ones GitHub Actions runs, so there is one source of truth for local validation and CI.

The required validation gates are intentionally **Docker-free**. Heavier suites remain available as explicit commands when you want extra coverage, but they are not part of the default validation path.

### Backend changes

```bash
make ci-backend-pr
```

### Frontend changes

```bash
make ci-frontend
```

### Systemic or cross-layer changes

Also run:

```bash
make ci-main
```

This includes changes touching infrastructure, Docker, workers, queues, auth, onboarding, adapters/providers, webhooks, or end-to-end API/UI flows.

### Full PR validation

If a branch touches both backend and frontend, run:

```bash
make ci-pr
```

### Enforce the gate locally on every push

To make the repo run the same validation automatically before every push:

```bash
make install-githooks
```

That configures `core.hooksPath=.githooks`, and the versioned `pre-push` hook runs the minimal required push gate:

- `make ci-pr`

`make ci-main` remains available as the fast main-branch validation gate, but the automatic pre-push hook still uses `make ci-pr` so everyday pushes stay predictable and Docker-free.

### Explicit Docker-backed suites

These are still available locally, but they are no longer part of the required validation gate:

```bash
make test-integration
make test-e2e
```

> Do not add a build step just because files changed. In this repository, the required contributor gate is lint + tests + vet, plus E2E when the change is systemic.

## Testing expectations

- **TDD is required**: write the failing test first, then make it pass, then refactor.
- Use **manual mocks**. Do not introduce mock frameworks.
- Keep unit tests next to the code they validate.
- Use Testcontainers for integration tests.
- Tag integration tests with `//go:build integration`.
- Tag end-to-end tests with `//go:build e2e` when applicable.

If a test fails, stop and fix the design or implementation. Do not patch around broken behavior.

## Commit style

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

Examples:

- `feat: add workspace webhook retry settings`
- `fix: resolve tenant scope cache invalidation`
- `docs: clarify SES lifecycle prerequisites`

Repository-specific rules:

- Do **not** add `Co-Authored-By` trailers.
- Do **not** add AI attribution to commits or PRs.
- Keep commits intentional and easy to review.

## Pull request expectations

A strong PR in Senda is:

- Small enough to review in one sitting
- Backed by the right tests
- Documented when behavior changes
- Clear about tradeoffs, risks, and follow-up work

Before opening a PR, make sure you can explain:

- What changed
- Why it changed
- How it was validated
- Whether docs, migrations, config, or security implications exist

Use the PR template and fill it in completely.

## Issues, support, and triage

Use the right channel:

- **Bug report**: reproducible incorrect behavior
- **Feature request**: scoped proposal with problem statement and expected outcome
- **Security issue**: follow [SECURITY.md](SECURITY.md) and report privately
- **Docs improvement**: open an issue or PR directly if the change is obvious and low risk

Before opening an issue:

1. Check the README and docs.
2. Search existing issues and pull requests.
3. Reduce the report to one problem per issue.

## Working with the story system

Senda uses a `stories/` workflow for planned work.

- If you are continuing maintainer-directed work, check `stories/in-progress/` first.
- If nothing is in progress, review `stories/MANIFEST.md`.
- For brand-new work, start with an issue first; maintainers may decide to track larger work through the story system.

Do not move or rewrite project planning artifacts unless the maintainers explicitly ask for it.

## Documentation expectations

Update docs when you change:

- Public behavior
- Setup or developer workflow
- API contracts
- Security-sensitive behavior
- Architecture decisions that affect contributors or operators

If code and docs disagree, the PR is not ready.

## Need to report abusive or unacceptable behavior?

Please see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for reporting instructions.
