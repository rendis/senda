# Senda API Reference

Complete API reference for Senda v1. All endpoints under `/api/v1/` unless noted. Request and response bodies use `application/json`. For an interactive collection, see [docs/postman/](postman/).

---

## Authentication

| Scheme     | Header                                 | Plane      | Scope                                      |
| ---------- | -------------------------------------- | ---------- | ------------------------------------------ |
| OIDC / JWT | `Authorization: Bearer <oidc_token>`   | Management | User identity, RBAC role determines access |
| API Key    | `Authorization: ApiKey senda_live_xxx` | Data       | Workspace-scoped, used by applications     |

API keys are scoped to a single workspace. Management tokens carry the member's RBAC role evaluated against the resource hierarchy.

### RBAC Roles (highest to lowest)

| Role            | Description                                                       |
| --------------- | ----------------------------------------------------------------- |
| Superadmin      | Full platform access. Manages tenants, global resources, members. |
| TenantAdmin     | Manages a tenant and all its workspaces.                          |
| WorkspaceAdmin  | Full control over a single workspace.                             |
| WorkspaceEditor | Create and modify resources within a workspace.                   |
| WorkspaceViewer | Read-only access to workspace resources.                          |

---

## Error Format

All errors use a consistent JSON structure via `pkg/apperr`:

```json
{ "code": 422, "message": "template_type is required" }
```

### Error Codes

| Status | Meaning               | Common Causes                                                 |
| ------ | --------------------- | ------------------------------------------------------------- |
| 400    | Bad Request           | Malformed JSON, missing required fields, invalid query params |
| 401    | Unauthorized          | Missing/expired token, invalid API key                        |
| 403    | Forbidden             | Insufficient RBAC role                                        |
| 404    | Not Found             | Resource does not exist or is soft-deleted                    |
| 409    | Conflict              | Duplicate resource (unique code collision)                    |
| 422    | Unprocessable Entity  | Validation failure on semantically invalid input              |
| 429    | Too Many Requests     | Rate limit exceeded (token-bucket, per workspace)             |
| 500    | Internal Server Error | Unexpected server-side failure                                |

---

## Pagination

All list endpoints use cursor-based pagination. Cursors are opaque UUIDv7 strings.

| Parameter | Type    | Default  | Description                                     |
| --------- | ------- | -------- | ----------------------------------------------- |
| `cursor`  | string  | _(none)_ | Cursor from a previous response's `next_cursor` |
| `limit`   | integer | 20       | Items per page (max varies by endpoint)         |

```json
{ "items": [ ... ], "next_cursor": "019012ab-..." }
```

When `next_cursor` is `null` or absent, there are no more pages.

---

## Infrastructure Endpoints (No Auth)

| Method | Path       | Description                        |
| ------ | ---------- | ---------------------------------- |
| GET    | `/health`  | Basic application status           |
| GET    | `/healthz` | Database connectivity check        |
| GET    | `/metrics` | Prometheus metrics scrape endpoint |

---

## Public Endpoints (No Auth)

| Method | Path                           | Description                                      |
| ------ | ------------------------------ | ------------------------------------------------ |
| GET    | `/t/o/:tracking_id`            | Open-tracking pixel (1x1 transparent GIF)        |
| GET    | `/public/video-thumbnail`      | Video thumbnail composite image                  |
| POST   | `/api/v1/webhooks/ses/inbound` | AWS SES inbound webhook (SNS signature verified) |

---

## Onboarding

| Method | Path                        | Auth | Description                         |
| ------ | --------------------------- | ---- | ----------------------------------- |
| GET    | `/api/v1/onboarding/status` | None | Current onboarding state            |
| POST   | `/api/v1/onboarding/setup`  | OIDC | Initialize the platform (first-run) |

---

## Data Plane (API Key Auth)

All requests require `Authorization: ApiKey senda_live_xxx`. The key implicitly scopes operations to its workspace.

### POST /api/v1/send

| Field           | Type     | Required | Description                                                  |
| --------------- | -------- | -------- | ------------------------------------------------------------ |
| `to`            | string   | yes      | Recipient email address                                      |
| `template_type` | string   | yes      | Slug in `tenant:workspace:slug` format                       |
| `variables`     | object   | no       | Key-value pairs for template interpolation                   |
| `locale`        | string   | no       | BCP-47 locale (e.g., `en`, `es-419`). Falls back to default. |
| `cc`            | string[] | no       | CC recipients                                                |
| `bcc`           | string[] | no       | BCC recipients                                               |
| `external_id`   | string   | no       | Caller-provided idempotency/correlation ID                   |

**Response (202):**

```json
{ "tracking_id": "019012ab-7c3d-7def-8abc-1234567890ab", "status": "queued" }
```

**Full curl example:**

```bash
curl -X POST https://senda.example.com/api/v1/send \
  -H "Content-Type: application/json" \
  -H "Authorization: ApiKey senda_live_sk_abc123def456" \
  -d '{
    "to": "user@example.com",
    "template_type": "acme:production:welcome-email",
    "variables": {
      "first_name": "Jane",
      "activation_url": "https://app.example.com/activate?token=xyz"
    },
    "locale": "en",
    "external_id": "signup-jane-20260226"
  }'
```

### Query Emails

| Method | Path                                 | Description                               |
| ------ | ------------------------------------ | ----------------------------------------- |
| GET    | `/api/v1/emails`                     | List emails (paginated, workspace-scoped) |
| GET    | `/api/v1/emails/export`              | Export emails                             |
| GET    | `/api/v1/emails/:tracking_id`        | Get email by tracking ID                  |
| GET    | `/api/v1/emails/:tracking_id/events` | Get delivery events                       |

---

## Member Profile (OIDC Auth)

| Method | Path                 | Description            |
| ------ | -------------------- | ---------------------- |
| GET    | `/api/v1/members/me` | Current member profile |

---

## Management API (OIDC Auth, RBAC Enforced)

All management endpoints live under `/api/v1/manage/`. The path hierarchy mirrors the resource hierarchy: global, tenant, workspace.

### Tenants (Superadmin)

Base: `/api/v1/manage/tenants`

| Method | Path                    | Description        |
| ------ | ----------------------- | ------------------ |
| POST   | `/tenants`              | Create tenant      |
| GET    | `/tenants`              | List tenants       |
| GET    | `/tenants/:tenant_code` | Get tenant         |
| PUT    | `/tenants/:tenant_code` | Update tenant      |
| DELETE | `/tenants/:tenant_code` | Soft-delete tenant |

### Workspaces (TenantAdmin+)

Base: `/api/v1/manage/tenants/:tenant_code/workspaces`

| Method | Path                          | Description           |
| ------ | ----------------------------- | --------------------- |
| POST   | `/workspaces`                 | Create workspace      |
| GET    | `/workspaces`                 | List workspaces       |
| GET    | `/workspaces/:workspace_code` | Get workspace         |
| PUT    | `/workspaces/:workspace_code` | Update workspace      |
| DELETE | `/workspaces/:workspace_code` | Soft-delete workspace |

### Members (Superadmin)

Base: `/api/v1/manage/members`

| Method | Path                          | Description   |
| ------ | ----------------------------- | ------------- |
| POST   | `/members`                    | Create member |
| GET    | `/members`                    | List members  |
| GET    | `/members/:id`                | Get member    |
| POST   | `/members/:id/roles`          | Assign role   |
| DELETE | `/members/:id/roles/:role_id` | Remove role   |

### Platform Config (Superadmin)

| Method | Path                    | Description            |
| ------ | ----------------------- | ---------------------- |
| GET    | `/api/v1/manage/config` | Get platform config    |
| PUT    | `/api/v1/manage/config` | Update platform config |

---

### Workspace-Scoped Resources

Base: `/api/v1/manage/tenants/:tc/workspaces/:wc`

Required role varies by operation (typically WorkspaceEditor+ for writes, WorkspaceViewer+ for reads).

#### Injectors

| Method | Path                    | Description            |
| ------ | ----------------------- | ---------------------- |
| POST   | `/injectors`            | Create injector        |
| GET    | `/injectors`            | List injectors         |
| GET    | `/injectors/:id`        | Get injector           |
| PUT    | `/injectors/:id/values` | Update injector values |

#### Adapters

| Method | Path                                    | Description                    |
| ------ | --------------------------------------- | ------------------------------ |
| POST   | `/adapters`                             | Create adapter                 |
| GET    | `/adapters`                             | List adapters                  |
| GET    | `/adapters/:id`                         | Get adapter                    |
| PUT    | `/adapters/:id`                         | Update adapter                 |
| DELETE | `/adapters/:id`                         | Soft-delete adapter            |
| POST   | `/adapters/:id/test`                    | Test connectivity              |
| GET    | `/adapters/:id/setup-guide`             | Provider setup instructions    |
| POST   | `/adapters/:id/auto-provision-tracking` | Auto-provision tracking domain |

#### Adapter Identities

| Method | Path                                        | Description        |
| ------ | ------------------------------------------- | ------------------ |
| GET    | `/adapters/:id/identities`                  | List identities    |
| POST   | `/adapters/:id/identities`                  | Create identity    |
| POST   | `/adapters/:id/identities/sync`             | Sync from provider |
| DELETE | `/adapters/:id/identities/:iid`             | Delete identity    |
| POST   | `/adapters/:id/identities/:iid/set-default` | Set as default     |

#### Template Types

| Method | Path                  | Description          |
| ------ | --------------------- | -------------------- |
| POST   | `/template-types`     | Create template type |
| GET    | `/template-types`     | List template types  |
| GET    | `/template-types/:id` | Get template type    |

#### Templates

| Method | Path                     | Description      |
| ------ | ------------------------ | ---------------- |
| POST   | `/templates`             | Create template  |
| GET    | `/templates`             | List templates   |
| GET    | `/templates/:id`         | Get template     |
| POST   | `/templates/:id/disable` | Disable template |
| POST   | `/templates/:id/enable`  | Enable template  |

#### Template Versions

| Method | Path                                   | Description          |
| ------ | -------------------------------------- | -------------------- |
| POST   | `/templates/:id/versions`              | Create version       |
| GET    | `/templates/:id/versions`              | List versions        |
| GET    | `/templates/:id/versions/:vid`         | Get version          |
| PUT    | `/templates/:id/versions/:vid`         | Update draft version |
| POST   | `/templates/:id/versions/:vid/publish` | Publish version      |

#### Template Version Locales

| Method | Path                                           | Description   |
| ------ | ---------------------------------------------- | ------------- |
| POST   | `/templates/:id/versions/:vid/locales`         | Add locale    |
| GET    | `/templates/:id/versions/:vid/locales`         | List locales  |
| PUT    | `/templates/:id/versions/:vid/locales/:locale` | Update locale |
| DELETE | `/templates/:id/versions/:vid/locales/:locale` | Remove locale |

#### Template Preview & Test

| Method | Path                       | Description                   |
| ------ | -------------------------- | ----------------------------- |
| POST   | `/templates/:id/preview`   | Render preview (returns HTML) |
| POST   | `/templates/:id/test-send` | Send test email               |

#### API Keys

| Method | Path                   | Description    |
| ------ | ---------------------- | -------------- |
| POST   | `/api-keys`            | Create API key |
| GET    | `/api-keys`            | List API keys  |
| DELETE | `/api-keys/:id/revoke` | Revoke API key |

#### Webhooks

| Method | Path                 | Description     |
| ------ | -------------------- | --------------- |
| POST   | `/webhooks`          | Create webhook  |
| GET    | `/webhooks`          | List webhooks   |
| GET    | `/webhooks/:id`      | Get webhook     |
| PUT    | `/webhooks/:id`      | Update webhook  |
| DELETE | `/webhooks/:id`      | Delete webhook  |
| POST   | `/webhooks/:id/test` | Send test event |

#### Emails (Management View)

| Method | Path                          | Description                      |
| ------ | ----------------------------- | -------------------------------- |
| GET    | `/emails`                     | List emails (management filters) |
| GET    | `/emails/:tracking_id`        | Get email                        |
| GET    | `/emails/:tracking_id/events` | Get delivery events              |

#### Suppression List

| Method | Path                  | Description                          |
| ------ | --------------------- | ------------------------------------ |
| POST   | `/suppression/add`    | Add address to suppression list      |
| POST   | `/suppression/check`  | Check if address is suppressed       |
| POST   | `/suppression/remove` | Remove address from suppression list |

#### Audit Log

| Method | Path         | Description                               |
| ------ | ------------ | ----------------------------------------- |
| GET    | `/audit-log` | Query audit entries (paginated, filtered) |

#### Dashboard

| Method | Path               | Description                            |
| ------ | ------------------ | -------------------------------------- |
| GET    | `/dashboard/stats` | Workspace email statistics and metrics |

---

### Global-Scoped Resources (Superadmin)

Base: `/api/v1/manage/global`

Global resources mirror the workspace-scoped structure but apply at the platform level. Only Superadmins have access. Global resources participate in the resolution chain and can be inherited by tenants and workspaces.

| Resource           | Endpoints                                       |
| ------------------ | ----------------------------------------------- |
| Injectors          | CRUD + values (same as workspace-scoped)        |
| Adapters           | CRUD + test + setup-guide + auto-provision      |
| Adapter Identities | List, create, sync, delete, set-default         |
| Template Types     | CRUD                                            |
| Templates          | CRUD + versions + locales + preview + test-send |
| Audit Log          | GET with filters                                |
| Dashboard          | GET stats                                       |

---

## Webhook Events

Senda delivers real-time event notifications to configured webhook URLs.

| Event              | Description                                       |
| ------------------ | ------------------------------------------------- |
| `email.queued`     | Email accepted and placed in the send queue       |
| `email.sent`       | Email handed off to the provider                  |
| `email.delivered`  | Provider confirmed delivery to recipient MTA      |
| `email.bounced`    | Email bounced (hard or soft)                      |
| `email.complained` | Recipient marked the email as spam                |
| `email.failed`     | Permanent send failure                            |
| `email.opened`     | Recipient opened the email (tracking pixel fired) |
| `*`                | Wildcard, subscribes to all events                |

### Signature Verification

Every webhook request includes an HMAC-SHA256 signature in `X-Senda-Signature`, computed over the raw request body using the webhook's secret key.

```
X-Senda-Signature: sha256=<hex_digest>
```

Verification example:

```python
import hmac, hashlib

def verify(body: bytes, secret: str, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

---

## Rate Limiting

The data plane enforces token-bucket rate limiting per workspace. When exceeded, the API returns `429 Too Many Requests`. Retry after the period indicated in the `Retry-After` response header.

---

## Postman Collection

An importable Postman collection covering all endpoints is available at `docs/postman/senda-api-v1.postman_collection.json`.
