# Status

- state: planned
- percent: 0%
- dependency: cycle-2 closed
- worktree: `/.worktrees/spec-send-context-and-media-hotpath`
- reviewer_final: Kuhn + Volta
- notes:
  - ciclo 3 de performance para converger el contexto de envío a una sola fuente de verdad y optimizar el hot path de media
  - incluye unificación de `SendContextBuilder`/`SendService`, cache de miniaturas sin copia en hit, y reuse de transport/client con keep-alive
- DoD: hot path convergente + benchmarks/hit-path optimizado + doble signoff
