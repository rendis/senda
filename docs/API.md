
# Senda API Reference

Complete API reference for Senda v1. All JSON endpoints live under `/api/v1/` unless noted.

## Authentication

| Scheme | Header | Plane | Notes |
| --- | --- | --- | --- |
| OIDC / JWT | `Authorization: Bearer <oidc_token>` | Management | Human/member identity with RBAC |
| Workspace API key | `Authorization: Bearer senda_prod_...` or `senda_test_...` | Data plane | Raw key selects workspace environment |
| External integration | Custom per profile + `X-Senda-Environment` | External surface | Uses registered auth method + workspace resolver |

Important rules:

- Use the environment-aware workspace API key prefixes documented here.
- Environment is **not** part of the send `ref`.
- External integration requests must include `X-Senda-Environment: prod|test`.

## Error format

Most endpoints use:

```json
{ "code": 422, "message": "validation failed" }
```

Validation errors can include field details.

## Pagination

List endpoints use cursor-based pagination with `cursor` and `limit`. When present, `next_cursor` can be supplied to fetch the next page.

## Public / unauthenticated endpoints

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Basic health |
| GET | `/healthz` | Readiness / deeper health |
| GET | `/metrics` | Prometheus metrics |
| GET | `/t/o/:tracking_id` | Open-tracking pixel |
| GET | `/public/video-thumbnail` | Video thumbnail composite |
| GET | `/api/v1/onboarding/status` | Onboarding status |
| POST | `/api/v1/webhooks/ses/inbound` | SNS-signed SES event ingestion |

## Data plane

Data-plane requests require a raw workspace API key.

Environment comes from the key prefix:

- `senda_prod_...`
- `senda_test_...`

The send ref format stays:

`tenant_code:workspace_code:template_type_slug`

### Send

| Method | Path | Description |
| --- | --- | --- |
| POST | `/api/v1/send` | Queue one logical send request |
| POST | `/api/v1/send/batch` | Queue multiple same-template items with independent variables/injectors |

### Email query

| Method | Path | Description |
| --- | --- | --- |
| GET | `/api/v1/emails` | List workspace emails |
| GET | `/api/v1/emails/export` | Export emails |
| GET | `/api/v1/emails/:tracking_id` | Get one email |
| GET | `/api/v1/emails/:tracking_id/events` | Get lifecycle events |

## Member profile

| Method | Path | Auth |
| --- | --- | --- |
| GET | `/api/v1/members/me` | OIDC |

## Management API

Management requests require OIDC and RBAC. There are two workspace route families:

1. **shared logical workspace routes** — identity and logical CRUD
2. **environment-scoped workspace routes** — environment-specific runtime/resources

### Tenants

Base: `/api/v1/manage/tenants`

| Method | Path |
| --- | --- |
| POST | `/tenants` |
| GET | `/tenants` |
| GET | `/tenants/:tenant_code` |
| PUT | `/tenants/:tenant_code` |
| DELETE | `/tenants/:tenant_code` |

### Workspaces (logical/shared)

Base: `/api/v1/manage/tenants/:tenant_code/workspaces`

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/workspaces` | Create logical workspace pair (`prod` + `test`) |
| GET | `/workspaces` | List logical workspaces |
| GET | `/workspaces/:workspace_code` | Get shared workspace identity |
| PUT | `/workspaces/:workspace_code` | Update shared logical fields |
| DELETE | `/workspaces/:workspace_code` | Soft-delete logical pair |

### Workspaces (environment-scoped)

Base: `/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code`

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `` | Get environment-specific workspace state |
| PUT | `` | Update environment-specific workspace state |
| POST | `/runtime/reset` | Test-only runtime reset |
| GET | `/policies` | Get workspace policies |
| PUT | `/policies` | Update workspace policies |

Runtime reset is only valid for `environment=test`.

### Members

| Method | Path |
| --- | --- |
| GET | `/api/v1/manage/members` |
| POST | `/api/v1/manage/members` |
| GET | `/api/v1/manage/members/:member_id` |
| DELETE | `/api/v1/manage/members/:member_id/access` |
| PUT | `/api/v1/manage/members/:member_id/role` |
| POST | `/api/v1/manage/members/:member_id/roles` |
| DELETE | `/api/v1/manage/members/:member_id/roles/:role_id` |

Tenant and workspace member routes also exist under the respective tenant/workspace paths.

Scoped access revocation routes:

| Method | Path |
| --- | --- |
| DELETE | `/api/v1/manage/tenants/:tenant_code/members/:member_id/access` |
| DELETE | `/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/access` |
| DELETE | `/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/access` |

These DELETE routes revoke all role assignments for the member in the selected scope. They do not delete the member identity.

Scoped role replacement routes:

| Method | Path |
| --- | --- |
| PUT | `/api/v1/manage/tenants/:tenant_code/members/:member_id/role` |
| PUT | `/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/role` |
| PUT | `/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code/members/:member_id/role` |

These PUT routes enforce a single local role assignment per scope. They create the scoped assignment when missing, replace the existing local assignment when it differs, and return `200 OK` idempotently when the member already has that local role. Legacy `POST .../roles` / `DELETE .../roles/:role_id` routes remain temporarily for backward compatibility, but they no longer allow multiple local roles within the same scope.

### Config

| Method | Path |
| --- | --- |
| GET | `/api/v1/manage/config` |
| PUT | `/api/v1/manage/config` |

Global config includes external integration profiles.

## Workspace-scoped resources

The same resource families are available under both:

- shared workspace base: `/api/v1/manage/tenants/:tenant_code/workspaces/:workspace_code`
- environment workspace base: `/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code`

For runtime resources, use the environment-scoped base.

### Injectors

| Method | Path |
| --- | --- |
| POST | `/injectors` |
| GET | `/injectors` |
| GET | `/injectors/:name` |
| PUT | `/injectors/:name` |
| PUT | `/injectors/:name/fields/:field_name` |
| PUT | `/injectors/:name/values` |
| DELETE | `/injectors/:name` |

### Adapters and identities

| Method | Path |
| --- | --- |
| GET | `/adapters` |
| POST | `/adapters` |
| POST | `/adapters/validate-ses` |
| GET | `/adapters/:id` |
| PUT | `/adapters/:id` |
| DELETE | `/adapters/:id` |
| POST | `/adapters/:id/test` |
| GET | `/adapters/:id/workspace-access` |
| PUT | `/adapters/:id/workspace-access` |
| GET | `/adapters/:id/identities` |
| POST | `/adapters/:id/identities` |
| POST | `/adapters/:id/identities/sync` |
| DELETE | `/adapters/:id/identities/:identity_id` |
| POST | `/adapters/:id/identities/:identity_id/set-default` |
| GET | `/adapters/:id/identities/:identity_id/workspace-access` |
| PUT | `/adapters/:id/identities/:identity_id/workspace-access` |

Sharing rules:

- manage sharing from tenant `_system`
- Gmail sharing is adapter-level
- SES sharing is email-identity-level
- SES domain identities are not shareable
- shared child-workspace entries are read-only

### Template types

| Method | Path |
| --- | --- |
| POST | `/template-types` |
| GET | `/template-types` |
| GET | `/template-types/:slug` |
| PUT | `/template-types/:slug` |
| DELETE | `/template-types/:slug` |

Test-only template-type controls:

- `test_recipient_mode`
- `test_recipient_addresses`

These are only valid in the `test` environment.

### Templates, versions, locales

| Method | Path |
| --- | --- |
| GET | `/template-types/:slug/templates` |
| POST | `/templates` |
| POST | `/templates/:template_id/fork` |
| GET | `/templates/:template_id/versions` |
| GET | `/templates/:template_id/versions/:version_id` |
| POST | `/templates/:template_id/versions` |
| PUT | `/templates/:template_id/versions/:version_id` |
| POST | `/templates/:template_id/versions/:version_id/clone` |
| POST | `/templates/:template_id/versions/:version_id/publish` |
| GET | `/templates/:template_id/versions/:version_id/locales` |
| GET | `/templates/:template_id/versions/:version_id/locales/:locale` |
| POST | `/templates/:template_id/versions/:version_id/locales/:locale` |
| PUT | `/templates/:template_id/versions/:version_id/locales/:locale` |
| DELETE | `/templates/:template_id/versions/:version_id/locales/:locale` |
| POST | `/templates/:template_id/preview-mjml` |
| POST | `/templates/:template_id/test-send` |
| GET | `/templates/:template_id/bulk-send-config` |
| POST | `/templates/:template_id/bulk-send` |
| POST | `/templates/:template_id/disable` |
| POST | `/templates/:template_id/enable` |
| DELETE | `/templates/:template_id` |
| DELETE | `/templates/:template_id/versions/:version_id` |

Notes:

- `clone` creates an exact draft copy of one version and all its locales.
- `fork` copies inherited template behavior into local workspace ownership.

### API keys

| Method | Path |
| --- | --- |
| POST | `/api-keys` |
| GET | `/api-keys` |
| DELETE | `/api-keys/:id` |

Created keys return the raw key only once and use the environment-aware prefix for that workspace environment.

### Webhooks

| Method | Path |
| --- | --- |
| POST | `/webhooks` |
| GET | `/webhooks` |
| GET | `/webhooks/:id` |
| PUT | `/webhooks/:id` |
| DELETE | `/webhooks/:id` |
| POST | `/webhooks/:id/test` |

### Emails / suppression / audit / dashboard

| Method | Path |
| --- | --- |
| GET | `/emails` |
| GET | `/emails/:tracking_id` |
| GET | `/emails/:tracking_id/events` |
| POST | `/suppression` |
| GET | `/suppression/:email` |
| DELETE | `/suppression/:email` |
| GET | `/audit-log` |
| GET | `/dashboard-stats` |

## External integration surface

Base: `/api/v1/external/:profile_slug`

### Public bootstrap

| Method | Path |
| --- | --- |
| GET | `/bootstrap` |
| GET | `/environments/:environment/bootstrap` |

### Authenticated external workspace surface

Base:

`/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code`

Required request header:

```http
X-Senda-Environment: prod|test
```

Key endpoints:

| Method | Path | Capability |
| --- | --- | --- |
| GET | `/session` | `builder_access` |
| GET | `/template-types` | `list_templates` |
| GET | `/template-types/:slug` | `list_templates` |
| GET | `/template-types/:slug/templates` | `list_templates` |
| GET | `/templates/:template_id/versions` | `view_versions` |
| GET | `/templates/:template_id/versions/:version_id` | `view_versions` |
| PUT | `/templates/:template_id/versions/:version_id` | `edit_versions` |
| POST | `/templates/:template_id/versions/:version_id/publish` | `publish_versions` |
| GET | `/templates/:template_id/versions/:version_id/locales` | `locale_access` |
| GET | `/templates/:template_id/versions/:version_id/locales/:locale` | `locale_access` |
| POST | `/templates/:template_id/versions/:version_id/locales/:locale` | `locale_access` |
| PUT | `/templates/:template_id/versions/:version_id/locales/:locale` | `locale_access` |
| DELETE | `/templates/:template_id/versions/:version_id/locales/:locale` | `locale_access` |
| POST | `/templates/:template_id/preview-mjml` | `builder_access` |
| POST | `/templates/:template_id/test-send` | `test_send` |
| GET | `/injectors` | `builder_access` |
| GET | `/injectors/:name` | `builder_access` |
| GET | `/policies` | `builder_access` |

The effective workspace may differ from the path workspace if the registered resolver returns a read-only fallback.

## Global scope

Base: `/api/v1/manage/global`

Global scope mirrors the workspace resource families for superadmin use.

## Webhook events

Typical outbound events include:

- `email.queued`
- `email.sent`
- `email.delivered`
- `email.bounced`
- `email.complained`
- `email.failed`
- `email.opened`
- `*`

## Postman

A Postman collection is available in `docs/postman/`.
