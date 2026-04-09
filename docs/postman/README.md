# Senda API - Postman Collection

Colección Postman para explorar la API v1 de Senda sin inventar payloads a mano.

## Files

| File | Description |
| --- | --- |
| `senda-api-v1.postman_collection.json` | Colección principal |
| `senda-local.postman_environment.json` | Environment local |
| `senda-staging.postman_environment.json` | Environment staging/template |

## Quick Start

1. Importá la colección
2. Importá el environment local o staging
3. Configurá `base_url`, `oidc_token` y/o `api_key`
4. Corré `Health` para validar conectividad

## Authentication

### Management API

Bearer OIDC/JWT en `oidc_token` para `/api/v1/manage/*` y `/api/v1/members/me`.

### Data plane

Bearer API key raw (`senda_live_*`) en `api_key` para:

- `POST /api/v1/send`
- `POST /api/v1/send/batch`
- `GET /api/v1/emails`
- `GET /api/v1/emails/:tracking_id`
- `GET /api/v1/emails/:tracking_id/events`
- `GET /api/v1/emails/export`

## Collection Structure

| Folder | Notes |
| --- | --- |
| Health | health / healthz / metrics |
| Onboarding | status + setup |
| Tenants / Workspaces / Members / Config | management core |
| Injectors | schema, field runtime policy, values, inherited list |
| Adapters | CRUD, identities, `_system` sharing |
| Template Types | CRUD |
| Templates | CRUD + preview + versions/locales + test-send + bulk-send |
| Send | `send` + `send/batch` |
| Emails | data-plane query |
| Suppression / Audit Log / Webhooks / API Keys | workspace ops |
| SES Webhooks | public SNS endpoint |
| Global (Superadmin) | recursos globales |

## Variables

| Variable | Description |
| --- | --- |
| `base_url` | base URL del server |
| `oidc_token` | JWT de management |
| `api_key` | API key raw de workspace |
| `tenant_code` | tenant actual |
| `workspace_code` | workspace actual |
| `system_workspace_code` | default `_system` |
| `member_id` | member actual |
| `role_id` | role actual |
| `adapter_id` | adapter actual |
| `identity_id` | identity actual |
| `injector_name` | injector actual |
| `template_type_id` | template type actual |
| `template_id` | template actual |
| `version_id` | version actual |
| `tracking_id` | tracking actual |
| `webhook_id` | webhook actual |
| `api_key_id` | API key record actual |

## Injectors notes

La colección ya refleja el modelo runtime actual:

- `GET /injectors?include_inherited=true`
- `PUT /injectors/:name`
- `PUT /injectors/:name/fields/:field_name`
- `PUT /injectors/:name/values`
- `DELETE /injectors/:name`

### Runtime precedence

- `allow_overwrite=false` → gana `default_value`
- `allow_overwrite=true` → `reqBody.injectors > code injector value > default_value`
- la precedencia es **por field**

### Important semantics

- `PUT /injectors/:name` reemplaza el schema completo
- `PUT /injectors/:name/fields/:field_name` sirve para ajustar política runtime de un field puntual
- `PUT /injectors/:name/values` persiste valores del scope actual

## Shared adapter notes

- Gmail se comparte desde `_system` a nivel adapter
- SES se comparte desde `_system` a nivel **email identity**
- las domain identities SES no son shareables
- la colección ya evita referencias viejas a Domains/DKIM porque Senda sigue el ADR de provider-managed auth

## Recommended order

1. Health
2. Onboarding
3. Create workspace
4. Create adapter
5. Create template type
6. Create template
7. Create/publish version
8. Create API key
9. Send / Send Batch / Test Send / Bulk Send
10. Query Emails

## Error format

Todas las requests esperan este envelope:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "validation failed",
    "details": [
      {
        "field": "name",
        "message": "is required"
      }
    ],
    "request_id": "req_123"
  }
}
```
