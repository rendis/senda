# ADR-0001: Provider-Managed Email Authentication

- **Status:** Accepted
- **Date:** 2026-02-25
- **Owners:** Backend + QA

## Context

Había contradicción entre documentos y código respecto a autenticación de email:
- Parte de la especificación describía DKIM/SPF/DMARC implementado en app (tablas `domains`, signing DKIM propio).
- El backend actual y README operan sobre modelo provider-managed (SES/Gmail).

Esto generaba ambigüedad funcional para R-13/R-14 y para el gate de QA.

## Decision

Senda adopta **provider-managed email auth** como única fuente de verdad para P0:

1. SPF/DKIM/DMARC son responsabilidad del provider (SES/Gmail).
2. Senda **no** firma DKIM ni genera registros DNS desde la app.
3. Senda valida capacidad de envío mediante identidades del provider (sync + default identity).
4. Flujos/artefactos de `domains` y DKIM in-app quedan deprecados para P0.

## Consequences

- `POST /send` valida identidad efectiva del adapter, no dominio DKIM local.
- Sync de identidades SES/Gmail es parte del flujo funcional requerido.
- Bloques de PRD/TECH_SPEC sobre domain verification in-app se marcan como históricos/deprecated.
- QA usa este ADR como criterio de interpretación para R-13/R-14.
