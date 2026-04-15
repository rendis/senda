# Status

- state: in_progress
- percent: 85%
- dependency: cycle-2 closed
- worktree: `main`
- reviewer_final: Lorentz
- notes:
  - ya quedó absorbido en `main` el lookup OIDC por `issuer + subject`, el origen seguro para logout federado, `/metrics` obligatorio en producción, SNS default-deny cuando no hay binding configurado y el pinning/redaction del endpoint público de thumbnails
  - este batch reencuadra el stream y deja explícito que el runtime actual mantiene fallback controlado por email solo para members aún unbound; NO se cambia comportamiento productivo en este batch
  - el follow-up `media-thumbnail-hotpath-optimization` ya quedó cerrado sobre `main`; este stream ya no arrastra trabajo de media y el único remanente es el signoff de política sobre si el fallback de unbound members se acepta como transición o migra a un modelo futuro más estricto
- DoD: política de identidad explícita + docs del perímetro alineadas con el runtime real + signoff de seguridad
