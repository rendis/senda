# ADR-0002: Transitional Email Fallback for Unbound OIDC Members

- **Status:** Accepted
- **Date:** 2026-04-15
- **Owners:** Backend + Security
- **Related:** closes `openspec/changes/perimeter-identity-default-deny`

## Context

The OIDC authentication handler (`internal/http/middleware/auth.go`) resolves a `Member` in two steps:

1. `memberStore.GetByOIDCIdentity(ctx, claims.Issuer, claims.Subject)` — primary lookup (`auth.go:115`).
2. On `NotFound`, `findUnboundMemberByEmail(ctx, memberStore, claims.Email)` — fallback by email (`auth.go:121`, implementation at `auth.go:139-153`).

This two-step flow exists because Senda currently supports two distinct identity-entry paths:

- **First-member onboarding** (`internal/service/onboarding.go:103-108`) persists `oidc_issuer` + `oidc_subject` at member creation, so the first login resolves via the primary lookup.
- **Invite flow** (`internal/http/handler/member.go:246-258`) creates invited members **without** OIDC identity. The invitee's first login cannot hit `GetByOIDCIdentity` and must rely on the email fallback to authenticate.

Removing the fallback unconditionally would break production: every invited member would be unable to authenticate on first login. The `perimeter-identity-default-deny` stream surfaced this as an open policy question — is this fallback an accepted transition, or must the system migrate to strict OIDC binding at invite time?

## Decision

Senda **accepts the email fallback as an explicit, documented transition** with the following invariants:

1. The fallback only applies to members whose `OIDCIssuer == nil` **and** `OIDCSubject == nil`. This is enforced in `findUnboundMemberByEmail` (`auth.go:148-150`): if either field is set, the fallback returns `ErrNotFound` instead of the member.
2. The first successful authentication via fallback is expected to persist `oidc_issuer + oidc_subject` on the member record, converting the member to **bound**. Subsequent logins resolve through the primary lookup and the fallback no longer applies to that member.
3. The fallback is **default-deny compatible**: it cannot promote an attacker over an already-bound member (the guard at `auth.go:148-150` blocks hijack attempts), and unknown emails still produce `403 FORBIDDEN` via `auth.go:123`.

This decision leaves runtime behavior unchanged and treats the fallback as a transitional seam, not a permanent feature.

## Consequences

**Positive**

- Invited members can authenticate on first login without a separate manual binding step.
- No production migration or data backfill is required.
- The existing test coverage (`internal/http/middleware/auth_test.go:386` — `TestAuth_OIDCFallsBackToEmailForUnboundInvitee`, and `auth_test.go:446` — `TestAuth_OIDCRejectsEmailFallbackForBoundMember`) already locks both sides of this policy.

**Negative**

- A residual transitional surface exists: an attacker who knows the email of an unbound invitee can attempt OIDC from any provider that asserts that email. Mitigation is operational — tenants are expected to scope OIDC providers appropriately, and the window closes as soon as the invitee logs in once.
- The policy lives in two files (`auth.go` + this ADR); future contributors must be aware that removing the fallback is a behavioral change requiring invite-flow redesign first.

**Neutral**

- The eventual path to strict binding (pre-seeding `oidc_issuer/subject` from the invite link when the tenant has a single trusted provider) is viable but **out of scope** for this ADR. If that work is pursued, it should open as a separate OpenSpec stream (`invite-flow-oidc-pre-binding` or similar) and can safely deprecate this fallback at that point.

## References

- `internal/http/middleware/auth.go:106-153` — fallback implementation.
- `internal/http/middleware/auth_test.go:386,446` — regression tests for both policy sides.
- `internal/http/handler/member.go:246-258` — invite flow that creates unbound members.
- `internal/service/onboarding.go:103-108` — first-member flow that pre-binds OIDC identity.
- `openspec/changes/perimeter-identity-default-deny/verify-report.md` — empirical evidence supporting this decision.
