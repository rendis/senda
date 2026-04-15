# Status

- state: done
- percent: 100%
- dependency: rereview
- worktree: `/.worktrees/spec-send-path-amortization-and-cache-precision`
- reviewer_final: "Kuhn + Volta"
- notes:
  - el stream cerró los dos blockers finales del re-review: el contexto compartido ahora parte desde la locale efectiva del workspace cuando corresponde, preservando la semántica de `Send` sin reintroducir lookups redundantes
  - `SendBatch` ya no sobrescribe `partial` con el primer tracking en fan-out multi-recipient; además el estado top-level `failed` queda reservado para batches con todos los items realmente fallidos, no parcialmente fallidos
  - se agregaron regressions tests para ambos bugs y se ejecutó un slice dirigido de `internal/service`; la evidencia real quedó actualizada en `verify-report.md`
  - Kuhn y Volta aprobaron el stream tras validar los fixes finales de semántica de locale y reporte `partial` en fan-out multi-recipient
