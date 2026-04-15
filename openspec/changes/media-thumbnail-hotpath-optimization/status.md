# Status

- state: planned
- percent: 0%
- dependency: security-perimeter-hardening closed
- worktree: `main`
- reviewer_final: Kuhn + Lorentz
- notes:
  - follow-up de performance extraído desde `perimeter-identity-default-deny` para que el hot path de `/public/video-thumbnail` no siga mezclado con policy de identidad
  - alcance: reuse seguro de transport/client y reducción de copias/buffers en thumbnails, manteniendo intactas las semánticas actuales de allowlist, SSRF guard, pinning, redaction y respuestas HTTP
  - este stream es performance-only; no reabre decisiones de auth ni el endurecimiento del perímetro ya aprobado
- DoD: hot path de thumbnails optimizado sin cambios observables de seguridad/semántica + verify focalizado
