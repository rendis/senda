# API Keys & Data Plane

API keys authenticate machine-to-machine sends and email queries. The data
plane is small and stable.

## API key model

- Stored hashed (HMAC-SHA256 with a server pepper); the raw value is
  shown ONLY in the creation response.
- Bound to one workspace environment. Token prefix decides everything:
  - `senda_prod_<random>` ⇒ env=prod, workspace = the prod row.
  - `senda_test_<random>` ⇒ env=test, workspace = the test row.
- Bypass RBAC. Only the data plane and `/api/v1/emails/...` accept them.
- Cannot be rotated in place; revoke + create a new one.

## Endpoints

### Management — API keys (`<ws>` and `<envWS>`)

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST | `<ws>/api-keys` | workspace_admin | `{name}` (max 100). Returns `{id, key, name, hint, created_at, created_by}`. **`key` is the raw value and is shown ONCE.** No `prefix` field; the prefix is part of `key` itself (`senda_prod_…` / `senda_test_…`). |
| GET | `<ws>/api-keys` | workspace_admin | Cursor pagination. Items: `{id, name, hint, created_at, created_by, last_used_at?, revoked_at?}`. Never returns the raw key or its hash. |
| DELETE | `<ws>/api-keys/:id` | workspace_admin | Hard revoke (sets `revoked_at`). 204. |

### Data plane — send

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/send` | API key | Body: `{ref, to: string[], cc?: string[], bcc?: string[], variables?, injectors?, external_id?, locale?}`. `to` is a flat list of email strings (no `{address, name}` objects), **max 50 entries** per call — sending more returns 422 `VALIDATION_ERROR`. For larger fan-out use `/send/batch`. 202 `{status, tracking_ids: [{to, tracking_id, status, error?}], external_id?, template_resolved, template_version}`. |
| POST | `/api/v1/send/batch` | API key | Body: `{ref, items: [{to: string, cc?: string[], bcc?: string[], variables?, injectors?, external_id?, locale?}]}` — note `to` is a single string per item. Up to 100 items by default. 202 `{status, template_resolved, items: [{index, to, tracking_id?, status, external_id?, error?}], accepted_count, suppressed_count, failed_count}`. |

### Data plane — emails query

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/api/v1/emails` | API key | Cursor pagination. Filters: `external_id`, `recipient`, `status`, `template_type`, `adapter_id`, `since`, `until` (RFC3339), `limit` (def 25, max 100). |
| GET | `/api/v1/emails/export` | API key | Streams CSV. Same filters. Full data, no row cap. |
| GET | `/api/v1/emails/:tracking_id` | API key | Email + events inline. |
| GET | `/api/v1/emails/:tracking_id/events` | API key | Events only. |

There is also a management mirror under `<ws>/emails[/...]` (OIDC, RBAC =
workspace_viewer+). The fields are the same.

## `ref` format

```
tenant_code:workspace_code:template_type_slug
```

- All three components required.
- **Never** put environment in `ref`. Environment comes from the API key
  prefix.
- Token prefix decides the environment **before** Senda resolves the `ref`.
  If the workspace resolved from `ref` does not match the API key's own
  workspace, the request fails with 403 `FORBIDDEN` and message
  "API key scope does not match template workspace". A literal cross-env
  attempt (e.g. test key against a prod-only workspace UUID) ends in the
  same place because the test environment cannot resolve a prod workspace.
- `_system` as `workspace_code` → 422 `SYSTEM_WORKSPACE_BLOCKED`.

## Send error catalog

| HTTP | Code | Cause |
|---|---|---|
| 422 | `NO_ADAPTER` | Template type has no `adapter_id`. |
| 422 | `NO_DEFAULT_IDENTITY` | Adapter has no default identity (or the bound `sender_identity_id` is missing). |
| 422 | `DOMAIN_NOT_VERIFIED` | Selected identity is not in `verified` state. |
| 422 | `TEST_RECIPIENT_POLICY_UNCONFIGURED` | env=test send without recipient policy. |
| 422 | `SUPPRESSED` | Reserved error for explicit suppression rejection. The normal `POST /send` path does NOT return this — suppressed recipients are accepted with a 202 and a per-recipient `tracking_ids[].status = "suppressed"` (and batch reports `suppressed_count`). Treat this code as a defensive case, not the standard suppression signal. |
| 422 | `NO_PUBLISHED_VERSION` | Template has no `published` version. |
| 422 | `SYSTEM_WORKSPACE_BLOCKED` | `_system` is not allowed to receive sends. |
| 409 | `TEMPLATE_DISABLED` | Template `is_disabled = true`. |
| 403 | `FORBIDDEN` | API key's workspace does not match the workspace resolved from `ref` — message: "API key scope does not match template workspace". |
| 429 | `RATE_LIMITED` | Adapter rate limit hit (`rate_limit_per_second`). |
| 400 | `BAD_REQUEST` | Malformed request body or path. |
| 422 | `VALIDATION_ERROR` | Field-level validation failed. `details[]` lists `{field, message}` per failing field. |

## Operational rules

- **Recipient policy in env=test**: required. Configure on the test
  workspace (`test_recipient_mode`, `test_recipient_addresses`) or on the
  template type. `replace` swaps recipients with the safe list; `append`
  adds them. See `management-tenants-workspaces.md`.
- **Suppression**: per-workspace list (see `audit-config-and-extras.md`).
  Suppressed recipients are dropped silently. The single-send response
  reflects the per-recipient outcome inside `tracking_ids[].status` (e.g.
  `suppressed`); the batch response also exposes `suppressed_count`.
- **Open tracking** is per-workspace (`open_tracking_enabled` on the
  env-scoped row). When on, sends rewrite links and embed the 1×1 pixel.
  See `webhooks-and-events.md`.
- **`external_id`** is the caller-side correlation id. Indexed; query
  efficiently with `?external_id=...`.
- **`tracking_id`** is server-issued (UUIDv7); use it to follow a specific
  message. `POST /send` returns one `tracking_ids` entry **per `to`
  recipient only**. CC and BCC addresses are stored on each generated
  email record (`email.cc`, `email.bcc`) but do NOT receive their own
  tracking IDs. `POST /send/batch` returns one `items[]` entry per item
  (which carries the `to` for that item).

## Flujo end-to-end

1. As `workspace_admin` (OIDC): `POST <ws>/api-keys` and **store the raw key now**.
2. From the backend service, `POST /api/v1/send` with the key.
3. Receive 202 with `tracking_id`.
4. (Optional) `GET /api/v1/emails/:tracking_id/events` to follow lifecycle.
5. Bulk: `POST /api/v1/send/batch` with up to 100 items.
6. Reporting / export: `GET /api/v1/emails/export?since=...&until=...`.

## Cuándo consultar OpenAPI / MCP

- Exact `to` shape (`"email@x"` vs `{address, name}`) for both `POST /send`
  and items in batch.
- Filter list and types for `GET /emails`.

## Gotchas

- **The raw API key is shown once.** If the response is lost, revoke and
  rotate; there is no recovery path.
- **`ref` is brittle**: misspell the slug → 404 (or unexpected resolution
  via `_system`). Prefer hardcoded constants in the consumer.
- **Cross-environment**: a prod API key against a test workspace is not a
  graceful no-op — it is a 403 `FORBIDDEN` ("API key scope mismatch"). The
  generic `FORBIDDEN` code is shared with other RBAC denials, so disambiguate
  on the message string. Make sure your service
  selects the right key per environment.
- **Rate limit** is per adapter, not per key. A burst can stall multiple
  workspaces sharing one adapter.
- **CSV export streams** indefinitely; client must read the body fully.
- **Management `/emails` mirror** is for human operators and dashboards;
  prefer the data plane for backend integrations (no OIDC needed).
- **`POST /send` returns 202** even when nothing was actually queued (e.g.
  all recipients suppressed). Inspect each entry in `tracking_ids[].status`
  before assuming a delivery is in flight. For batch, also check
  `accepted_count` / `suppressed_count` / `failed_count`.
