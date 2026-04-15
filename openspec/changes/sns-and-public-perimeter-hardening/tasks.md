# Tasks — SNS y perímetro público endurecidos

## 1. Contrato de política y configuración

1.1 [x] Definir una validación de entorno explícita para distinguir producción, desarrollo y test.  
1.2 [x] Endurecer `oidc.discovery_url` para que HTTPS sea obligatorio en producción y en entornos compartidos.  
1.3 [x] Endurecer `allowed_origins` con la misma regla: HTTP solo en contextos no productivos y explícitos.  
1.4 [x] Bloquear `SENDA_SNS_SKIP_SIGNATURE_VERIFICATION` cuando el entorno sea producción.  
1.5 [x] Introducir parámetros de política para SNS y media: destino esperado, timeout y límites de cache.

## 2. SNS inbound y confirmación segura

2.1 [x] Atar el inbound de SNS al `TopicArn` esperado exacto.  
2.2 [x] Validar también la cuenta esperada del ARN.  
2.3 [x] Confirmar suscripciones solo después de pasar la validación de política.  
2.4 [x] Sustituir cualquier `http.Client{}` implícito por un cliente con timeout finito.  
2.5 [x] Sanitizar logs para no volcar `SubscribeURL` completo ni datos sensibles de confirmación.  
2.6 [x] Cubrir el flujo con tests negativos para `TopicArn`, cuenta y bypass operativo.

## 3. Perímetro público de media

3.1 [x] Reemplazar el cache actual por una estructura bounded con TTL y eviction.  
3.2 [x] Validar esquema y destino con allowlist explícita antes de fetch.  
3.3 [x] Aplicar timeout de transporte y política de redirects segura o directamente cerrada, incluyendo allowlist en cada hop de redirect.  
3.4 [x] Redactar logs operativos para no exponer URLs completas.  
3.5 [x] Agregar tests de rechazo para destinos inseguros, cache sin crecimiento y expiración.

## 4. Integraciones externas y origen permitido

4.1 [x] Reusar la política de validación de origen para `allowed_origins`.  
4.2 [x] Asegurar que la validación de `oidc.discovery_url` y orígenes sea consistente entre config y middleware.  
4.3 [x] Añadir tests de producción vs. no producción para la aceptación o rechazo de HTTP.

## 5. Validación autónoma final

5.1 [x] Construir un escenario E2E autónomo positivo para SNS, OIDC/orígenes y media permitidos.  
5.2 [x] Construir escenarios E2E negativos para cada guard rail: SNS no ligado, HTTP en prod, media fuera de allowlist y bypass de firmas en prod.  
5.3 [x] Verificar que los logs y errores no filtren `SubscribeURL` completo ni otros secretos operativos.  
5.4 [x] Consolidar evidencia final para revisión de seguridad con Lorentz.

## Dependencia y paralelismo

- Las fases 2, 3 y 4 pueden dividirse entre subtrabajos si la política común de configuración ya está acordada.
- La fase 5 depende de que todas las guard rails estén implementadas y probadas a nivel unitario/integración.
- No iniciar cambios paralelos sobre las mismas superficies (`provider_webhook.go`, `media.go`, validación de config compartida) sin coordinar el contrato de política.
