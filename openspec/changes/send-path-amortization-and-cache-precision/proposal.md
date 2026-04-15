# Amortización del send path y precisión de cache

## Problema

La re-auditoría ciega volvió a confirmar que la ruta de envío está pagando trabajo repetido en el punto más caro del sistema:

- `SendBatch` ejecuta la tubería completa por cada item, aunque varias resoluciones son estables dentro del mismo lote.
- La revocación de accesos en `ReplaceAdapterWorkspaceAccess` y `ReplaceIdentityWorkspaceAccess` recorre workspace por workspace y termina en un patrón N+1.
- La invalidación de cache por prefijo usa un `DELETE ... WHERE key LIKE ...` demasiado amplio para un hot path que necesita precisión.
- Falta un benchmark claro y comparable del hot path real; hoy no hay una medición estructural que permita defender una amortización o una regresión.

El problema no es de micro-optimización. Es de arquitectura de ejecución: estamos re-ejecutando trabajo estable, interrogando la base más veces de las necesarias y borrando cache con más alcance del que el cambio realmente requiere.

## Intención de la solución

La solución debe ser estructural y reutilizable:

1. Introducir un `ResolvedSendContext` inmutable que concentre todo lo estable de una resolución de envío.
2. Derivar un `SendPlan` por item lógico, con solo las diferencias variables que realmente cambian entre envíos.
3. Reutilizar ese contexto/plan en `SendBatch` para amortizar resolución, renderizado estable y preparación del hot path.
4. Reescribir la revocación de accesos como operación set-based, con una consulta que valide el conjunto completo de bajas en una sola ida a PostgreSQL.
5. Sustituir la invalidación por prefijo por invalidación precisa basada en claves concretas y scopes explícitos.
6. Añadir benchmarks y validación E2E autónoma para demostrar que la mejora existe y no rompe semántica.

## Alcance

### En alcance

- Refactor del pipeline de envío para reutilizar contexto estable.
- Extracción de `ResolvedSendContext` / `SendPlan` o equivalente funcional.
- Eliminación del N+1 en revocación de grants.
- Endurecimiento de la invalidación de cache para que sea exacta o scope-aware.
- Benchmarks del hot path y pruebas E2E autónomas.

### Fuera de alcance

- Cambiar el contrato público del API de envío.
- Reescribir el modelo de dominio del email fuera de lo necesario para la amortización.
- Introducir un nuevo backend de cache o una capa de infraestructura externa.
- Cambiar reglas funcionales de negocio que no estén relacionadas con el hot path.

## Dependencias y paralelismo ciclo 2

### Dependencias

- `autonomous-e2e-isolation` para obtener un harness autónomo e isolado que sirva como evidencia E2E confiable.
- La estructura actual del send path y de los grants en PostgreSQL, porque este cambio se apoya en los seams existentes para transformarlos.

### Paralelismo

- Puede avanzar en paralelo con streams de documentación, QA y hardening que no toquen la ruta caliente de envío.
- Debe coordinarse con cualquier otro stream de ciclo 2 que modifique `internal/service/send.go`, `internal/service/adapter_access.go`, `internal/adapter/pgcache/client.go` o el worker de envío.
- Si aparece un cambio competidor en los mismos seams, este stream debe ir primero o definirse una frontera clara; no se deben mezclar dos refactors estructurales sobre la misma ruta crítica sin orden.

## Alternativas consideradas

1. **Seguir con helpers internos dentro de `SendService`.**
   - Ventaja: menor costo inicial.
   - Desventaja: el trabajo estable sigue repitiéndose y el problema queda escondido.

2. **Crear una fachada nueva pero dejar el monolito debajo.**
   - Ventaja: parece modular.
   - Desventaja: duplica conceptos y no baja el costo real del hot path.

3. **Introducir `ResolvedSendContext` / `SendPlan` y rediseñar los seams de acceso y cache.**
   - Ventaja: corrige estructura, medible y con límites claros.
   - Desventaja: requiere más trabajo y tests.

La opción 3 es la correcta. Aquí no buscamos cosmética; buscamos una base defendible.

## Riesgos

- **Regresión funcional en batch:** si el contexto compartido captura algo que en realidad es por-item, se puede mezclar estado entre mensajes.
- **Regresión de permisos:** una revocación set-based mal diseñada puede pasar por alto un workspace en uso o bloquear más de lo debido.
- **Regresión operativa de cache:** una invalidación demasiado estrecha puede dejar entradas obsoletas vivas; una demasiado amplia nos devuelve al mismo problema.
- **Benchmark engañoso:** si la medición no separa contexto, grants, cache y persistencia, no va a demostrar amortización real.

## Rollback

El rollback debe ser explícito y completo:

- volver a la ruta anterior de `SendBatch` si el nuevo plan introduce regresión,
- restaurar la revocación por iteración si la versión set-based no preserva semántica,
- reactivar la invalidación anterior solo como medida temporal si la precisión nueva falla,
- conservar los benchmarks como evidencia, aunque se revierta la implementación.

No se acepta dejar un sistema parcialmente refactorizado con la ilusión de que “luego se corrige”. Si falla la base, se revierte la base.

## Reviewer final

Kuhn + Volta
