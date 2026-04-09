---
name: senda
description: >-
  Interact with the Senda email orchestration API via MCP tools.
  Manage tenants, workspaces, members, email adapters, templates,
  injectors, webhooks, API keys, and send emails.
  Trigger: When working with senda, email orchestration, email templates,
  email sending, or managing email provider adapters.
allowed-tools:
  - mcp__senda__*
---

# Senda MCP

MCP integration for Senda, generado desde el OpenAPI usando [mcp-openapi-proxy](https://github.com/rendis/mcp-openapi-proxy). Cada endpoint expone una tool `senda_{method}_{path}`.

**Repository:** https://github.com/rendis/senda
**Install proxy:** `go install github.com/rendis/mcp-openapi-proxy/cmd/mcp-openapi-proxy@latest`

## Setup

### Claude Code

El repo ya incluye `.mcp.json`.

```bash
make dev
# Claude detecta .mcp.json automáticamente
```

### OpenAI Codex

Editar `~/.codex/config.toml`:

```toml
[mcp_servers.senda]
command = "mcp-openapi-proxy"
args = []

[mcp_servers.senda.env]
MCP_SPEC = "https://raw.githubusercontent.com/rendis/senda/main/cmd/senda/docs/openapi.yaml"
MCP_BASE_URL = "<your-api-url>"
MCP_TOOL_PREFIX = "senda"
MCP_AUTH_PROFILE = "senda"
MCP_OIDC_ISSUER = "http://localhost:9090/realms/senda"
MCP_OIDC_CLIENT_ID = "tether-mcp"
```

### Gemini CLI

Editar `~/.gemini/settings.json` o `.gemini/settings.json`:

```json
{
  "mcpServers": {
    "senda": {
      "command": "mcp-openapi-proxy",
      "args": [],
      "env": {
        "MCP_SPEC": "https://raw.githubusercontent.com/rendis/senda/main/cmd/senda/docs/openapi.yaml",
        "MCP_BASE_URL": "<your-api-url>",
        "MCP_TOOL_PREFIX": "senda",
        "MCP_AUTH_PROFILE": "senda",
        "MCP_OIDC_ISSUER": "http://localhost:9090/realms/senda",
        "MCP_OIDC_CLIENT_ID": "tether-mcp"
      }
    }
  }
}
```

### OIDC login flow

```bash
mcp-openapi-proxy login
mcp-openapi-proxy status
mcp-openapi-proxy logout
```

## Available MCP Tools

Naming pattern: `senda_{method}_{path}`. Usá `senda_list_endpoints` para discovery completo.

### Key Tools

| Tool | Purpose |
| --- | --- |
| `senda_get_health` | Health check |
| `senda_post_api_v1_send` | Send async |
| `senda_post_api_v1_send_batch` | Batch send async con contexto por item |
| `senda_get_api_v1_emails` | List sent emails |
| `senda_get_api_v1_onboarding_status` | Check onboarding |
| `senda_post_api_v1_onboarding_setup` | First-time setup |
| `senda_post_api_v1_manage_tenants` | Create tenant |
| `senda_post_api_v1_manage_tenants_tc_workspaces` | Create workspace |
| `senda_post_api_v1_manage_members` | Create global member |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_adapters` | Create adapter |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_templates` | Create template |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_templates_template_id_test_send` | Sync template test-send |
| `senda_get_api_v1_manage_tenants_tc_workspaces_wc_templates_template_id_bulk_send_config` | Read bulk-send limits |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_templates_template_id_bulk_send` | Queue bulk-send |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_api_keys` | Create API key |

## Quick Start Workflow

```text
1. senda_get_api_v1_onboarding_status
2. senda_post_api_v1_onboarding_setup
3. senda_get_api_v1_manage_tenants
4. senda_post_api_v1_manage_tenants_tc_workspaces
5. senda_post_api_v1_manage_tenants_tc_workspaces_wc_adapters
6. senda_post_api_v1_manage_tenants_tc_workspaces_wc_templates
7. senda_post_api_v1_manage_tenants_tc_workspaces_wc_api_keys
8. senda_post_api_v1_send
9. senda_post_api_v1_send_batch (optional)
10. senda_get_api_v1_emails
```

## API Groups

### Public

- `GET /health`
- `GET /healthz`
- `GET /metrics`
- `GET /t/o/{tracking_id}`
- `GET /public/video-thumbnail`
- `GET /api/v1/onboarding/status`
- `POST /api/v1/webhooks/ses/inbound`

### Data Plane (`WorkspaceAPIKeyBearer`)

Auth: `Authorization: Bearer senda_live_...`

- `POST /api/v1/send`
- `POST /api/v1/send/batch`
- `GET /api/v1/emails`
- `GET /api/v1/emails/{tracking_id}`
- `GET /api/v1/emails/{tracking_id}/events`
- `GET /api/v1/emails/export`

`send` y `send/batch` aceptan `variables` **e `injectors`** en runtime.

### Management API (`ManagementBearer`)

Auth: `Authorization: Bearer <oidc-jwt>`

#### Tenants (Superadmin)
- `POST/GET /api/v1/manage/tenants`
- `GET/PUT/DELETE /api/v1/manage/tenants/{tenant_code}`

#### Workspaces (TenantAdmin+)
- `POST/GET /api/v1/manage/tenants/{tc}/workspaces`
- `GET/PUT/DELETE /api/v1/manage/tenants/{tc}/workspaces/{wc}`

#### Members
- `GET/POST /api/v1/manage/members`
- `GET /api/v1/manage/members/{member_id}`
- `POST /api/v1/manage/members/{member_id}/roles`
- `DELETE /api/v1/manage/members/{member_id}/roles/{role_id}`
- scoped member routes also exist under tenant/workspace hierarchy

#### Workspace resources (under `.../workspaces/{wc}/`)

| Resource | Create | List | Get | Update | Delete | Extra |
| --- | --- | --- | --- | --- | --- | --- |
| Adapters | POST | GET | `GET /{id}` | `PUT /{id}` | DELETE | `POST /{id}/test`, `GET /{id}/setup-guide`, sharing endpoints |
| Adapter identities | POST | GET | — | — | DELETE | `POST /sync`, `POST /set-default`, identity workspace access |
| Template types | POST | GET | `GET /{slug}` | `PUT /{slug}` | DELETE | `GET /{slug}/templates` |
| Templates | POST | GET | `GET /{template_id}` | — | — | disable/enable, preview, versions, locales, `test-send`, `bulk-send-config`, `bulk-send` |
| Injectors | POST | GET | `GET /{name}` | `PUT /{name}`, `PUT /{name}/fields/{field_name}`, `PUT /{name}/values` | DELETE | `GET ?include_inherited=true` |
| API keys | POST | GET | — | — | DELETE | — |
| Webhooks | POST | GET | `GET /{id}` | `PUT /{id}` | DELETE | `POST /{id}/test` |
| Emails | — | GET | `GET /{tracking_id}` | — | — | `GET /{tracking_id}/events` |
| Suppression | POST | — | `GET /{email}` | — | DELETE | — |
| Audit log | — | GET | — | — | — | — |
| Dashboard | — | GET | — | — | — | stats |

#### Global scope (`/api/v1/manage/global/...`)

Misma idea que workspace scope para recursos globales. En injectors globales también existen:

- `POST /injectors`
- `GET /injectors`
- `GET /injectors/{name}`
- `PUT /injectors/{name}`
- `PUT /injectors/{name}/fields/{field_name}`
- `DELETE /injectors/{name}`

## Injectors runtime model

### DB schema fields

Cada `injector_field` puede definir:

- `default_value`
- `allow_overwrite`

### Runtime precedence

La precedencia es **por field**, no por injector completo:

- `allow_overwrite=false` → siempre gana `default_value`
- `allow_overwrite=true` → `reqBody.injectors > code injector value > default_value`
- fields no definidos en el schema DB pero devueltos por un code injector se agregan al resultado runtime

### Operational notes

- `GET /injectors?include_inherited=true` devuelve workspace + `_system` + global, deduplicado por prioridad
- `PUT /injectors/{name}` reemplaza el schema completo del injector
- `PUT /injectors/{name}/fields/{field_name}` actualiza la política runtime de un field sin reemplazar el resto
- `PUT /injectors/{name}/values` persiste valores del scope actual

## Shared adapters from `_system`

- `GET/PUT /api/v1/manage/tenants/{tc}/workspaces/{wc}/adapters/{id}/workspace-access`
  - válido para Gmail compartido desde `_system`
- `GET/PUT /api/v1/manage/tenants/{tc}/workspaces/{wc}/adapters/{id}/identities/{identity_id}/workspace-access`
  - válido para SES compartido por **email identity** desde `_system`
- SES `domain` identities no son shareables
- child workspaces ven recursos heredados como read-only

## Auth schemes

| Scheme | Type | Used by |
| --- | --- | --- |
| `ManagementBearer` | HTTP Bearer JWT | `/api/v1/manage/*`, `/api/v1/members/me`, onboarding setup |
| `WorkspaceAPIKeyBearer` | HTTP Bearer `senda_live_*` | `/api/v1/send*`, `/api/v1/emails*` |

## RBAC summary

| Role | Scope | Access |
| --- | --- | --- |
| Superadmin | Global | todo, incluyendo recursos globales, tenants y members |
| TenantAdmin | Tenant | CRUD de workspaces y administración tenant |
| WorkspaceAdmin | Workspace | adapters, template types, templates, API keys, webhooks, injectors schema CRUD |
| WorkspaceEditor | Workspace | template versions/locales, injector values, injector field runtime policy, test-send, bulk-send |
| WorkspaceViewer | Workspace | lectura |

## Go SDK (embedders)

Senda también puede embederse vía `sdk`.

```go
engine := sdk.NewWithConfig("config.yaml")
engine.RegisterInjector(&MyInjector{})
engine.SetInitFunc(myInit)
engine.OnStart(func(ctx context.Context) error { return nil })
engine.OnShutdown(func(ctx context.Context) error { return nil })
engine.Run()
```

### Injector interface

```go
type Injector interface {
    Code() string
    Resolve() (ResolveFunc, []string)
    IsCritical() bool
    Timeout() time.Duration
}
```

`InjectorContext` expone headers, variables, init data, injectors resueltos y `RequestInjectors()` para overrides runtime del request body.

## Key Configuration

| Config | Env Var | Required | Purpose |
| --- | --- | --- | --- |
| Database URL | `SENDA_DATABASE_URL` | Yes | PostgreSQL 16 |
| Master Key | `SENDA_MASTER_KEY` | Yes | AES-256-GCM |
| OIDC Discovery | `SENDA_OIDC_DISCOVERY_URL` | Yes | issuer discovery |
| OIDC Client ID | `SENDA_OIDC_CLIENT_ID` | Yes | management auth |
| OIDC Client Secret | `SENDA_OIDC_CLIENT_SECRET` | Yes | management auth |
| Port | `SENDA_PORT` | No | HTTP port |
| Environment | `SENDA_ENVIRONMENT` | No | prod/dev |

## SES Adapter Lifecycle

### Provisioning flow (6 steps)

| Step | Name | AWS Operation |
| --- | --- | --- |
| 1 | `create_configuration_set` | `ses:CreateConfigurationSet` |
| 2 | `create_sns_topic` | `sns:CreateTopic` |
| 3 | `create_event_destination` | `ses:CreateConfigurationSetEventDestination` |
| 4 | `subscribe_webhook` | `sns:Subscribe` |
| 5 | `save_configuration` | DB only |
| 6 | `verify_subscription` | `sns:GetSubscriptionAttributes` |

### Deprovision flow (4 steps)

| Step | Name | AWS Operation |
| --- | --- | --- |
| 10 | `deprov_unsubscribe_webhook` | `sns:Unsubscribe` |
| 11 | `deprov_delete_event_destination` | `ses:DeleteConfigurationSetEventDestination` |
| 12 | `deprov_delete_sns_topic` | `sns:DeleteTopic` |
| 13 | `deprov_delete_configuration_set` | `ses:DeleteConfigurationSet` |
