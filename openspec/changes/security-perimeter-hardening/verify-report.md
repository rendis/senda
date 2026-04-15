# Verify Report — security-perimeter-hardening

## Scope verified

- SNS inbound exact `TopicArn` binding and account-ID binding
- SNS replay/dedup persistence keyed by `TopicArn + MessageId`
- External integration token transport restricted to `x-senda-external-token`
- Webhook outbound redirect handling
- Media public allowlist seam, deny-by-default posture, and destination pinning/revalidation

## Commands executed

```bash
cd /Users/rendis/Documents/Projects/Libraries/senda/.worktrees/spec-security-perimeter-hardening && go test ./internal/http/handler -run '^TestHandleVideoThumbnail_(SSRFBlocked|SSRFSpecialPurposeIPv4Blocked|MixedDNSAnswersFailClosed|RedirectToPrivateDestinationRejected|PinsResolvedDestinationAcrossRedirects)$' -count=1
cd /Users/rendis/Documents/Projects/Libraries/senda/.worktrees/spec-security-perimeter-hardening && SENDA_INTEGRATION_DATABASE_URL='postgres://senda:senda@127.0.0.1:56359/senda?sslmode=disable' go test -tags integration ./internal/adapter/postgres -run '^TestSNSReplayRepo_' -count=1 -timeout 10m
cd /Users/rendis/Documents/Projects/Libraries/senda/.worktrees/spec-security-perimeter-hardening && SENDA_E2E_EXTERNAL_STACK=1 SENDA_BASE_URL='http://127.0.0.1:8080' MAILPIT_URL='http://127.0.0.1:8025' SENDA_DATABASE_URL='postgres://senda:senda@127.0.0.1:56359/senda?sslmode=disable' go test -tags e2e ./test/e2e -run '^TestSecurityPerimeterHardening01_AutonomousFlow$' -count=1 -timeout 15m
cd /Users/rendis/Documents/Projects/Libraries/senda/.worktrees/spec-security-perimeter-hardening && go test ./internal/adapter/river -run '^TestWebhookWorker_RedirectResponse_DoesNotFollowAndCancelsJob$' -count=1
```

## Results

- `go test ./internal/http/handler -run '^TestHandleVideoThumbnail_(SSRFBlocked|SSRFSpecialPurposeIPv4Blocked|MixedDNSAnswersFailClosed|RedirectToPrivateDestinationRejected|PinsResolvedDestinationAcrossRedirects)$' -count=1` ✅
- `SENDA_INTEGRATION_DATABASE_URL=... go test -tags integration ./internal/adapter/postgres -run '^TestSNSReplayRepo_' -count=1 -timeout 10m` ✅
- `SENDA_E2E_EXTERNAL_STACK=1 ... go test -tags e2e ./test/e2e -run '^TestSecurityPerimeterHardening01_AutonomousFlow$' -count=1 -timeout 15m` ✅
- `go test ./internal/adapter/river -run '^TestWebhookWorker_RedirectResponse_DoesNotFollowAndCancelsJob$' -count=1` ✅

Reviewer final: `Lorentz` → **APPROVED**

## Notes

- SNS replay/dedup persistence is now implemented in PostgreSQL with durable claim semantics and stale-window rejection; the integration verification was executed against a live Postgres connection via `SENDA_INTEGRATION_DATABASE_URL` to avoid the long Testcontainers image bootstrap.
- The autonomous E2E flow covers SNS topic/account rejection, duplicate/stale replay rejection, query-token rejection, and media pinning/rebinding/redirect denial without relying on the full Docker app bootstrap.
- Mixed DNS answers for media pinning now fail closed; a public-first answer set with any private, reserved IPv6, or IPv4 special-purpose address is rejected before any request is issued.
- Webhook outbound redirect denial is covered by the targeted worker regression test.
- No build was run, per stream constraints.
- Final security signoff confirmed that IPv4 special-purpose ranges `0.0.0.0/8` and `192.0.2.0/24` are now blocked explicitly and covered by unit + E2E evidence.
