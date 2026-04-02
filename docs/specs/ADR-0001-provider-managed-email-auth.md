# ADR-0001: Provider-Managed Email Authentication

- **Status:** Accepted
- **Date:** 2026-02-25
- **Owners:** Backend + QA

## Context

There was a contradiction between the documentation and the code regarding email authentication:
- Part of the specification described DKIM/SPF/DMARC implemented in the app (custom `domains` tables, in-app DKIM signing).
- The current backend and README follow a provider-managed model (SES/Gmail).

This created functional ambiguity for R-13/R-14 and for the QA gate.

## Decision

Senda adopts **provider-managed email auth** as the single source of truth for P0:

1. SPF/DKIM/DMARC are the provider's responsibility (SES/Gmail).
2. Senda **does not** sign DKIM or generate DNS records from the app.
3. Senda validates sending capability through provider identities (sync + default identity).
4. The `domains` flows/artifacts and in-app DKIM are deprecated for P0.

## Consequences

- `POST /send` validates the effective identity of the adapter, not a local DKIM domain.
- SES/Gmail identity sync is part of the required functional flow.
- PRD/TECH_SPEC blocks about in-app domain verification are marked as historical/deprecated.
- QA uses this ADR as the interpretation criterion for R-13/R-14.
