# Design — Taxonomía CI honesta y descubribilidad de tests frontend

## Objetivo técnico

Construir una superficie de validación que sea:

- **honesta**: solo promete lo que realmente existe
- **ejecutable**: cada nombre apunta a un contrato real
- **pequeña**: menos aliases, menos ambigüedad
- **descubrible**: el frontend tiene un entrypoint obvio para sus tests
- **autoverificable**: el repositorio puede detectar drift por sí mismo

## Taxonomía propuesta

| Categoría | Automática | Entrypoint canónico | Semántica |
| --- | --- | --- | --- |
| Backend PR | Sí | `make ci-backend-pr` | `lint + vet + swagger-check + test` |
| Frontend PR | Sí | `make ci-frontend` | valida frontend y usa `web/package.json:test` como entrypoint canónico |
| PR agregado | Sí | `make ci-pr` | backend PR + frontend PR |
| System PR | No | `make system-pr` | validación manual/observacional |
| System nightly | No | `make system-nightly` | validación manual/observacional ampliada |

### Regla central

No debe existir un nombre de gate que sugiera un flujo de `main` si no hay un workflow real, distinto y respaldado por CI.

Eso implica:

- `ci-main` no debe seguir publicándose como gate CI
- `ci-backend-main` no debe seguir publicándose como gate CI
- si alguna vez se necesita un flujo de main de verdad, tendrá que nacer con contrato propio, no como alias semánticamente falso

## Surface simplification

La simplificación debe ir en esta dirección:

1. **Consolidar el frontend**
   - `web/package.json:test` es el único punto canónico para tests frontend
   - `make ci-frontend` lo consume
   - si el runner necesita normalizar cwd o bootstrap, se resuelve detrás de ese script, no en el consumidor

2. **Eliminar la duplicación de `main`**
   - quitar la superficie pública de `ci-main` y `ci-backend-main`
   - reducir la ayuda de `make help` a comandos que tengan semántica real

3. **Separar automático de observacional**
   - los workflows `workflow_dispatch` deben estar documentados como manuales
   - `system-pr` y `system-nightly` no deben narrarse como bloqueos automáticos de PR

## Validación autónoma

Se debe añadir un check explícito, propuesto como `make ci-taxonomy-check`, respaldado por un script dedicado.

### Qué debe validar

- que `Makefile` expone solo la taxonomía acordada
- que `scripts/run-github-gates.sh` no exporta modos sin contrato público
- que `web/package.json` tiene `test`
- que `README.md`, `docs/DEVELOPMENT.md` y `docs/specs/TESTING_STRATEGY.md` describen la misma semántica
- que los workflows `pull_request` y `workflow_dispatch` no son narrados como si fueran lo mismo
- que no existen menciones de `ci-main` / `ci-backend-main` como gates activos si no hay un workflow real detrás

### Principio de implementación

El check debe comparar contratos declarados, no inferir intención desde prose suelta.

Eso evita falsos positivos y hace que el guardrail sea barato de mantener.

## Flujo operativo

1. Un contributor ejecuta `make ci-frontend` o `make ci-pr`.
2. El gate usa un entrypoint frontend único y descubible.
3. La documentación y la ayuda del repo describen la misma taxonomía.
4. `make ci-taxonomy-check` valida que no haya drift entre comandos, workflows y docs.

## Tradeoffs

### Mantener aliases de `main`

- **Pros**: menos cambios visibles para quien ya los usa.
- **Contras**: semántica engañosa, más superficie que mantener, peor onboarding.

### Eliminarlos de la superficie pública

- **Pros**: honestidad, menos sprawl, menos ambigüedad.
- **Contras**: obliga a asumir que el repo no tiene un main gate real hoy.

La recomendación es eliminar la ambigüedad, no maquillarla.

### Agregar un wrapper de tests frontend

- **Pros**: un solo entrypoint, más fácil de recordar y verificar.
- **Contras**: requiere fijar convenciones de ejecución para que funcione desde cualquier caller.

### No agregar validation autónoma

- **Pros**: menos trabajo inicial.
- **Contras**: el drift vuelve a aparecer y la taxonomía se degrada rápido.

La recomendación es agregarla.

## Criterio de éxito

Al terminar este cambio:

- cualquiera puede identificar qué es automático y qué es observacional sin leer entre líneas
- los tests frontend tienen un entrypoint canónico
- los aliases redundantes dejan de inflar la superficie pública
- existe un check autónomo que protege la taxonomía contra drift
- James puede revisar el cambio sin tener que reconstruir la semántica desde cero
