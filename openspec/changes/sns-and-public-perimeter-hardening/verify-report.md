# Verify Report — SNS y perímetro público endurecidos

## Resultado

- **Estado actual**: `done`
- **Conclusión**: Las slices locales de config, wiring de app, SNS inbound y media pública quedaron verdes, la validación autónoma E2E final quedó verde y Lorentz aprobó el stream tras verificar el fix de redirect-hop allowlist.

## Evidencia ejecutada

Comandos verdes:

- `go test ./config ./internal/app ./internal/http/handler`
- `go test ./internal/http/handler -run 'TestHandleVideoThumbnail_(MissingURL|InvalidScheme|InvalidScheme_File|Success_JPEG|Success_PNG|YouTubeFallback|UpstreamError|RejectsHostNotInAllowlist|CacheHit|CacheExpiresAfterTTL|EvictsOldestEntryWhenCacheFull|SSRFBlocked|ConcurrentSameURL)$|TestSESWebhook_(InvalidTopicArn_Returns400|RejectsUnexpectedRegisteredTopicArnOrAccount|SubscriptionConfirmation)$'`
- `go test ./internal/http/handler -run '^TestHandleVideoThumbnail_' -count=1`
- `go test ./internal/app -run 'Test(NewSNSHTTPClient_UsesExplicitTimeout|BuildSESWebhookHandler_UsesConfiguredSNSBinding|BuildMediaHandler_UsesConfiguredAllowlist)$'`
- `SENDA_BASE_URL=http://localhost:8090 MAILPIT_URL=http://localhost:9025 SENDA_E2E_JWT_SECRET=e2e-test-jwt-secret-at-least-32-characters-long go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/ -run '^TestSESLifecycle01_HappyPath$'`
- `make test-e2e-ses`

## Qué se verificó

- La config rechaza HTTP en producción para `oidc.discovery_url` y `allowed_origins`, y bloquea `SENDA_SNS_SKIP_SIGNATURE_VERIFICATION` en prod.
- El webhook SNS acepta solo TopicArns registrados y sanitiza `SubscribeURL` en logs.
- El app wiring ya crea clientes SNS con timeout finito y pasa la policy al handler.
- `/public/video-thumbnail` ahora tiene allowlist de hosts, cache bounded con TTL/eviction, timeout explícito, allowlist aplicada en cada redirect hop y redacción de URLs en logs.

## Pendientes

- Ninguno bloqueante dentro del stream. Queda absorbido en el board global del ciclo 2.
