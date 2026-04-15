# Status

- state: in_progress
- percent: 55%
- dependency: cycle-2 closed
- worktree: `/.worktrees/spec-perimeter-identity-default-deny`
- reviewer_final: Lorentz
- notes:
  - ya quedó aplicado el binding OIDC por `sub+iss`, el origen público seguro para logout, `/metrics` obligatorio en prod, SNS default-deny cuando no hay binding configurado y probes SES/SNS no destructivos
  - pendiente: rematar hot path/pinning/redaction del thumbnail endpoint y luego correr el slice de verificación final
- DoD: perímetro default-deny + validación negativa representativa + signoff de seguridad
