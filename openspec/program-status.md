# Senda SDD Program Status

## Cycle 1 — Global Board

| change | estado | fase | dependencia/paralelo | reviewer final | último E2E | último score/panel | worktree |
| --- | --- | --- | --- | --- | --- | --- | --- |
| autonomous-e2e-isolation | done | approved | base del ciclo; habilitador completo | James | dual-stack smoke ok (runtime.artifact_dir validado, cleanup sin residuos) | baseline audit 6.1/10 | `/.worktrees/spec-autonomous-e2e-isolation` |
| security-perimeter-hardening | done | approved | paralelo habilitado tras autonomous-e2e-isolation | Lorentz | E2E/security evidence green; media pinning fail-closed IPv4+IPv6 | baseline audit 6.1/10 | `/.worktrees/spec-security-perimeter-hardening` |
| send-core-rework | done | approved | paralelo habilitado tras autonomous-e2e-isolation | Volta + Kuhn | evidencia autónoma/black-box green; batch y burst model aprobados | baseline audit 6.1/10 | `/.worktrees/spec-send-core-rework` |
| surface-modularization-and-sdk-hardening | done | approved | habilitado tras send-core-rework | Volta | E2E autónomo green; signoff de arquitectura aprobado | baseline audit 6.1/10 | `/.worktrees/spec-surface-modularization-and-sdk-hardening` |
| ci-gates-and-doc-alignment | done | approved | paralelo habilitado tras autonomous-e2e-isolation | James | `make ci-frontend` OK; taxonomía CI alineada | baseline audit 6.1/10 | `/.worktrees/spec-ci-gates-and-doc-alignment` |

## Cycle 2 — Re-auditoría ciega y nuevos streams

- Resultado panel fresco:
  - seguridad: **5/10**
  - arquitectura: **8/10**
  - performance: **7/10**
  - maintainability/DX: **7/10** (testing/confianza operativa: **8/10**)
- Conclusión: el umbral **> 8.5** todavía NO se cumple, así que se abre ciclo 2.

| change | estado | fase | dependencia/paralelo | reviewer final | último E2E | último score/panel | worktree |
| --- | --- | --- | --- | --- | --- | --- | --- |
| sns-and-public-perimeter-hardening | done | approved | paralelo con send; composición sigue diferida hasta cerrar reviews de seams activos | Lorentz | `make test-e2e-ses` + slices locales verdes | security re-audit 5/10 | `/.worktrees/spec-sns-and-public-perimeter-hardening` |
| send-path-amortization-and-cache-precision | done | approved | rework final cerrado y aprobado sin tocar `app.go`/`server.go` | Kuhn + Volta | batch E2E verde + budgets documentados | performance re-audit 7/10 | `/.worktrees/spec-send-path-amortization-and-cache-precision` |
| composition-boundary-slimming | done | approved | implementación/documentación cerradas y aprobadas | Volta | slices representativos de management/data-plane/external/SES inbound verdes | architecture re-audit 8/10 | `/.worktrees/spec-composition-boundary-slimming` |
| ci-taxonomy-and-test-discoverability | done | approved | paralelo con todos; depende conceptualmente de `ci-gates-and-doc-alignment` ya cerrado | James | `make ci-taxonomy-check` + `make ci-pr` verdes | DX re-audit 6-7/10 | `/.worktrees/spec-ci-taxonomy-and-test-discoverability` |

## Cycle 3 — Re-auditoría sobre `main` integrado

- Resultado panel fresco sobre `main` absorbido:
  - seguridad: **7.1/10**
  - arquitectura: **8.2/10**
  - performance: **8.3/10**
  - maintainability/DX: **8.2/10**
- Conclusión: el umbral **> 8.5** todavía NO se cumple, así que se abre ciclo 3.
- Handoff operativo para la próxima sesión: `openspec/session-handoff-cycle-3.md`

| change | estado | fase | dependencia/paralelo | reviewer final | último E2E | último score/panel | worktree |
| --- | --- | --- | --- | --- | --- | --- | --- |
| perimeter-identity-default-deny | in_progress | apply | ejecución directa; OIDC `sub+iss`, logout origin, metrics token, SNS default-deny y validator no destructivo ya aplicados; falta rematar media hot path/pinning | Lorentz | slice unit+integration verde; falta verify final | security re-audit 7.1/10 | `/.worktrees/spec-perimeter-identity-default-deny` |
| sdk-and-http-composition-decoupling | planned | proposal | paralelo seguro con security si no toca el mismo wiring; agrupa SDK estable, composer por superficie y server explícito | Volta | pendiente | architecture re-audit 8.2/10 | `/.worktrees/spec-sdk-and-http-composition-decoupling` |
| send-context-and-media-hotpath | planned | proposal | coordinar con architecture; unifica fuente de verdad de send context y optimiza hot path de thumbnails/cache/transport | Kuhn + Volta | pendiente | performance re-audit 8.3/10 | `/.worktrees/spec-send-context-and-media-hotpath` |
| ci-drift-and-cache-pagination | in_progress | apply | ejecución directa; cursor completo en invalidación por tenant ya aplicado, falta taxonomy check sobre PR template/AGENTS | James | invalidación paginada verde; falta verify DX | DX re-audit 8.2/10 | `/.worktrees/spec-ci-drift-and-cache-pagination` |
