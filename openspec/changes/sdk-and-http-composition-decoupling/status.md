# Status

- state: planned
- percent: 0%
- dependency: cycle-2 closed
- worktree: `main`
- reviewer_final: Volta
- notes:
  - ciclo 3 de arquitectura para desacoplar el SDK del dominio interno y reducir el composer/wiring monolítico
  - incluye enums/contratos estables en `sdk`, composer por superficie y reducción del server mutable basado en nil-checks
- DoD: boundary SDK estable + composición HTTP explícita + signoff de arquitectura
