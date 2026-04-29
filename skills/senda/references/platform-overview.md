# Platform Overview

Mental model and shared vocabulary for everything else.

## Surfaces

| Surface | Auth | Path prefix | Purpose |
|---|---|---|---|
| Management plane | OIDC (`OIDCOnly()`) | `/api/v1/manage/...` | Humans configuring tenants, workspaces, templates, injectors, adapters, keys, etc. |
| Data plane | Raw API key | `/api/v1/send`, `/api/v1/send/batch`, `/api/v1/emails…` | Machines sending and querying delivery state. |
| External integration | Profile-scoped custom auth + `X-Senda-Environment` header | `/api/v1/external/:profile_slug/...` | Embedded builders/portals using a profile-mediated auth method. |
| Public | None | `/health`, `/healthz`, `/api/v1/onboarding/...`, `/t/o/:tracking_id`, `/public/video-thumbnail` | Health, onboarding, tracking pixel, image proxy. |
| Provider | Implicit (SNS signature) | `/api/v1/webhooks/ses/inbound` | SES → SNS → Senda inbound webhook. |

## Scope chain

```
global  →  tenant _system workspace  →  workspace
```

Runtime resolution actually traverses **only `[workspace, _system]`**. Globally
scoped resources (templates, injectors, template types) are NOT in the chain
of a workspace runtime. Global-scoped resources only matter to Superadmins
operating the global surface and to test-sends invoked at global scope. See
`resolution-and-inheritance.md` for the precise algorithm.

### `_system` workspace

- One per tenant; created together with the tenant.
- Marked by `is_system = true`; addressed by `workspace_code = _system`.
- Holds tenant-wide defaults: shareable adapters, identities, injectors,
  template types, templates.
- Owns the policy toggles consulted by all child workspaces:
  `allow_workspace_local_templates`, `allow_workspace_inherited_template_forks`,
  `allow_workspace_local_injectors` (`workspace_policies` updates only succeed
  here).
- Cannot receive sends (`SYSTEM_WORKSPACE_BLOCKED`).

### Owned vs inherited vs forked

- **owned**: created in this scope; mutable from this scope.
- **inherited**: visible from a parent (`_system`) but read-only here. Trying
  to mutate yields `READ_ONLY_INHERITED_INJECTOR`, `inherited templates are
  read-only in workspace scope`, etc.
- **forked**: a child workspace copied an inherited template into local
  ownership. Fork copies all versions and locales; sets `is_fork = true`,
  `origin_template_id` for traceability. Forks do not auto-resync with origin.

## Environments

Every logical workspace exists twice: `prod` and `test`. They share
`LogicalWorkspaceID`, `code`, and `name`, but have independent: templates,
versions, locales, adapters, identities, injectors, API keys, suppression,
emails history, audit log entries, policies, recipient safety settings.

### Where environment comes from

- **Management plane (env-scoped routes)**: explicit in path —
  `/api/v1/manage/environments/:environment/...`.
- **Management plane (shared routes)**: `code`/`name` only; environment is
  irrelevant for purely logical updates.
- **Data plane**: token prefix — `senda_prod_…` or `senda_test_…`.
- **External integration**: header — `X-Senda-Environment: prod|test`.

### Test-only behavior

Available only in `env=test`:

- `test_recipient_mode` (`replace` default | `append`) and
  `test_recipient_addresses` on workspace and on template type.
- `POST .../runtime/reset` purges runtime/business history (emails, events).
  Functional config (templates, adapters, injectors, API keys) survives.
- Sending under test routes deliveries through the test recipient policy.

In `prod`, those fields are inaccessible / rejected.

## Auth & roles

| Mechanism | Where | Notes |
|---|---|---|
| OIDC bearer | Management plane | `Authorization: Bearer <id_token>`. `OIDCOnly()` middleware blocks API keys here. |
| Raw API key | Data plane | `Authorization: Bearer senda_prod_…` or `senda_test_…`. Bypasses RBAC. |
| External token + profile | External surface | Required headers depend on the registered `auth_method_name`; always include `X-Senda-Environment`. |

Roles (single role per scope assignment after migration 045):

| Role | Level | Scope |
|---|---:|---|
| `superadmin` | 100 | global |
| `tenant_admin` | 80 | tenant |
| `workspace_admin` | 60 | workspace |
| `workspace_editor` | 40 | workspace |
| `workspace_viewer` | 20 | workspace |

Permission matrix lives in `rbac-and-members.md`.

## Common identifiers

- **Tenant**: `code` (URL-safe slug) and UUID v7.
- **Workspace**: `code` (per tenant), UUID per environment (`prod` and `test`
  have different UUIDs but same `LogicalWorkspaceID`).
- **`ref` (send)**: `tenant_code:workspace_code:template_type_slug`. Never
  embed environment.
- **Template type**: `slug` (kebab-case), unique per scope.
- **Injector**: `name` (VARCHAR 100), unique per scope.
- **Tracking ID**: UUID v7 string.

## Encoding & quotas (defaults)

- Pagination: cursor base64-JSON `{t, id}`; query params `cursor`, `limit`
  (default 25, max 100).
- Send batch: max 100 items per call (configurable).
- Send recipients: max 50 per send.
- API key name: max 100 chars.

## Where to go next

| Need | File |
|---|---|
| Pick an endpoint, error codes, headers | `operating-via-mcp.md` |
| Permissions detail | `rbac-and-members.md` |
| Resolution algorithm internals | `resolution-and-inheritance.md` |
