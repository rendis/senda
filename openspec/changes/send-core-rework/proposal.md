# Send Core Rework

## Problema

El flujo actual de envío vive concentrado en `internal/service/send.go` y mezcla demasiadas responsabilidades en una sola ruta crítica: resolución de tenant/workspace/template, validación, política de test recipients, supresión, resolución de from identity, snapshots de variables/injectors, persistencia y enqueue.

La evidencia del repositorio confirma el acoplamiento:

- `SendService.Send()` ejecuta la cadena completa de orquestación y tiene complejidad alta.
- La supresión se evalúa de forma secuencial por destinatario, lo que convierte el hot path en un patrón N llamadas por N recipients.
- `take_send_token()` usa `FOR UPDATE` sobre `token_buckets`, y la cola de envío corre con hasta 30 workers; eso eleva la contención por adapter en el punto más caliente del sistema.
- La tabla `emails` guarda snapshots y payload amplio en la fila operativa, aunque parte de esa información solo se necesita fuera del hot path o en el worker.

El resultado es un core difícil de probar, difícil de perfilar y demasiado caro en la ruta de aceptación.

## Solution intent

Rehacer el core de envío alrededor de límites explícitos y use cases pequeños, manteniendo el comportamiento funcional pero rompiendo la monolítica interna.

La dirección propuesta es:

1. Separar el envío en use cases explícitos: resolución del contexto, evaluación de supresión, persistencia operativa y enqueue.
2. Evaluar supresión en batch por workspace/destinatarios, no destinatario por destinatario.
3. Reducir la contención del rate limit por adapter con una API de reserva/acceso menos serializante que el camino actual por mensaje.
4. Dividir la fila operativa en datos calientes mínimos y payload frío diferido.

No buscamos un parche. Buscamos una reestructuración interna real del core.

## Alcance

### En alcance

- Reestructuración interna de `SendService` hacia use cases y puertos explícitos.
- Cambio del modelo de evaluación de supresión a batch.
- Ajustes al rate limiting para disminuir contención por adapter.
- Separación hot/cold de la persistencia de `emails`.
- Tests unitarios, integración y bench funcional para validar el nuevo core.

### Fuera de alcance

- Cambios al contrato público del API de envío.
- Cambios en frontend, dashboard o cache salvo que un benchmark demuestre impacto real en la ruta caliente.
- Reglas de negocio de proveedor, webhooks o ingesta de eventos, salvo la adaptación necesaria al nuevo modelo de persistencia.
- Compatibilidad retroactiva interna: este cambio puede rehacer estructuras sin mantener una capa de compatibilidad interna paralela.

## Riesgos

- **Riesgo de regresión funcional:** la ruta de envío toca validación, supresión y persistencia atómica. Si se parte mal, el sistema puede aceptar o rechazar mensajes incorrectamente.
- **Riesgo de migración de datos:** separar hot/cold implica mover columnas y puede afectar queries de lectura si no se rediseñan bien.
- **Riesgo de contención residual:** un rate limit mal rediseñado puede reducir la contención pero romper el cumplimiento real del límite por adapter.
- **Riesgo de sobre-diseño:** partir en demasiados fragmentos sin una frontera clara puede degradar mantenibilidad en lugar de mejorarla.

## Dependencias

- `autonomous-e2e-isolation` para validar el cambio con E2E autónomo y ambientes aislados.
- Migraciones de PostgreSQL para el split hot/cold y el nuevo contrato de persistencia.
- La infraestructura de River y los workers de envío, porque el worker consume la fila operativa y debe seguir siendo idempotente.

## Alternatives considered

1. **Seguir con un service monolítico y extraer helpers internos.**
   - Ventaja: menor costo inicial.
   - Desventaja: no corrige la falta de límites ni la contención del hot path.

2. **Crear facades delgadas alrededor del monolito.**
   - Ventaja: parece modular sin mover mucho código.
   - Desventaja: la complejidad solo queda escondida; el núcleo sigue ancho y acoplado.

3. **Rehacer el core con use cases y puertos explícitos.**
   - Ventaja: permite atacar la arquitectura y el performance juntos.
   - Costo: más trabajo inicial y migración interna más invasiva.

La opción 3 es la correcta para este caso.

## Rollback

Si la reestructuración introduce regresión, el rollback debe ser explícito y completo:

- revertir la migración de código del core de envío,
- restaurar la ruta anterior de persistencia y enqueue,
- revertir la migración de esquema asociada si todavía no depende de datos irreversibles,
- volver a la implementación anterior de rate limiting y supresión.

No se aceptan parches superficiales sobre una base quebrada. Si el diseño nuevo falla, se revierte el bloque completo y se reabre con evidencia.

## Reviewer final

Volta + Kuhn
