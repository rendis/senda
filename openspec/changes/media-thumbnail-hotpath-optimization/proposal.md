# Proposal

## Why
`/public/video-thumbnail` is already hardened, but its hot path still does avoidable work per request:
- a fresh fetch session/client path is built for each request
- image download/composition performs full-buffer reads and extra copies
- the remaining work is about throughput and allocation pressure, not security policy

Keeping that inside `perimeter-identity-default-deny` mixes two different concerns and increases the risk of changing auth policy while only chasing performance.

## Scope
- optimize the thumbnail fetch/composition path for lower churn and fewer copies
- keep current security semantics exactly as they are today:
  - allowlist / SSRF guard
  - destination pinning and fail-closed DNS handling
  - URL redaction
  - cache behavior and HTTP response contract

## Non-goals
- no auth/identity changes
- no SNS/webhook changes
- no behavior changes to thumbnail validation or security posture
- no broad media-handler redesign beyond the hot path

## Ownership note
This spec intentionally owns the media dimension that was previously described inside `perimeter-identity-default-deny`. That perimeter stream now stays focused on the remaining identity-policy decision for unbound-member fallback.
