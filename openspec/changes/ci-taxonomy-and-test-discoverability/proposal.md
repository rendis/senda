# Taxonomía CI honesta y descubribilidad de tests frontend

## Problema

La re-auditoría de maintainability/DX dejó un problema claro: la superficie actual de validación mezcla semánticas que no son equivalentes.

- `ci-main` y `ci-backend-main` no representan un contrato distinto y, en la práctica, duplican o disfrazan el path de PR.
- El repositorio tiene tests frontend reales, pero no un entrypoint canónico y fácil de descubrir.
- `Makefile` expone alias y wrappers redundantes que aumentan el sprawl.
- Los workflows manuales del sistema existen, pero la documentación todavía no distingue con suficiente honestidad qué es automático y qué es observacional/manual.
- No existe una validación autónoma que proteja esa taxonomía contra drift futuro.

El resultado es una UX de mantenimiento confusa: el nombre promete una cosa, pero el contrato operativo dice otra.

## Propuesta

Voy a reducir la superficie a una taxonomía pequeña, honesta y ejecutable:

1. **Gates automáticos de PR**
   - backend PR
   - frontend PR
   - PR agregado

2. **Validaciones manuales / observacionales**
   - system PR
   - system nightly

3. **Entrypoint canónico de tests frontend**
   - `web/package.json` debe exponer `test`
   - `make ci-frontend` debe consumir ese entrypoint, no comandos ad hoc dispersos

4. **Simplificación de aliases**
   - eliminar o dejar de publicar `ci-main` y `ci-backend-main` como gates
   - no mantener semánticas de `main` sin un workflow real que las respalde

5. **Validación autónoma**
   - agregar un check explícito de taxonomía que compare `Makefile`, scripts, workflows y documentación
   - el check debe fallar si un nombre, trigger o ayuda textual promete más de lo que existe

## Alcance

### En alcance

- `Makefile`
- `scripts/run-github-gates.sh`
- `web/package.json`
- `README.md`
- `docs/DEVELOPMENT.md`
- `docs/specs/TESTING_STRATEGY.md`
- `.github/workflows/*.yml`
- un check autónomo de consistencia para gates / comandos / docs

### Fuera de alcance

- lógica de producto
- backend funcional
- cambios de dominio
- ampliación del system gate con cobertura ficticia
- tocar el board global de OpenSpec

## Dependencias y paralelismo

### Dependencia mínima

- Este cambio se apoya en la rebaselínea previa de gates/documentación, porque parte de ese contrato ya fue estabilizado.

### Puede avanzar en paralelo con

- `send-core-rework`
- `surface-modularization-and-sdk-hardening`
- `security-perimeter-hardening`
- cualquier stream funcional que no cambie la semántica de los workflows del sistema

### Condición de coordinación

- Si otro stream sigue redefiniendo el comportamiento del system harness, este cambio debe respetar esa verdad final antes de fijar la redacción documental.

## Alternativas consideradas

1. **Mantener `ci-main` y `ci-backend-main` como “wrappers locales”**
   - Rechazado: sigue siendo sprawl y mantiene una semántica ambigua sin valor diferencial real.

2. **Crear un nuevo workflow de main para justificar los nombres**
   - Rechazado para este cambio: introduce costo operativo sin necesidad comprobada.

3. **Solo corregir documentación**
   - Rechazado: la documentación sola no cura un contrato operativo desalineado.

4. **Agregar solo el script `test` al frontend**
   - Insuficiente: mejora discoverability, pero no arregla la taxonomía ni el drift de aliases.

## Rollback

Si el cambio introduce fricción, el rollback debe ser directo:

- restaurar la superficie anterior de comandos
- retirar el check autónomo de taxonomía
- volver a la documentación previa
- mantener intacto el comportamiento de producto

## Reviewer final

James
