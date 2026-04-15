# Tasks — Hardening del perímetro de seguridad

## 1. Rediseño del perímetro

1.1 [x] Definir el contrato de confianza para SNS inbound, webhooks outbound, media pública e integraciones externas.  
1.2 [x] Cerrar la lista de destinos permitidos y la política deny-by-default.  
1.3 [x] Definir los cambios de configuración y el estado persistente necesario para replay/deduplicación.  

## 2. Implementación SNS inbound

2.1 [x] Alinear `provider_webhook.go` con el `TopicArn` esperado exacto.  
2.2 [x] Validar también la cuenta esperada derivada del ARN o de la config.  
2.3 [x] Implementar deduplicación/replay con persistencia en PostgreSQL.  
2.4 [x] Rechazar mensajes duplicados, fuera de ventana o con origen no esperado.  

## 3. Implementación outbound webhook

3.1 [x] Bloquear redirects automáticos en `WebhookWorker`.  
3.2 [x] Convertir el redirect en falla permanente o equivalente seguro.  
3.3 [x] Asegurar que la política SSRF sea explícita y no implícita.  

## 4. Implementación media pública

4.1 [x] Introducir allowlist explícita para hosts públicos.  
4.2 [x] Pinnear el destino resuelto durante todo el request.  
4.3 [x] Revalidar redirects con la misma política y bloquear destinos inseguros.  
4.4 [x] Cubrir DNS rebinding y destinos privados/reservados.  

## 5. Implementación de autenticación externa

5.1 [x] Eliminar el fallback a `?token=` en integraciones externas.  
5.2 [x] Mantener el header dedicado como único transporte permitido.  
5.3 [x] Asegurar respuestas de rechazo claras y consistentes.  

## 6. Pruebas de seguridad

6.1 [x] Agregar tests negativos para `TopicArn` y cuenta incorrectos.  
6.2 [x] Agregar tests de replay/deduplicación.  
6.3 [x] Agregar tests de redirect en webhook outbound.  
6.4 [x] Agregar tests de rebinding/pinning en media fetch.  
6.5 [x] Agregar tests de rechazo de token por query string.  

## 7. Validación E2E autónoma

7.1 [x] Encadenar un flujo E2E con casos válidos y negativos para cada borde.  
7.2 [x] Verificar que los fallos de seguridad no degraden la evidencia de prueba.  
7.3 [x] Consolidar el resultado final para signoff de seguridad.  
