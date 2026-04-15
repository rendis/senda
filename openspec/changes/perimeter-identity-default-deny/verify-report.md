# Verify Report

## Scope verified
- `internal/http/middleware/auth.go` now documents the real human-auth contract in code: `issuer + subject` lookup first, plus controlled email fallback only for members that are still unbound.
- `internal/http/middleware/auth_test.go` explicitly covers both sides of that policy: unbound invitees may fall back by email, while already-bound members reject that fallback.
- `internal/http/handler/member.go` still creates invited members without persisting `oidc_issuer` / `oidc_subject`, so removing the fallback in this batch would change production behavior.
- `internal/service/onboarding.go` still binds the first member at creation time, which confirms the current system intentionally supports two identity-entry paths.
- `config/config.go`, `internal/http/handler/provider_webhook.go`, and `internal/http/handler/media.go` already contain the runtime hardening this stream originally tracked: metrics token in production, SNS default-deny, and thumbnail pinning/redaction.
- Thumbnail hot-path performance work is now split to `media-thumbnail-hotpath-optimization`; this stream stays focused on the remaining identity-policy decision.

## Commands executed

1. `rg -n 'GetByOIDCIdentity|findUnboundMemberByEmail|TestAuth_OIDCFallsBackToEmailForUnboundInvitee|TestAuth_OIDCRejectsEmailFallbackForBoundMember|OIDCIssuer|OIDCSubject' internal/http/middleware/auth.go internal/http/middleware/auth_test.go internal/http/handler/member.go internal/service/onboarding.go`
   - result: PASS
   - purpose: verify the runtime still implements `issuer + subject` first, plus explicit unbound-email fallback, and that invite vs onboarding paths still differ in binding behavior.

2. `rg -n 'metrics_token|MetricsToken|SNS binding is not configured|TestSESWebhook_RejectsUnconfiguredSNSBinding|redactURL|ensurePinned|TestHandleVideoThumbnail_PinsResolvedDestinationAcrossRedirects|TestHandleVideoThumbnail_MixedDNSAnswersFailClosed' config/config.go config/config_test.go internal/http/handler/provider_webhook.go internal/http/handler/provider_webhook_test.go internal/http/handler/media.go internal/http/handler/media_test.go`
   - result: PASS
   - purpose: confirm the non-auth perimeter hardening already lives on `main` and should no longer be described as pending in this stream.

3. `go test ./internal/http/middleware -run 'TestAuth_OIDC(FallsBackToEmailForUnboundInvitee|RejectsEmailFallbackForBoundMember)$'`
   - result: PASS
   - purpose: prove the current runtime intentionally preserves the transitional fallback behavior.

4. `go test ./config -run 'TestLoad_ProductionRequiresMetricsToken$'`
   - result: PASS
   - purpose: re-validate the production-only metrics token guard that is already part of the absorbed perimeter hardening.

5. `go test ./internal/http/handler -run 'TestSESWebhook_RejectsUnconfiguredSNSBinding$|TestHandleVideoThumbnail_(PinsResolvedDestinationAcrossRedirects|MixedDNSAnswersFailClosed)$'`
   - result: PASS
   - purpose: confirm SNS default-deny and thumbnail pinning/fail-closed behavior remain covered while this batch only re-scopes documentation.

## Policy signoff

- Decision: [ADR-0002 — Transitional Email Fallback for Unbound OIDC Members](../../../docs/specs/ADR-0002-transitional-email-fallback-unbound-members.md) (Accepted 2026-04-15).
- Scope of decision: the email fallback for unbound members (`internal/http/middleware/auth.go:121` → `auth.go:139-153`) is accepted as an explicit, documented transition, conditioned on the anti-hijack guard at `auth.go:148-150`.
- Reviewer final: Lorentz.
- No runtime change: this closure is documentation-only. The tests listed above remain the authoritative behavioral lock.

## Final assessment
- state recommended: `done`
- reviewer_final: `Lorentz`
- reason: runtime hardening originally tracked by this stream was already on `main`. The residual policy decision — whether the unbound-email fallback is an accepted transition or must migrate to strict binding — is now resolved by ADR-0002 as an accepted transition with explicit invariants. A future migration to strict binding remains possible as a separate stream (`invite-flow-oidc-pre-binding` or similar) without reopening this one.
