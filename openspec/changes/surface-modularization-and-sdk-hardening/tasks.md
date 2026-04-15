# Tasks: Modularización de superficies y endurecimiento del SDK

## Phase 1: Contratos y partición estructural

- [x] 1.1 Inventariar dependencias actuales en `internal/app/app.go`, `internal/http/server.go` y `sdk/*.go` para separar ownership por superficie.
- [x] 1.2 Definir ensambladores/constructores por superficie sin mover comportamiento todavía.
- [x] 1.3 Alinear el contrato público mínimo del SDK con los seams realmente soportados.

## Phase 2: Refactor de bootstrap y handlers

- [x] 2.1 Extraer del bootstrap la composición específica de `management` y `data-plane`.
- [x] 2.2 Separar el wiring de `external-integration` en un ensamblador propio.
- [x] 2.3 Adelgazar handlers en `internal/http/handler/*.go` para que solo adapten request → servicio.
- [x] 2.4 Reorganizar `internal/http/server.go` para que cada grupo de rutas refleje ownership explícito.

## Phase 3: Endurecimiento del SDK

- [x] 3.1 Reducir reexportaciones en `sdk/interfaces.go` y `sdk/types.go` a lo estrictamente público.
- [x] 3.2 Ajustar `sdk/engine.go` para depender de seams explícitos y no de internals innecesarios.
- [x] 3.3 Actualizar la guía pública en `docs/extensibility-guide.md` con el contrato endurecido.

## Phase 4: Verificación y E2E

- [x] 4.1 Agregar/verificar tests unitarios para bootstrap mínimo, handlers finos y superficie SDK.
- [x] 4.2 Verificar rutas de management y data-plane con cobertura de casos representativos.
- [x] 4.3 Verificar el flujo de external integration con seams explícitos y permisos preservados.
- [x] 4.4 Correr E2E autónomo contra management, data-plane, external integration y uso del SDK.

## Phase 5: Cierre

- [x] 5.1 Revisar consistencia arquitectónica final contra la spec.
- [x] 5.2 Documentar tradeoffs residuales y puntos que queden para `send-core-rework`.
