# Hardening del perímetro de seguridad

## Problema

Hoy el perímetro de confianza acepta más de lo que debería. La revisión del código confirmó cuatro superficies débiles que no se resuelven con un parche cosmético:

- `internal/http/handler/provider_webhook.go` solo valida que `TopicArn` empiece con `arn:aws:sns:`; no lo ata a un valor esperado concreto ni a una cuenta permitida.
- `internal/adapter/river/webhook_worker.go` crea un `http.Client` sin política explícita de redirects; por defecto el cliente puede seguir redirecciones.
- `internal/http/middleware/external_integration.go` todavía acepta `?token=...` como transporte alternativo del token.
- `internal/http/handler/media.go` valida y revalida destinos durante redirects, pero no pinnea el destino resuelto a nivel de transporte; eso deja una ventana para DNS rebinding.

El problema de fondo no es un bug aislado: es un modelo de confianza demasiado laxo. La corrección tiene que ser de perímetro, no cosmética.

## Intención de la solución

Endurecer el perímetro con una postura deny-by-default:

1. SNS inbound solo debe aceptar el `TopicArn` y la cuenta esperada, con protección explícita contra replay/deduplicación.
2. La entrega outbound de webhooks debe rechazar redirects o, si se habilita una política más flexible en el futuro, revalidar cada hop con la misma política SSRF.
3. La descarga de media pública debe requerir allowlist explícita y pinning del destino resuelto para bloquear DNS rebinding y destinos inseguros.
4. Las integraciones externas deben dejar de aceptar bearer token por query string.

Este cambio asume **sin retrocompatibilidad**. El objetivo es seguridad correcta, no migración blanda.

## Breaking changes aceptados

- SNS que no coincida exactamente con el `TopicArn` y la cuenta esperada será rechazada.
- Mensajes SNS duplicados o fuera de ventana de replay serán descartados.
- Webhooks con redirects dejarán de funcionar.
- Fetch de media desde hosts no autorizados o no pinneables dejará de funcionar.
- Integraciones externas que dependan de `?token=` deberán migrar al transporte por header.

## Alcance

### En alcance

- Verificación estricta de confianza para inbound SNS.
- Deduplicación y anti-replay explícitos.
- Política outbound SSRF para webhooks.
- Pinning y deny-by-default en media pública.
- Eliminación del token por query string en integraciones externas.
- Tests negativos y E2E de seguridad para los casos críticos.

### Fuera de alcance

- Compatibilidad transitoria con el flujo inseguro anterior.
- Migraciones de cliente fuera del repo.
- Rediseño general del modelo de autenticación OIDC/API key.
- Cambios de comportamiento que no toquen el perímetro de seguridad.

## Dependencias

- PostgreSQL para persistir el estado de deduplicación/replay.
- Configuración explícita de los destinos permitidos para SNS y media.
- Harness de pruebas para validar negativos y E2E de seguridad.

## Alternativas consideradas

1. **Mantener compatibilidad y solo advertir.** Más fácil de desplegar, pero deja la superficie insegura viva y contradice el objetivo del cambio.
2. **Permitir redirects con validación por hop.** Es viable, pero aumenta la complejidad operativa y abre margen para errores de política. Solo lo aceptaría si un caso de negocio lo exige de forma explícita.
3. **Deny-by-default con allowlist/pinning obligatorio.** Más estricta, más segura y coherente con la dirección ya acordada. Es la opción elegida.

## Rollback

El rollback correcto es restaurar la versión anterior del perímetro y, si hiciera falta, revertir el cambio completo. No se propone rollback parcial porque reintroduciría incoherencia en el modelo de confianza.

## Reviewer final

Lorentz
