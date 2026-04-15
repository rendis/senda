# Tasks — Amortización del send path y precisión de cache

## Fase 1. Seams de contexto y plan

- [x] 1.1 Definir `ResolvedSendContext` como snapshot inmutable del estado estable de envío.
- [x] 1.2 Definir `SendPlan` como unidad de ejecución por item, con solo las variaciones necesarias.
- [x] 1.3 Refactorizar `SendBatch` para resolver el contexto compartido una sola vez.
- [x] 1.4 Reemplazar la delegación item-by-item a `Send` por un flujo amortizado que derive planes por item.
- [x] 1.5 Cubrir con tests que el batch conserva cardinalidad, resultados y aislamiento entre items.

## Fase 2. Revocación set-based

- [x] 2.1 Diseñar el contrato de port para validar uso de grants revocados en conjunto.
- [x] 2.2 Reescribir `ReplaceAdapterWorkspaceAccess` para dejar de consultar revocación workspace por workspace.
- [x] 2.3 Reescribir `ReplaceIdentityWorkspaceAccess` con la misma estrategia set-based.
- [x] 2.4 Implementar la consulta PostgreSQL agregada y la cobertura de integración correspondiente.
- [x] 2.5 Verificar que la semántica de bloqueo por “grant en uso” se mantiene.

## Fase 3. Invalidación de cache precisa

- [x] 3.1 Revisar los call sites que hoy dependen de invalidación por prefijo.
- [x] 3.2 Introducir invalidación por claves exactas y scopes explícitos para las entradas afectadas.
- [x] 3.3 Reducir o encerrar `DeletePattern` para que no siga siendo la ruta caliente de invalidación.
- [x] 3.4 Agregar tests para demostrar que solo se invalidan las claves esperadas.
- [x] 3.5 Confirmar que el nuevo contrato no borra más cache del necesario.

## Fase 4. Benchmarks y validación autónoma

- [x] 4.1 Agregar benchmarks del contexto resuelto, del batch amortizado y de la revocación set-based.
- [x] 4.2 Medir la invalidación precisa frente al camino anterior por patrón.
- [x] 4.3 Ejecutar validación E2E autónoma sobre el flujo amortizado.
- [x] 4.4 Comparar las mediciones relevantes del hot path antes y después.
- [x] 4.5 Preparar el signoff final para Kuhn y Volta.

## Criterios de cierre

- [x] No queda `SendBatch` delegando item-by-item a la ruta completa.
- [x] No queda revocación de grants con N+1 por workspace.
- [x] No queda invalidación caliente basada en barrido por prefijo.
- [x] Existe evidencia medible del hot path y validación E2E autónoma.

## Follow-up de review (2026-04-11)

- [x] Cerrar la brecha de amortización cuando `Workspace.DefaultLocale` participa como locale efectiva.
- [x] Reemplazar el path N+1 de suppressions en `SendBatch` por una evaluación set-based única para TO/CC/BCC.
- [x] Reemplazar la invalidación de resolved templates por scope explícito en `CacheInvalidator` / `PGCache`.
- [x] Agregar benchmarks dedicados para `ResolvedSendContext` y revocación set-based.

## Blockers finales del re-review (2026-04-11 late)

- [x] Corregir el contexto compartido para que la locale default del workspace respete la semántica efectiva de `Send` cuando difiere del default de la versión.
- [x] Corregir el agregado de resultados batch para que los fan-outs multi-recipient con mezcla de éxito/fallo permanezcan `partial` y no escalen a `failed` salvo cuando TODOS los items fallan por completo.
