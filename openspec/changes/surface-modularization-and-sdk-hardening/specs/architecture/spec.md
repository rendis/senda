# Architecture Specification

## Purpose

Definir fronteras estructurales para `management`, `data-plane`, `external-integration` y `public-sdk`, de modo que la composición, los handlers y el SDK queden alineados con ownership explícito.

## Requirements

### Requirement: Bounded Surfaces
El sistema MUST mantener las superficies `management`, `data-plane`, `external-integration` y `public-sdk` como contratos separados y reconocibles.

#### Scenario: Registration by surface
- GIVEN una inicialización estándar de la aplicación
- WHEN se construye el árbol de rutas y extensiones
- THEN cada superficie queda registrada por separado
- AND una superficie no expone rutas o seams de otra por accidente

#### Scenario: Disabled surface
- GIVEN una superficie que no recibe dependencias opcionales
- WHEN se compone la aplicación
- THEN esa superficie no se registra
- AND las demás superficies continúan funcionando

### Requirement: Thin Handlers
Los handlers MUST limitarse a parseo de request, extracción de contexto, validación local y delegación a servicios o ports.

#### Scenario: Management request delegation
- GIVEN una petición de management
- WHEN el handler procesa la solicitud
- THEN el handler delega la lógica de negocio a servicios o ports
- AND no construye infraestructura ni compone dependencias cruzadas

#### Scenario: Data-plane request scope
- GIVEN una petición de data-plane autenticada por API key
- WHEN el handler resuelve el workspace y ejecuta la acción
- THEN la decisión depende del contexto y del servicio
- AND no depende de wiring oculto en el handler

### Requirement: Minimal Bootstrap
`app.Bootstrap` MUST actuar como composition root mínimo y MUST delegar la composición concreta a ensambladores por superficie.

#### Scenario: Full application startup
- GIVEN configuración válida y extensiones opcionales
- WHEN la aplicación arranca
- THEN el bootstrap crea dependencias compartidas
- AND delega el wiring de cada superficie a su ensamblador correspondiente

#### Scenario: Extensionless startup
- GIVEN que no hay extensiones del SDK
- WHEN la aplicación arranca
- THEN el bootstrap sigue siendo válido
- AND la ausencia de extensiones no cambia la estructura de superficies

### Requirement: Explicit Versionable SDK
El paquete `sdk` MUST exponer seams públicos explícitos, estables y documentados, y MUST NOT reexportar tipos internos que no formen parte del contrato público.

#### Scenario: Embeddable integration
- GIVEN un proyecto consumidor del SDK
- WHEN registra injectors, init, auth o resolvers
- THEN usa únicamente tipos públicos del SDK
- AND puede hacerlo sin importar internals del core

#### Scenario: Internal contract change
- GIVEN un cambio en tipos internos de `internal/port` o `internal/app`
- WHEN el contrato público no cambia
- THEN el consumidor del SDK no debe romperse
- AND cualquier ruptura requiere versionado o cambio deliberado del contrato público
