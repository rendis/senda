# SNS y perímetro público endurecidos

## Problema

La re-auditoría ciega dejó claro que el perímetro de confianza sigue aceptando más de lo debido. No son fallas cosméticas; son fronteras mal definidas:

- El inbound de SNS no está atado a un `TopicArn` y una cuenta esperados de forma exacta.
- La confirmación automática de suscripciones sigue siendo demasiado abierta.
- `oidc.discovery_url` acepta `http://` sin un guard rail por entorno.
- `allowed_origins` para integraciones externas siguen aceptando `http://`.
- `/public/video-thumbnail` sigue funcionando como fetcher público con cache sin TTL ni política de eviction.
- `SENDA_SNS_SKIP_SIGNATURE_VERIFICATION` sigue siendo un bypass operativo que no queda bloqueado con dureza en producción.
- Hay clientes `http.Client{}` sin timeout explícito y logs que exponen `SubscribeURL` completo.

El problema de fondo es de modelo de confianza. El cambio correcto no es un parche aislado; es redefinir el perímetro con deny-by-default y validación por entorno.

## Intención de la solución

Este cambio cierra el perímetro con controles estructurales:

1. **SNS trust binding exacto**: el inbound debe aceptar solo el `TopicArn` y la cuenta esperados; la suscripción solo puede auto-confirmarse si también pasa por esa política.
2. **Guard rails de producción**: `oidc.discovery_url` y `allowed_origins` deben ser HTTPS en producción; `http://` solo puede sobrevivir en contextos explícitamente no productivos.
3. **Superficie pública de media acotada**: `/public/video-thumbnail` deja de ser un fetcher genérico y pasa a un proxy de miniatura con allowlist, timeout, cache con TTL y eviction.
4. **Bypass operativo bloqueado en producción**: `SENDA_SNS_SKIP_SIGNATURE_VERIFICATION` debe fallar cerrado en prod.
5. **Higiene operativa**: timeouts finitos en los clientes HTTP, y sanitización de logs para no volcar `SubscribeURL` ni URLs sensibles completas.

## Alcance

### En alcance

- Validación de configuración con awareness de entorno.
- SNS inbound: trust binding, confirmación segura y rechazo explícito de suscripciones no esperadas.
- Endurecimiento de `oidc.discovery_url` y `allowed_origins`.
- Endurecimiento de `/public/video-thumbnail` con política de acceso y cache bounded.
- Timeouts de transporte y sanitización de logs operativos.
- E2E autónomo al final para demostrar la postura de seguridad completa.

### Fuera de alcance

- Cambios en el modelo de autenticación OIDC en sí.
- Rediseño del flujo de envío o de la resolución de plantillas.
- Migraciones de clientes externos fuera del repo.
- Compatibilidad transitoria con el perímetro inseguro en producción.

## Dependencias y paralelismo

- **Dependencia base**: `autonomous-e2e-isolation` debe existir para que la validación final tenga un stack autónomo y repetible.
- **Paralelizable**: puede correr en paralelo con otros streams de ciclo 2 que no toquen SNS, media pública, validación de OIDC/orígenes ni clientes HTTP compartidos.
- **No debe solaparse**: con cualquier stream que reescriba `provider_webhook.go`, `media.go`, `config.go`, o la política de middleware/config compartida.
- **Criterio de secuencia**: el reviewer final de seguridad, Lorentz, debe ver evidencia de E2E autónomo antes de marcarlo como aprobado.

## Alternativas consideradas

1. **Permitir `http://` en producción y advertir**. Rechazada. Una advertencia no cierra el riesgo y deja la frontera abierta por error humano.
2. **Mantener el bypass de firmas para operar más rápido**. Rechazada. Un bypass operativo en producción no es un atajo aceptable.
3. **Solo endurecer la validación de entrada sin tocar transportes ni logs**. Rechazada. El borde inseguro también está en el transporte y en la observabilidad.
4. **Deny-by-default con excepciones solo para entornos no productivos y explícitos**. Elegida. Es la única postura coherente con el objetivo del cambio.

## Rollback

El rollback correcto es revertir el cambio completo de perímetro, no desarmarlo por partes. Un rollback parcial reintroduce el mismo modelo inseguro con una falsa sensación de control.

## Reviewer final

Lorentz
