# Status

- state: done
- percent: 100%
- dependency: cycle-2 closed
- worktree: `main`
- reviewer_final: Lorentz
- closed_at: 2026-04-15
- signoff: [ADR-0002](../../../docs/specs/ADR-0002-transitional-email-fallback-unbound-members.md) — Transitional Email Fallback for Unbound OIDC Members (Accepted)
- notes:
  - runtime hardening ya estaba absorbido en `main` antes del cierre: lookup OIDC por `issuer + subject`, origen seguro para logout federado, `/metrics` obligatorio en producción, SNS default-deny sin binding configurado, y pinning/redaction del endpoint público de thumbnails
  - el fallback controlado por email para members aún unbound se acepta como transición explícita documentada en ADR-0002; el guard anti-hijack en `internal/http/middleware/auth.go:148-150` mantiene el perímetro default-deny para members bound
  - el follow-up `media-thumbnail-hotpath-optimization` cerró por separado; este stream no arrastra trabajo de media
  - cero cambios de código en este cierre — es documentación pura
- DoD cumplido:
  - [x] política de identidad explícita → ADR-0002
  - [x] docs del perímetro alineadas con el runtime real → `verify-report.md` + handoff cycle 3
  - [x] signoff de seguridad → ADR-0002 aceptado (reviewer final Lorentz)
