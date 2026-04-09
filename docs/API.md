# Senda API Reference

Referencia práctica de la API v1 de Senda. Todos los endpoints usan JSON y viven bajo `/api/v1/` salvo health/tracking públicos. Para requests listas para importar, ver `docs/postman/`.

---

## Authentication

| Scheme | Header | Plane | Scope |
| --- | --- | --- | --- |
| OIDC / JWT | `Authorization: Bearer <oidc_token>` | Management | RBAC humano sobre recursos global / tenant / workspace |
| API Key | `Authorization: Bearer senda_live_xxx` | Data plane | Una workspace; pensado para aplicaciones |

Las API keys sólo sirven para data plane. La management API requiere OIDC/JWT.

### RBAC Roles

| Role | Description |
| --- | --- |
| `Superadmin` | Acceso total a plataforma y recursos globales |
| `TenantAdmin` | Administra un tenant y todas sus workspaces |
| `WorkspaceAdmin` | Control total de una workspace |
| `WorkspaceEditor` | Edita recursos funcionales de una workspace |
| `WorkspaceViewer` | Solo lectura |

---

## Error Envelope

La API usa un envelope consistente:

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

### Common Error Codes

| HTTP | `error.code` | Typical cause |
| --- | --- | --- |
| 400 | `BAD_REQUEST` | JSON inválido, params mal formados |
| 401 | `UNAUTHORIZED` | Token/API key ausente o inválida |
| 403 | `FORBIDDEN` | RBAC insuficiente |
| 404 | `NOT_FOUND` | Recurso inexistente o borrado lógicamente |
| 409 | `CONFLICT` | Colisión de unicidad o dependencia activa |
| 422 | `VALIDATION_ERROR` | Datos semánticamente inválidos |
| 429 | `RATE_LIMITED` | Rate limit por workspace |
| 500 | `INTERNAL_ERROR` | Error inesperado |

---

## Pagination

Los listados usan cursor-based pagination.

| Parameter | Type | Description |
| --- | --- | --- |
| `cursor` | string | Cursor opaco de la página anterior |
| `limit` | integer | Tamaño de página |

Respuesta típica:

```json
{
  "items": [],
  "next_cursor": "019012ab-...",
  "has_more": true
}
```

---

## Infrastructure / Public Endpoints

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/health` | None | Liveness básica |
| GET | `/healthz` | None | Health con chequeos de dependencias |
| GET | `/metrics` | None | Prometheus scrape |
| GET | `/t/o/:tracking_id` | None | Pixel de open tracking |
| GET | `/public/video-thumbnail` | None | Thumbnail pública |
| POST | `/api/v1/webhooks/ses/inbound` | None | Inbound webhook SES/SNS |
| GET | `/api/v1/onboarding/status` | None | Estado del onboarding |
| POST | `/api/v1/onboarding/setup` | OIDC | Setup inicial de plataforma |

---

## Data Plane (Workspace API Key)

### POST /api/v1/send

Envía un mensaje asincrónico usando una API key de workspace.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `ref` | string | yes | Formato `tenant:workspace:template_type_slug` |
| `to` | string[] | yes | Destinatarios primarios |
| `variables` | object | no | Variables del template |
| `injectors` | object | no | Overrides runtime por injector/field |
| `locale` | string | no | Locale preferido |
| `cc` | string[] | no | CC |
| `bcc` | string[] | no | BCC |
| `external_id` | string | no | Correlation/idempotency ID |

Ejemplo:

```json
{
  "ref": "acme:main:welcome-email",
  "to": ["user@example.com"],
  "variables": {
    "first_name": "Jane"
  },
  "injectors": {
    "student": {
      "full_name": "Jane Doe"
    }
  },
  "locale": "es",
  "external_id": "signup-jane-1"
}
```

### POST /api/v1/send/batch

Mismo `ref` para muchos mensajes, cada item con su propio contexto.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `ref` | string | yes | Template ref compartido |
| `items` | object[] | yes | Un mensaje lógico por item |
| `items[].to` | string | yes | Destinatario primario |
| `items[].variables` | object | no | Variables por item |
| `items[].injectors` | object | no | Overrides runtime por item |
| `items[].locale` | string | no | Locale por item |
| `items[].cc` | string[] | no | CC por item |
| `items[].bcc` | string[] | no | BCC por item |
| `items[].external_id` | string | no | Correlation ID por item |

Ejemplo:

```json
{
  "ref": "acme:main:welcome-email",
  "items": [
    {
      "to": "ana@example.com",
      "variables": {"first_name": "Ana"},
      "injectors": {
        "student": {"full_name": "Ana Pérez"}
      },
      "external_id": "msg-1"
    },
    {
      "to": "bob@example.com",
      "variables": {"first_name": "Bob"},
      "injectors": {
        "student": {"full_name": "Bob Smith"}
      },
      "external_id": "msg-2"
    }
  ]
}
```

### Injector runtime precedence

El merge es **por field**, no por injector completo:

- `allow_overwrite=false` → siempre gana `default_value`
- `allow_overwrite=true` → `reqBody.injectors > code injector value > default_value`
- si existe un field definido en código pero no en el schema DB, se agrega como field extra en runtime

Esto aplica tanto a `POST /api/v1/send` como a `POST /api/v1/send/batch`.

### Query Emails

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/v1/emails` | Lista emails de la workspace |
| GET | `/api/v1/emails/export` | Exporta emails |
| GET | `/api/v1/emails/:tracking_id` | Detalle de email |
| GET | `/api/v1/emails/:tracking_id/events` | Historial de eventos |

---

## Current Member

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/members/me` | OIDC | Perfil autenticado + roles |

---

## Management API

Base: `/api/v1/manage`

### Tenants (Superadmin)

| Method | Path |
| --- | --- |
| POST | `/tenants` |
| GET | `/tenants` |
| GET | `/tenants/:tenant_code` |
| PUT | `/tenants/:tenant_code` |
| DELETE | `/tenants/:tenant_code` |

### Workspaces (TenantAdmin+)

| Method | Path |
| --- | --- |
| POST | `/tenants/:tenant_code/workspaces` |
| GET | `/tenants/:tenant_code/workspaces` |
| GET | `/tenants/:tenant_code/workspaces/:workspace_code` |
| PUT | `/tenants/:tenant_code/workspaces/:workspace_code` |
| DELETE | `/tenants/:tenant_code/workspaces/:workspace_code` |

### Members

#### Global members (Superadmin)

| Method | Path |
| --- | --- |
| POST | `/members` |
| GET | `/members` |
| GET | `/members/:member_id` |
| POST | `/members/:member_id/roles` |
| DELETE | `/members/:member_id/roles/:role_id` |

#### Scoped member creation

También existen rutas scopiadas para crear/listar members en tenant/workspace, con las mismas reglas RBAC del scope resuelto.

**Cambio importante:** si creás un member con un email ya existente, Senda **reutiliza la identidad** (`members`) y sólo agrega el nuevo rol/scope (`member_roles`). No duplica personas por email.

### Config (Superadmin)

| Method | Path |
| --- | --- |
| GET | `/config` |
| PUT | `/config` |

---

## Workspace-scoped resources

Base: `/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code`

### Injectors

| Method | Path | Notes |
| --- | --- | --- |
| POST | `/injectors` | Crea schema completo |
| GET | `/injectors` | Lista del scope actual |
| GET | `/injectors?include_inherited=true` | Incluye workspace + `_system` + global, deduplicado por prioridad |
| GET | `/injectors/:name` | Lookup por nombre |
| PUT | `/injectors/:name` | Reemplaza schema completo |
| PUT | `/injectors/:name/fields/:field_name` | Actualiza `default_value` / `allow_overwrite` de un field |
| PUT | `/injectors/:name/values` | Setea valores persistidos por workspace |
| DELETE | `/injectors/:name` | Soft delete lógico del schema |

#### Create / replace schema body

```json
{
  "name": "student",
  "description": "Datos del alumno",
  "fields": [
    {
      "field_name": "full_name",
      "field_type": "string",
      "position": 1,
      "default_value": "Alumno",
      "allow_overwrite": true
    },
    {
      "field_name": "campus",
      "field_type": "string",
      "position": 2,
      "default_value": "central",
      "allow_overwrite": false
    }
  ]
}
```

#### Update one field runtime policy

```json
{
  "default_value": "central",
  "allow_overwrite": false
}
```

#### Set persistent values

```json
{
  "values": [
    {"field_name": "full_name", "value": "Jane Doe"},
    {"field_name": "campus", "value": "north"}
  ]
}
```

**Runtime semantics:**

- `PUT /injectors/:name` es **destructivo respecto del schema**: reemplaza fields del injector completo
- `default_value` vive en `injector_fields`
- `allow_overwrite` decide si runtime puede usar request/code o debe fijar el default
- `PUT /injectors/:name/values` persiste valores del scope actual; en runtime el fallback base sigue siendo el schema + cadena de scopes + overrides

### Adapters

| Method | Path |
| --- | --- |
| POST | `/adapters` |
| GET | `/adapters` |
| GET | `/adapters/:id` |
| PUT | `/adapters/:id` |
| DELETE | `/adapters/:id` |
| POST | `/adapters/:id/test` |
| GET | `/adapters/:id/setup-guide` |
| POST | `/adapters/:id/auto-provision-tracking` |
| GET | `/adapters/:id/workspace-access` |
| PUT | `/adapters/:id/workspace-access` |

### Adapter identities

| Method | Path |
| --- | --- |
| GET | `/adapters/:id/identities` |
| POST | `/adapters/:id/identities` |
| POST | `/adapters/:id/identities/sync` |
| DELETE | `/adapters/:id/identities/:identity_id` |
| POST | `/adapters/:id/identities/:identity_id/set-default` |
| GET | `/adapters/:id/identities/:identity_id/workspace-access` |
| PUT | `/adapters/:id/identities/:identity_id/workspace-access` |

**Sharing rules**

- el sharing se administra desde la workspace `_system`
- Gmail se comparte a nivel adapter
- SES se comparte a nivel **email identity**, no dominio
- identidades SES de tipo `domain` no son shareables
- un child workspace ve adapters heredados como read-only

### Template types

| Method | Path |
| --- | --- |
| POST | `/template-types` |
| GET | `/template-types` |
| GET | `/template-types/:slug` |
| PUT | `/template-types/:slug` |
| DELETE | `/template-types/:slug` |
| GET | `/template-types/:slug/templates` |

### Templates, versions y locales

| Method | Path |
| --- | --- |
| POST | `/templates` |
| GET | `/templates` |
| GET | `/templates/:template_id` |
| POST | `/templates/:template_id/disable` |
| POST | `/templates/:template_id/enable` |
| POST | `/templates/:template_id/versions` |
| GET | `/templates/:template_id/versions` |
| GET | `/templates/:template_id/versions/:version_id` |
| PUT | `/templates/:template_id/versions/:version_id` |
| POST | `/templates/:template_id/versions/:version_id/publish` |
| POST | `/templates/:template_id/versions/:version_id/locales/:locale` |
| GET | `/templates/:template_id/versions/:version_id/locales` |
| GET | `/templates/:template_id/versions/:version_id/locales/:locale` |
| PUT | `/templates/:template_id/versions/:version_id/locales/:locale` |
| DELETE | `/templates/:template_id/versions/:version_id/locales/:locale` |
| POST | `/templates/:template_id/preview-mjml` |

### Test send

| Method | Path | Notes |
| --- | --- | --- |
| POST | `/templates/:template_id/test-send` | Envía síncrono usando published o latest draft |

Body:

```json
{
  "recipient_email": "qa@example.com",
  "variables": {
    "first_name": "QA"
  },
  "injectors": {
    "student": {
      "full_name": "QA User"
    }
  },
  "locale": "es"
}
```

También respeta la precedencia runtime de injectors por field.

### Bulk send

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/templates/:template_id/bulk-send-config` | Config de límites y comportamiento UI |
| POST | `/templates/:template_id/bulk-send` | Encola batch usando la versión publicada actual |

Body:

```json
{
  "items": [
    {
      "to": "ana@example.com",
      "variables": {"first_name": "Ana"},
      "injectors": {
        "student": {"full_name": "Ana Pérez"}
      },
      "external_id": "bulk-1"
    }
  ]
}
```

Cada item acepta el mismo shape runtime que `send/batch`: `variables`, `injectors`, `locale`, `cc`, `bcc`, `external_id`.

### API Keys

| Method | Path |
| --- | --- |
| POST | `/api-keys` |
| GET | `/api-keys` |
| DELETE | `/api-keys/:id` |

### Webhooks

| Method | Path |
| --- | --- |
| POST | `/webhooks` |
| GET | `/webhooks` |
| GET | `/webhooks/:id` |
| PUT | `/webhooks/:id` |
| DELETE | `/webhooks/:id` |
| POST | `/webhooks/:id/test` |

### Emails / audit / suppression / dashboard

| Resource | Routes |
| --- | --- |
| Emails | `GET /emails`, `GET /emails/:tracking_id`, `GET /emails/:tracking_id/events` |
| Suppression | `POST /suppression`, `GET /suppression/:email`, `DELETE /suppression/:email` |
| Audit log | `GET /audit-log` |
| Dashboard | `GET /dashboard-stats` |

---

## Global-scoped resources

Base: `/api/v1/manage/global`

El scope global replica la mayoría de los recursos de workspace para configuración plataforma-wide.

| Resource | Coverage |
| --- | --- |
| Injectors | create/list/get/update schema/update field/delete |
| Adapters | CRUD + test + setup guide + auto provision |
| Adapter identities | list/create/sync/delete/set-default |
| Template types | CRUD |
| Templates | create/list/get/disable/enable + versions + locales + preview + test-send |
| Audit log | list |
| Dashboard | stats |

---

## Webhook Events

| Event | Description |
| --- | --- |
| `email.queued` | aceptado y encolado |
| `email.sent` | entregado al provider |
| `email.delivered` | provider confirmó delivery |
| `email.bounced` | bounce duro o blando |
| `email.complained` | complaint / spam report |
| `email.failed` | fallo permanente |
| `email.opened` | pixel de apertura |
| `*` | wildcard |

### Signature

Los webhooks salientes usan HMAC-SHA256 en `X-Senda-Signature`:

```text
sha256=<hex_digest>
```

---

## Rate limiting

El data plane aplica token bucket por workspace. Si excedés el límite, responde `429 RATE_LIMITED`.

---

## Postman

Colección importable: `docs/postman/senda-api-v1.postman_collection.json`.
