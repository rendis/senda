# Audit Log, Global Config & Extras

Audit log queries, the global configuration endpoints, suppression list, and
the few public utilities (tracking pixel, media proxy) live here. Some
overlap with `webhooks-and-events.md` for tracking and SES; this file owns
the rest.

## Audit log

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `<ws>/audit-log` and `<envWS>/audit-log` | workspace_viewer+ | Cursor pagination. Filters: `cursor`, `limit`, `actor_id` (UUID), `action`, `entity_type`, `since`, `until` (RFC3339). Scope fixed to the workspace + tenant of the path. |
| GET | `/api/v1/manage/global/audit-log` | superadmin | Same filters, no scope filter. |

Each entry typically carries `actor_id`, `action`, `entity_type`, `entity_id`,
`tenant_id`, `workspace_id`, `created_at`, and a small `details` JSON.

Common `action` values include `tenant.create`, `workspace.update`,
`adapter.create`, `adapter.workspace_access.set`, `template.fork`,
`version.publish`, `apikey.create`, `apikey.revoke`, `webhook.create`,
`webhook.test`, `policy.update`, `runtime.reset`. Use `senda_describe_endpoint`
or query for distinct values to learn the catalog of your deployment.

## Global config

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `/api/v1/manage/config` | superadmin | Returns `email_defaults`, `alerts`, `domain`, `external_integrations.profiles[]`, OIDC info (`discovery_url`, `client_id`, `client_secret_set`). |
| PUT | `/api/v1/manage/config` | superadmin | Patch parcial top-level: any of `email_defaults?`, `alerts?`, `domain?`, `external_integrations?`. The `external_integrations` body is an object `{profiles: [...]}` and **fully replaces** the profile list (no merge). OIDC fields are read-only here. |

### Sections

- **`email_defaults`** — `max_retries`, `backoff_base_seconds`,
  `log_retention_days`.
- **`alerts`** — `bounce_threshold_percent`, `complaint_threshold_percent`.
- **`domain`** — `recheck_interval_hours`.
- **`external_integrations`** — `{profiles: [...]}`. Each profile shape in
  `external-integration.md`.

## Suppression

Suppression has **two layers** that are checked together:

- **Workspace** (`suppression_workspace`): added explicitly via
  `POST <ws>/suppression`, OR automatically when a complaint event is
  processed for the workspace. Scoped to that workspace only.
- **Global** (`suppression_global`): added automatically **only** when a
  hard-bounce event is processed for any tenant. A single hard bounce
  therefore suppresses the address across every workspace in the
  deployment. Complaints do NOT propagate to the global layer.

`IsSuppressed(ctx, ws_id, email)` and `GetSuppressionStatuses` query both
tables and return suppressed = true if either matches. Operators planning
"unsuppress" flows need to be aware of the global layer.

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST | `<ws>/suppression` and `<envWS>/suppression` | workspace_admin | `{email, reason? (manual|hard_bounce|complaint), notes?}`. Returns 201. |
| GET | `<ws>/suppression/:email` and `<envWS>/suppression/:email` | workspace_viewer+ | Returns `{email, suppressed: bool, reason?}`. |
| DELETE | `<ws>/suppression/:email` and `<envWS>/suppression/:email` | **superadmin only** (handler-level extra check) | `?reason=...` optional. |

Suppressed addresses are skipped silently with `SUPPRESSED` in the send response.

## Public extras

### Tracking pixel — `GET /t/o/:tracking_id`

- Public endpoint.
- Returns a 1×1 transparent GIF with `Cache-Control: no-store`.
- Records an `email.opened` event asynchronously; 30s dedup window per
  tracking ID.

### Media thumbnail proxy — `GET /public/video-thumbnail`

- Public endpoint.
- `?url=<https://>` (only HTTP/HTTPS).
- SSRF-protected: blocks private/loopback ranges.
- Allowlist of hosts (default `img.youtube.com`, `i.ytimg.com`).
- Composes a play-button overlay and returns PNG.
- 24h in-memory cache, max 500 entries.

### Health & metrics

- `GET /health` — public; returns a static `{"status":"healthy"}`.
- `GET /healthz` — public; runs DB ping + River health check; 200 or 503.
- `GET /metrics` — Prometheus exposition; protected by static bearer
  (`Authorization: Bearer <metrics_token>`). Only registered when
  `config.server.metrics_token != ""`.

## Operational rules

- **Audit retention** is governed by `email_defaults.log_retention_days`
  unless your operator overrides it elsewhere.
- **Bounce/complaint alerts** fire when the rolling rate exceeds the
  configured percent. The dispatch path is operator-defined (often via the
  outbound webhook system).
- **Suppression DELETE** is intentionally restricted: it requires `superadmin`
  even though the route registration says `workspace_admin`. The handler
  enforces it.
- **Config PUT** for `external_integrations` is replace-all; read-modify-write
  carefully if multiple profiles already exist.
- **OIDC fields** are not editable through this endpoint. Change them via
  config files / env vars and restart the server.

## Flujo end-to-end — produce an audit/incident report

1. `senda_call_endpoint GET /api/v1/manage/global/audit-log?since=...&until=...`
   for the deployment-wide view.
2. Filter by `actor_id`, `action`, or `entity_type` to narrow down.
3. Cross-reference with `GET /api/v1/emails` (data plane or `<ws>/emails`)
   when the audit entry references send / template events.

## Cuándo consultar OpenAPI / MCP

- Audit log entry shape (`details` JSON varies per action).
- Global config response (especially the OIDC and `domain` blocks).

## Gotchas

- **`external_integrations` is replace-all**. To add one profile, fetch the
  current list (`GET /config`), append to `external_integrations.profiles`,
  and PUT the whole `{external_integrations: {profiles: [...]}}` object.
  Sending a bare array fails with 400 `BAD_REQUEST`.
- **Suppression DELETE is superadmin-only** at the handler level — surprises
  workspace admins who can ADD entries but cannot remove them.
- **Health endpoints are unauthenticated**; do not place sensitive info in
  any future extension to them.
- **Metrics endpoint** uses a static token; rotate it via config and restart.
- **Tracking pixel and media proxy are public**; expect crawlers and don't
  rely on header-based gating.
