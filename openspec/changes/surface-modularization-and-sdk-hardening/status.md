# Status

- state: done
- percent: 100%
- dependency: cleared (send-core-rework done)
- worktree: `/.worktrees/spec-surface-modularization-and-sdk-hardening`
- reviewer_final: Volta
- notes: surface modularization is complete — external-integration session routing is owned by the external surface without builder handlers, runtime seams are explicit inside the handler boundary, the SDK no longer leaks `internal/domain.Environment` in its public contracts, the autonomous E2E harness passes on the aligned migration baseline, and Volta granted the final architecture signoff.
- DoD: E2E autónomo exitoso + signoff de arquitectura
