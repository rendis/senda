# Status

- state: done
- percent: 100%
- dependency: cycle-2 closed
- worktree: `main`
- reviewer_final: worker verification
- notes:
  - el recorrido completo por cursor en `InvalidateTenantWorkspaces` ya estaba absorbido en `main` y sigue cubierto por test
  - `ci-taxonomy-check` ahora cubre también `.github/pull_request_template.md` y `AGENTS.md` como fuentes del contrato operativo
  - ambos documentos quedaron alineados con los gates públicos vigentes (`ci-backend-pr`, `ci-frontend`, `ci-pr`, `ci-taxonomy-check`) y sin referencias stale a `make ci-main`
  - el stream ya cuenta con verify report explícito y no deja trabajo real pendiente
- DoD: contratos operativos sin drift + invalidación completa + signoff DX
