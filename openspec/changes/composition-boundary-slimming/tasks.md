# Tasks: composition-boundary-slimming

## Phase 1: Definir las fronteras

- [x] 1.1 Fijar el mapa de composición: shared infra, surface wiring, router registrars y boundary SES provider-specific.
- [x] 1.2 Crear los helpers base en `internal/app/bootstrap.go` para que `internal/app/app.go` deje de concentrar el wiring completo.
- [x] 1.3 Definir la API interna del boundary SES (`internal/adapter/ses/webhook`) antes de mover parsing y mapping.
- [x] 1.4 Ajustar `internal/http/server_test.go` para capturar el contrato de rutas actual antes de partir el switchboard.

## Phase 2: Slimming real del composition root

- [x] 2.1 Mover la creación de dependencias compartidas fuera de `internal/app/app.go` hacia helpers explícitos.
- [x] 2.2 Separar la construcción de handlers y servicios en bundles por superficie, sin cambiar el comportamiento visible.
- [x] 2.3 Mantener la inicialización de River, OIDC, cache, repositorios y send service, pero con menor acoplamiento en el archivo raíz.
- [x] 2.4 Verificar con tests de `internal/app` que el bootstrapping sigue produciendo una aplicación válida.

## Phase 3: Partición del router por superficie

- [x] 3.1 Convertir `internal/http/server.go` en un entrypoint pequeño que delega en registradores por superficie.
- [x] 3.2 Crear registradores separados para management, data-plane, external-integration, rutas públicas y provider webhooks.
- [x] 3.3 Preservar los prefijos y guards actuales para que el contrato externo no cambie.
- [x] 3.4 Reforzar `internal/http/server_test.go` con casos representativos por superficie para comprobar que el contrato de rutas quedó intacto.

## Phase 4: Extracción de SES fuera del handler HTTP

- [x] 4.1 Mover el parsing de SNS envelope, el parsing del payload SES, el parseo de timestamps y la traducción a `ProviderEvent` a `internal/adapter/ses/webhook`.
- [x] 4.2 Simplificar `internal/http/handler/provider_webhook.go` para que solo haga coordinación HTTP, verificación y delegación.
- [x] 4.3 Mantener el manejo de `SubscriptionConfirmation` y de notificaciones `Notification` sin cambiar la semántica de respuesta HTTP.
- [x] 4.4 Añadir tests unitarios para delivery, bounce, complaint y subscription confirmation en el boundary nuevo.

## Phase 5: Validación y cierre

- [x] 5.1 Ejecutar validación representativa de router y webhook SES con pruebas unitarias y slices focalizados.
- [x] 5.2 Ejecutar una validación E2E/autónoma suficiente para demostrar que management, data-plane, external integration y SES inbound no se rompieron.
- [x] 5.3 Revisar la arquitectura final contra la intención del cambio: composition root más fino, router particionado y SES fuera del handler HTTP.
- [x] 5.4 Capturar tradeoffs residuales y dejar claro qué no se resolvió a propósito para no abrir scope extra.
