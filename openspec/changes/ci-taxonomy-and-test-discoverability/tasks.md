# Tasks — Taxonomía CI honesta y descubribilidad de tests frontend

## Fase 1 — Baseline y contratos

- [x] 1.1 Inventariar la superficie actual de comandos, workflows y docs que menciona gates de CI.
- [x] 1.2 Confirmar qué nombres representan gates reales y cuáles son alias redundantes o engañosos.
- [x] 1.3 Cerrar la decisión de taxonomía: automático vs manual/observacional vs local-convenience.
- [x] 1.4 Dejar explícita la dependencia de rebaselínea previa para no reabrir semánticas ya estabilizadas.

## Fase 2 — Simplificación de alias y comandos

- [x] 2.1 Retirar `ci-main` y `ci-backend-main` de la superficie pública si no existe un workflow distinto que los respalde.
- [x] 2.2 Reducir `Makefile` y `scripts/run-github-gates.sh` a la taxonomía mínima acordada.
- [x] 2.3 Asegurar que `ci-pr` compone únicamente validaciones automáticas reales.
- [x] 2.4 Ajustar la ayuda/comentarios de comandos para que no inventen semántica.

## Fase 3 — Descubribilidad de tests frontend

- [x] 3.1 Añadir `test` en `web/package.json` como entrypoint canónico.
- [x] 3.2 Hacer que `make ci-frontend` consuma ese entrypoint en lugar de depender de comandos ad hoc.
- [x] 3.3 Verificar que el runner sea estable tanto desde `web/` como desde el repo raíz vía `pnpm --dir web test`.
- [x] 3.4 Documentar el comando frontend más visible en `README.md` y en la guía de desarrollo.

## Fase 4 — Honestidad documental

- [x] 4.1 Reescribir `README.md` para reflejar la taxonomía real.
- [x] 4.2 Reescribir `docs/DEVELOPMENT.md` para alinear validaciones, aliases y workflows.
- [x] 4.3 Reescribir `docs/specs/TESTING_STRATEGY.md` para separar automático de manual/observacional.
- [x] 4.4 Asegurar que cualquier workflow `workflow_dispatch` queda etiquetado como manual.

## Fase 5 — Validación autónoma

- [x] 5.1 Crear `make ci-taxonomy-check` o un target equivalente con un script dedicado.
- [x] 5.2 Validar que la taxonomía declarada en Makefile, scripts, workflows y docs coincide.
- [x] 5.3 Fallar si existe drift: alias no respaldados, documentación mintiendo o entrypoints ausentes.
- [x] 5.4 Incluir el check en el flujo PR donde corresponda.

## Fase 6 — Cierre y review

- [x] 6.1 Revisar la superficie final buscando duplicación residual.
- [x] 6.2 Confirmar que el resultado es entendible sin conocimiento previo del repo.
- [x] 6.3 Preparar el cambio para revisión final por James.
- [x] 6.4 Dejar nota clara de rollback si se necesita restaurar la superficie anterior.

## Evidencia final

- `make ci-taxonomy-check` ✅
- `make ci-pr` ✅
- Warning del runner frontend: resuelto por código (`web/package.json` + `web/tests/test-root.mjs`), no quedó pendiente de documentación
