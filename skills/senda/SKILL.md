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

MCP integration for Senda — a multi-tenant email orchestration platform with MJML templates, provider abstraction (SES, Gmail, SMTP), RBAC, audit trails, and webhooks.

Uses [mcp-openapi-proxy](https://github.com/rendis/mcp-openapi-proxy) to auto-generate MCP tools from the OpenAPI spec. Each API endpoint becomes a callable tool.

**Repository**: https://github.com/rendis/senda
**Install proxy**: `go install github.com/rendis/mcp-openapi-proxy/cmd/mcp-openapi-proxy@latest`

## Setup

### Claude Code

The project includes `.mcp.json` — Claude Code auto-detects it. No manual setup needed.

```bash
make dev          # Start API on port 8081 (docker-compose: postgres + keycloak + senda)
# MCP auto-configured via .mcp.json
```

Verify: `claude mcp list` → should show `senda` connected.

### OpenAI Codex

Edit `~/.codex/config.toml`:

```toml
[mcp_servers.senda]
command = "mcp-openapi-proxy"
args = []

[mcp_servers.senda.env]
MCP_SPEC = "https://raw.githubusercontent.com/rendis/senda/main/cmd/senda/docs/openapi.yaml"
MCP_BASE_URL = "<your-api-url>"
MCP_TOOL_PREFIX = "senda"
```

### Gemini CLI

Edit `~/.gemini/settings.json` (global) or `.gemini/settings.json` (project):

```json
{
  "mcpServers": {
    "senda": {
      "command": "mcp-openapi-proxy",
      "args": [],
      "env": {
        "MCP_SPEC": "https://raw.githubusercontent.com/rendis/senda/main/cmd/senda/docs/openapi.yaml",
        "MCP_BASE_URL": "<your-api-url>",
        "MCP_TOOL_PREFIX": "senda"
      }
    }
  }
}
```

### OIDC Authentication (production)

```bash
mcp-openapi-proxy login     # browser-based OIDC PKCE login
mcp-openapi-proxy status    # check auth status
mcp-openapi-proxy logout    # clear stored tokens
```

## Available MCP Tools

Tool naming: `senda_{method}_{path}`. Use `senda_list_endpoints` to discover all.

### Key Tools

| Tool | Purpose |
|------|---------|
| `senda_get_health` | Health check |
| `senda_post_api_v1_send` | Send an email (async, 202) |
| `senda_get_api_v1_emails` | List sent emails |
| `senda_get_api_v1_onboarding_status` | Check if onboarding needed |
| `senda_post_api_v1_onboarding_setup` | Run first-time setup |
| `senda_post_api_v1_manage_tenants` | Create tenant |
| `senda_get_api_v1_manage_tenants` | List tenants |
| `senda_post_api_v1_manage_tenants_tc_workspaces` | Create workspace |
| `senda_get_api_v1_manage_members` | List members |
| `senda_post_api_v1_manage_members` | Create member |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_adapters` | Create email adapter |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_templates` | Create template |
| `senda_post_api_v1_manage_tenants_tc_workspaces_wc_api_keys` | Create API key |

## Quick Start Workflow

```
1. senda_get_api_v1_onboarding_status       → check if setup needed
2. senda_post_api_v1_onboarding_setup       → create tenant + member (first time)
3. senda_get_api_v1_manage_tenants          → list tenants
4. senda_post_api_v1_manage_tenants_tc_workspaces  → create workspace
5. senda_post_api_v1_manage_tenants_tc_workspaces_wc_adapters  → add email provider
6. senda_post_api_v1_manage_tenants_tc_workspaces_wc_templates → create template
7. senda_post_api_v1_manage_tenants_tc_workspaces_wc_api_keys  → create API key
8. senda_post_api_v1_send                   → send email with API key
9. senda_get_api_v1_emails                  → check delivery status
```

## API Groups

### Public (no auth)

- `GET /health` — health check
- `GET /healthz` — full health (pinger)
- `GET /metrics` — Prometheus metrics
- `GET /t/o/{tracking_id}` — email open-tracking pixel
- `GET /public/video-thumbnail` — video thumbnail composite
- `GET /api/v1/onboarding/status` — check setup status

### Data Plane (WorkspaceAPIKeyBearer)

Auth: `Authorization: Bearer senda_live_...` (workspace API key)

- `POST /api/v1/send` — send email (202 Accepted, async)
- `GET /api/v1/emails` — list emails (cursor pagination)
- `GET /api/v1/emails/{tracking_id}` — get email details
- `GET /api/v1/emails/{tracking_id}/events` — get email events
- `GET /api/v1/emails/export` — export emails (CSV/JSON)

### Webhooks Inbound (no auth, SNS signature)

- `POST /api/v1/webhooks/ses/inbound` — SES/SNS event ingestion

### Management API (ManagementBearer — OIDC JWT)

Auth: `Authorization: Bearer <keycloak-jwt>`

All routes under `/api/v1/manage/` require OIDC JWT + role-based access.

#### Tenants (Superadmin)
- `POST/GET /api/v1/manage/tenants`
- `GET/PUT/DELETE /api/v1/manage/tenants/{tenant_code}`

#### Workspaces (TenantAdmin+)
- `POST/GET /api/v1/manage/tenants/{tc}/workspaces`
- `GET/PUT/DELETE /api/v1/manage/tenants/{tc}/workspaces/{wc}`

#### Members (Superadmin)
- `GET/POST /api/v1/manage/members`
- `GET /api/v1/manage/members/{id}`
- `POST /api/v1/manage/members/{id}/roles`
- `DELETE /api/v1/manage/members/{id}/roles/{role_id}`

#### Workspace Resources (under `.../workspaces/{wc}/`)

| Resource | Create | List | Get | Update | Delete | Test |
|----------|--------|------|-----|--------|--------|------|
| Adapters | POST | GET | GET /{id} | PUT /{id} | DELETE /{id} | POST /{id}/test |
| Adapter Identities | POST | GET | — | — | DELETE /{id} | — |
| Templates | POST | GET | GET /{id} | — | — | POST /{id}/test-send |
| Template Versions | POST | GET | GET /{vid} | PUT /{vid} | — | POST /{vid}/publish |
| Template Locales | POST | GET | GET /{loc} | PUT /{loc} | DELETE /{loc} | — |
| Template Types | POST | GET | GET /{slug} | — | — | — |
| Injectors | POST | GET | GET /{name} | PUT /{name}/values | — | — |
| API Keys | POST | GET | — | — | DELETE /{id} | — |
| Webhooks | POST | GET | GET /{id} | PUT /{id} | DELETE /{id} | POST /{id}/test |
| Emails | — | GET | GET /{tid} | — | — | — |
| Suppression | POST | GET /{email} | — | — | DELETE /{email} | — |
| Audit Log | — | GET | — | — | — | — |
| Dashboard Stats | — | GET | — | — | — | — |

#### Global Scope (Superadmin, under `/api/v1/manage/global/`)
Same resources as workspace-scoped but for system-wide configuration.

#### Config (Superadmin)
- `GET/PUT /api/v1/manage/config`

#### Current User
- `GET /api/v1/members/me` — get authenticated member profile (OIDC only)

## Auth Schemes

| Scheme | Type | Format | Used by |
|--------|------|--------|---------|
| ManagementBearer | HTTP Bearer | JWT from OIDC (Keycloak) | Management API (`/api/v1/manage/*`) |
| WorkspaceAPIKeyBearer | HTTP Bearer | `senda_live_...` | Data Plane (`/api/v1/send`, `/api/v1/emails`) |

## RBAC Roles

| Role | Scope | Access |
|------|-------|--------|
| Superadmin | Global | All operations, tenant/member management, global config |
| TenantAdmin | Tenant | Workspace CRUD, tenant settings |
| WorkspaceAdmin | Workspace | Adapters, API keys, webhooks, template publish, suppression |
| WorkspaceEditor | Workspace | Template versions/locales, injector values, test-send |
| WorkspaceViewer | Workspace | Read-only access to all workspace resources |

## Go SDK (for embedders)

Senda can be embedded as a library via the `sdk` package:

```go
import "github.com/rendis/senda/sdk"

engine := sdk.NewWithConfig("config.yaml")
engine.RegisterInjector(&MyInjector{})      // custom template variables
engine.SetInitFunc(myInit)                   // per-request init
engine.OnStart(func(ctx context.Context) error { return nil })
engine.OnShutdown(func(ctx context.Context) error { return nil })
engine.Run()
```

### Injector Interface

```go
type Injector interface {
    Code() string                                           // unique name
    Resolve() (ResolveFunc, []string)                       // resolver + dependencies
    IsCritical() bool                                       // true = failure aborts send
    Timeout() time.Duration                                 // max resolution time (0 = 30s)
}
type ResolveFunc func(ctx context.Context, injCtx *InjectorContext) (map[string]any, error)
```

Templates access injected values as `{{ injector.CODE.fieldName }}`.

## Key Configuration

| Config | Env Var | Required | Purpose |
|--------|---------|----------|---------|
| Database URL | `SENDA_DATABASE_URL` | Yes | PostgreSQL 16 connection |
| Master Key | `SENDA_MASTER_KEY` | Yes | AES-256-GCM encryption (32+ chars) |
| OIDC Discovery | `SENDA_OIDC_DISCOVERY_URL` | Yes | Keycloak .well-known URL |
| OIDC Client ID | `SENDA_OIDC_CLIENT_ID` | Yes | OIDC client identifier |
| OIDC Client Secret | `SENDA_OIDC_CLIENT_SECRET` | Yes | OIDC client secret |
| Port | `SENDA_PORT` | No | HTTP port (default: 8080) |
| Log Level | `SENDA_LOG_LEVEL` | No | debug, info, warn, error |
| Environment | `SENDA_ENVIRONMENT` | No | production, development |

## SES Adapter Lifecycle

### Provisioning Flow (6 steps)

Auto-provisions SES tracking resources when `SENDA_TRACKING_BASE_URL` is set.
Tracked in `adapter_provisioning_steps` table. Each step is idempotent.

| Step | Name | AWS Operation |
|------|------|---------------|
| 1 | create_configuration_set | ses:CreateConfigurationSet |
| 2 | create_sns_topic | sns:CreateTopic |
| 3 | create_event_destination | ses:CreateConfigurationSetEventDestination |
| 4 | subscribe_webhook | sns:Subscribe (HTTPS) |
| 5 | save_configuration | DB only (persist config set name) |
| 6 | verify_subscription | sns:GetSubscriptionAttributes (polling, 15s timeout) |

Resource naming:
- ConfigSet: `senda-{id[:8]}`
- Topic: `senda-ses-events-{id[:8]}`
- EventDest: `senda-events` (fixed)
- Webhook: `{baseURL}/api/v1/webhooks/ses/inbound`

### Deprovision Flow (4 steps)

Executes on adapter soft-delete (SES type only). Best-effort -- failure does not block deletion.
Tracked in same `adapter_provisioning_steps` table with `deprov_` prefix.

| Step | Name | AWS Operation |
|------|------|---------------|
| 10 | deprov_unsubscribe_webhook | sns:Unsubscribe |
| 11 | deprov_delete_event_destination | ses:DeleteConfigurationSetEventDestination |
| 12 | deprov_delete_sns_topic | sns:DeleteTopic |
| 13 | deprov_delete_configuration_set | ses:DeleteConfigurationSet |

### AWS IAM Permissions (full lifecycle)

**Sending (required)**:
- `ses:SendEmail`, `ses:SendRawEmail` -- Send emails
- `ses:ListEmailIdentities` -- Sync verified sender identities
- `ses:GetAccount` -- Check sandbox/production status

**Event Tracking Provisioning (required if `SENDA_TRACKING_BASE_URL` set)**:
- `ses:CreateConfigurationSet` -- Create tracking ConfigSet
- `ses:CreateConfigurationSetEventDestination` -- Link ConfigSet to SNS
- `ses:ListConfigurationSets` -- List existing ConfigSets
- `sns:CreateTopic` -- Create SNS Topic
- `sns:Subscribe` -- Subscribe webhook
- `sns:GetSubscriptionAttributes` -- Verify subscription confirmed
- `sns:ListTopics` -- List existing Topics

**Cleanup on Adapter Deletion (recommended)**:
- `ses:DeleteConfigurationSet` -- Remove ConfigSet on adapter delete
- `ses:DeleteConfigurationSetEventDestination` -- Remove EventDest on adapter delete
- `sns:Unsubscribe` -- Cancel subscription on adapter delete
- `sns:DeleteTopic` -- Remove Topic on adapter delete

Permissions validated by `ValidateCredentials()` (`validator.go`) -- cleanup checks are non-blocking.
