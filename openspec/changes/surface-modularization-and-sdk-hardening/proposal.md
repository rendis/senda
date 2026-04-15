# Proposal: Modularización de superficies y endurecimiento del SDK

## Intent

Hoy `internal/app/app.go` concentra demasiada composición, los handlers mezclan responsabilidad de transporte con orquestación, y el SDK sigue reexportando demasiado de contratos internos. El objetivo es separar superficies reales y hacerlas explícitas: `management`, `data-plane`, `external-integration` y `public-sdk`.

## Scope

### In Scope
- Separar la composición por superficie y reducir la carga de `app.Bootstrap`.
- Adelgazar handlers para que deleguen en servicios/ports y no compongan infraestructura.
- Endurecer `sdk/` con seams explícitos, estables y versionables.
- Alinear documentación y contratos públicos con esa separación.

### Out of Scope
- Cambios de comportamiento funcional del producto.
- Nuevos endpoints o cambios de negocio no requeridos por la modularización.
- Reescritura de providers, persistencia o workers.
- Ajustes cosméticos de nombres sin cambio estructural real.

## Capabilities

### New Capabilities
- `architecture`: contratos y fronteras explícitas para superficies, bootstrap y SDK.

### Modified Capabilities
- None.

## Approach

- Convertir `app.Bootstrap` en composition root mínimo.
- Introducir ensambladores por superficie para `management`, `data-plane`, `external-integration` y `public-sdk`.
- Reubicar responsabilidades fuera de handlers y fuera del server monolítico.
- Reducir aliases públicos del SDK a seams deliberados y documentados.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/app/app.go` | Modified | Bootstrap deja de concentrar la totalidad del wiring. |
| `internal/http/server.go` | Modified | Registro de rutas y grupos por superficie más nítido. |
| `internal/http/handler/*` | Modified | Handlers más finos, menos dependencia directa de composición. |
| `sdk/*.go` | Modified | API pública más explícita y menos acoplada a internals. |
| `docs/ARCHITECTURE.md` / `docs/extensibility-guide.md` | Modified | Contratos y límites reflejados en documentación. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Romper rutas existentes al extraer ensambladores | Medium | Mantener cobertura de rutas por superficie y validar compatibilidad. |
| Filtrar contratos internos en el SDK por diseño incompleto | High | Revisar explícitamente qué tipos son públicos y cuáles quedan internos. |
| Coordinar con `send-core-rework` sin duplicar esfuerzo | Medium | Definir seams compatibles y no asumir refactors del core que aún no existen. |

## Rollback Plan

Si la modularización introduce fricción, revertimos a la composición actual sin cambiar el comportamiento de negocio. El rollback debe preservar el contrato de rutas, los flujos públicos y la compatibilidad del SDK mientras se corrigen las fronteras.

## Dependencies

- Coordinación con `send-core-rework` para no fijar contratos que ese cambio vaya a romper.
- Revisión del ownership de superficies antes de mover lógica compartida.
- Alineación con la dirección arquitectónica ya acordada para `management`, `data-plane`, `external-integration` y `public-sdk`.

## Success Criteria

- [ ] `app.Bootstrap` queda como composición raíz mínima, no como contenedor de toda la aplicación.
- [ ] Cada superficie tiene ownership claro y handlers más finos.
- [ ] El SDK expone seams explícitos y deja de reexportar internals innecesarios.
- [ ] La arquitectura resultante queda documentada y lista para implementación coordinada.
