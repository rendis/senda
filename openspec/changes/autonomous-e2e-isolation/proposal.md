# Autonomous E2E Isolation

## Problema

Hoy la operabilidad de E2E/system depende de nombres fijos y recursos compartidos. Eso hace que dos stacks ejecutándose en paralelo compitan por la misma red, contenedores, reportes y rutas de artefactos. La evidencia del repo confirma el acoplamiento: `test/e2e/harness_test.go` usa `senda-e2e-*`, `internal/teststack/stack.go` usa prefijos fijos `senda-stack-pr-*`/`nightly-*`, y `test/system/system-runner.sh` es el orquestador base con artefactos y cleanup centralizados.

## Solución propuesta

Parametrizar el modelo de identidad de E2E/system para que cada spec/worktree/run produzca nombres únicos y rastreables para red, contenedores, reportes y directorios de artefactos. El camino determinístico por defecto debe ejecutar solo los stages base; visual/a11y/chaos quedan fuera y se activan explícitamente. El contrato debe ser artifacts-first: crear artefactos antes de ejecutar, registrar resultados por etapa, y garantizar cleanup aun cuando falle una fase.

## Alcance

### En alcance

- Aislamiento de redes, contenedores y reportes de E2E/system.
- Parametrización por spec/worktree/run.
- Contrato explícito de artefactos por etapa.
- Smoke final con dos stacks simultáneos.
- Limpieza garantizada al terminar.

### Fuera de alcance

- Cambios en comportamiento de producto.
- Integraciones externas o providers.
- Reescritura del runner funcional completo.
- Hacer heavy stages parte del camino determinístico por defecto.

## Dependencias

- Docker/Testcontainers disponibles.
- El orquestador base `test/system/system-runner.sh`.
- El harness E2E actual y el stack testable existente.

## Alternativas consideradas

1. **Mantener nombres fijos y serializar ejecuciones.** Más simple, pero destruye la autonomía y no resuelve colisiones.
2. **Usar UUID aleatorios en todo.** Evita colisiones, pero complica depuración, limpieza y trazabilidad.
3. **Namespace solo por suite.** Reduce choque parcial, pero sigue fallando con worktrees/specs concurrentes.

La alternativa elegida es un namespace estable + sufijo único por run, porque equilibra trazabilidad, limpieza y paralelismo real.

## Rollback

Si la parametrización introduce inestabilidad, se puede volver temporalmente al naming fijo mediante la ruta legacy del runner/harness mientras se corrigen los puntos de colisión. El rollback debe preservar el contrato de cleanup y artefactos.

## Reviewer final

James
