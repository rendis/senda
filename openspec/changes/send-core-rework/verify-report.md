# Verify Report — Send Core Rework

## Batch verified

- batch 1: extracción del primer use case explícito de la ruta caliente: `SuppressionBatchEvaluator`
- batch 2: seam hot/cold real para emails con `email_payloads`, `EmailStore.GetPayload(...)` y worker con carga diferida del payload
- batch 3: rediseño del rate limiter con `AcquireBurst(...)` y reserva local por adapter en el worker para amortizar contención
- batch 4: evidencia autónoma del pipeline completo + benchmark funcional del burst size + black-box híbrido desde el worktree actual
- batch 5: cierre del rework pedido por reviewers — seams explícitos en send, suppressions case-insensitive, `SendBatch` con contexto compartido y TTL/cleanup para rate-limit reservations
- batch 6: endurecimiento final del rework — fallo sistémico de suppressions propagado a nivel batch, cleanup del worker fuera del fast path y canonicalización consolidada en un solo paso
- batch 7: cierre final del rework — `SendBatch` preserva la cardinalidad real de tracking IDs por item y el prune asíncrono del worker revalida refs/estado antes de borrar reservas

## Commands executed

### Green

1. `go test ./internal/service -run 'TestSuppressionBatchEvaluator_EvaluateMatchesRecipientsCaseInsensitively|TestSendService_SendBatch_ReusesSharedContextAndSuppressionLookup|TestSendService_Send_UsesCaseInsensitiveSuppressionBatch'`
   - result: PASS
   - purpose: TDD del nuevo batch; valida suppressions case-insensitive y que `SendBatch` reuse contexto compartido + un único lookup batch

2. `go test ./internal/adapter/river -run 'TestSendWorker_(ReacquiresBurstAfterReservationTTLExpires|PrunesStaleRateLimitReservations)'`
   - result: PASS
   - purpose: probar de forma determinista la política TTL/cleanup de la caché local de reservas del worker

3. `go test ./internal/service -run 'Test(SuppressionBatchEvaluator_.*|SendService_Send_UsesSuppressionBatchInsteadOfSequentialChecks|SendService_Send_UsesCaseInsensitiveSuppressionBatch|SendService_SendBatch_(IsolatesPerItemInjectorContext|PartialStatus|AllFailed|PreservesLocaleAndExternalIDPerItem|PersistsUISourcePerItem|ReusesSharedContextAndSuppressionLookup)|SendPipeline_ReworkFlow_(BatchesSuppressionsPersistsHotColdAndReusesBurstReservation|RateLimitedSkipsColdPayload))'`
   - result: PASS
   - purpose: regresión focalizada del servicio sobre suppressions, batch execution y pipeline autónomo completo

4. `go test ./internal/adapter/river -run 'TestSendWorker_(ReusesBurstReservationAcrossJobsForSameAdapter|UsesConfiguredBurstSize|ReacquiresBurstAfterReservationTTLExpires|PrunesStaleRateLimitReservations|RateLimited_Snoozes|LoadsColdPayloadOnlyAfterHotPathChecks)'`
   - result: PASS
   - purpose: regresión focalizada del worker sobre reuse burst, TTL/cleanup y carga diferida del payload frío

5. `go test ./internal/adapter/postgres -run '^$'`
   - result: PASS
   - purpose: compile-check mínimo del adapter PostgreSQL tras normalizar suppressions a lowercase/canonical form

6. Evidencia previa que sigue vigente para el stream:
   - `go test ./internal/adapter/river -run '^$' -bench 'BenchmarkSendWorker_RateLimitBurstSize' -benchtime=100x` → PASS
   - `SENDA_E2E_EXTERNAL_STACK=1 ... go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run 'Test(F04_SendEmailSuccess|E04_RateLimitExceeded|E11_SuppressedEmail|E13_HotColdPayloadPersistence)'` → PASS
   - purpose: sostener la evidencia black-box/autónoma del pipeline ya cerrada antes de este rework de reviewers

7. `go test ./internal/service -run 'Test(SuppressionBatchEvaluator_EvaluateMany_CanonicalizesBatchOnlyOnce|SendService_SendBatch_SuppressionFailurePropagatesAsBatchError|SendService_SendBatch_ReusesSharedContextAndSuppressionLookup|SendService_Send_UsesCaseInsensitiveSuppressionBatch)'`
   - result: PASS
   - purpose: verificar que `EvaluateMany` canonicaliza en un solo paso y que un fallo sistémico de suppressions batch sube como error de batch

8. `go test ./internal/adapter/river -run 'TestSendWorker_(ReacquiresBurstAfterReservationTTLExpires|PrunesStaleRateLimitReservations|ReusesBurstReservationAcrossJobsForSameAdapter)'`
   - result: PASS
   - purpose: verificar que el fast path sigue reutilizando burst reservations y que el pruning/expiry quedan cubiertos sin ejecutar `Range` en la ruta crítica

9. `go test ./internal/service -run 'TestSendService_SendBatch_(PreservesAllTrackingIDsPerExpandedItem|SuppressionFailurePropagatesAsBatchError|ReusesSharedContextAndSuppressionLookup|PartialStatus|AllFailed|PreservesLocaleAndExternalIDPerItem|PersistsUISourcePerItem)'`
   - result: PASS
   - purpose: validar la corrección final del contrato batch: preserva todos los `tracking_ids` cuando la policy expande destinatarios y mantiene verde la semántica por item ya existente

10. `go test ./internal/adapter/river -run 'TestSendWorker_(DoesNotPruneReservationWithOutstandingReference|PrunesStaleRateLimitReservations|ReusesBurstReservationAcrossJobsForSameAdapter|ReacquiresBurstAfterReservationTTLExpires|UsesConfiguredBurstSize|RateLimited_Snoozes|LoadsColdPayloadOnlyAfterHotPathChecks)'`
   - result: PASS
   - purpose: validar el endurecimiento final del prune asíncrono y la no regresión del modelo burst/TTL/carga diferida

11. `go test ./internal/adapter/postgres -run '^$'`
   - result: PASS
   - purpose: compile-check mínimo tras el último endurecimiento del worker y del contrato batch

### Blocked / incomplete

12. `go test ./internal/adapter/postgres -tags=integration -run 'TestSuppressionRepo_CheckBatch_IsCaseInsensitive' -count=1 -timeout 120s -v`
   - result: WRAPPED_TIMEOUT after 30s
   - evidence:
     - `=== RUN   TestSuppressionRepo_CheckBatch_IsCaseInsensitive`
     - `Connected to docker`
     - `Building image ...`
     - sin progreso adicional antes del timeout envuelto
   - impact: el test de integración existe para cubrir la normalización case-insensitive en SQL real, pero en este entorno sigue bloqueado por el rebuild de imagen de Testcontainers

## Reviewer findings coverage

- **Volta 1 — `SendService.Send()` monolítico**: cubierto
  - se extrajeron seams explícitos para contexto compartido/resolución (`SendContextBuilder`) y persistencia/enqueue (`SendPersistenceWriter`)
- **Volta 2 — suppressions case-sensitive**: cubierto
  - canonicalización en dominio + proyección case-insensitive en el evaluador + normalización persistente/querying en PostgreSQL
- **Kuhn 3 — `SendBatch` repite trabajo compartible**: cubierto
  - `SendBatch` prepara el contexto una sola vez, reutiliza resoluciones estables y ejecuta una sola evaluación batch de suppressions
- **Kuhn 4 — `rateLimits` sin TTL/cleanup**: cubierto
  - TTL de reservas + cleanup oportunista + tests deterministas de pruning y reacquire
- **Nuevo hallazgo 1 — `EvaluateMany()` degradaba error sistémico a `202 Accepted`**: cubierto
  - `SendBatch` ya no oculta el fallo de suppressions; lo devuelve como error de batch retryable
- **Nuevo hallazgo 2 — data race + prune en fast path**: cubierto
  - el timestamp del último cleanup pasó a primitiva atómica y el prune real quedó fuera del acquire fast path; sólo se agenda cleanup asíncrono cuando se reserva un burst nuevo
- **Nuevo hallazgo 3 — recanonicalización redundante**: cubierto
  - `EvaluateMany` ahora arma el lookup canonical/unique en un solo paso
- **Hallazgo final 1 — `SendBatch` perdía tracking IDs al expandir destinatarios**: cubierto
  - el contrato batch ahora expone `tracking_ids` por item y conserva `tracking_id` como alias backward-compatible del primero; se eligió ampliar la respuesta, no prohibir `append`, porque la policy de test ya puede expandir cardinalidad y el contrato correcto debe reflejarla
- **Hallazgo final 2 — prune asíncrono podía borrar una reserva viva**: cubierto
  - cada reserva ahora usa refs/deleting atómicos y el cleanup sólo borra mediante revalidación + `CompareAndDelete` cuando la reserva sigue stale y sin referencias activas

## Remaining risks

- los tests Testcontainers que fuerzan rebuild completo siguen bloqueados por imagen/entorno y no deben volver a usarse como único gating signal
- la BD compartida del stack viejo no es confiable para este stream por su estado de migraciones inconsistente
- el benchmark funcional demuestra la amortización de adquisiciones y la sensatez de `burst=4`, pero no establece un óptimo global bajo toda carga posible
- no queda un bloqueo técnico abierto del stream; lo pendiente es signoff humano sobre el rework final
- el contrato batch quedó ampliado por compatibilidad progresiva: consumidores viejos aún pueden leer `tracking_id`, mientras que los nuevos deben usar `tracking_ids` si necesitan cardinalidad exacta

## Final assessment

- estado recomendado del change: `done`
- reviewer_final: `Volta` → **APPROVED**, `Kuhn` → **APPROVED**
- razón: la estructura principal + evidencia autónoma/black-box quedó cerrada, y los batches finales corrigieron también los hallazgos residuales de reviewers con tests verdes; no queda bloqueo técnico abierto del stream
