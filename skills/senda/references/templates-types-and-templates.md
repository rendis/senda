# Template Types & Templates

`TemplateType` defines a kind of email (e.g. "welcome-email", "password-reset"
with their `variable_schema`). `Template` is the concrete rendering bound to
a `TemplateType` for a given scope (global or workspace). Versions and locales
live one level deeper — see `versions-locales-and-builder.md`.

## Data model

### `TemplateType`

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `workspace_id` | uuid? | `NULL` = global; non-null = workspace (can be `_system`). |
| `slug` | string | Unique per scope (`UNIQUE NULLS NOT DISTINCT (slug, workspace_id)`). |
| `name`, `description` | string | |
| `adapter_id` | uuid? | Resolves which sender to use; if `NULL`, send fails 422 `NO_ADAPTER`. |
| `sender_identity_id` | uuid? | Optional; pins a specific identity. |
| `variable_schema` | jsonb | JSON Schema describing the `event.*` variables. |
| `test_recipient_mode` | enum? | Optional override of workspace's recipient policy in `env=test`. |
| `test_recipient_addresses` | string[] | Same. |
| `created_at`, `updated_at`, `deleted_at` | ts | Soft-delete via `deleted_at`. |

Computed at runtime (not persisted): `OwnerScope` (`global|tenant|workspace`),
`InheritedFromSystem` (true when read from `_system`).

### `Template`

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK |
| `template_type_id` | uuid | FK |
| `workspace_id` | uuid? | `NULL` = global. |
| `is_disabled` | bool | Kill switch; preserved. Send hits `TEMPLATE_DISABLED`. |
| `is_fork` | bool | true on forks. |
| `origin_template_id` | uuid? | Source on a fork; metadata only. |
| `created_at`, `updated_at`, `deleted_at` | ts | Soft-delete. |

Constraint: `UNIQUE NULLS NOT DISTINCT (template_type_id, workspace_id)` —
one template per (type, scope).

## Endpoints

### Template types — workspace-scoped

`<ws>` and `<envWS>` prefixes both apply.

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST | `<ws>/template-types` | workspace_admin | `{slug, name, description?, adapter_id?, sender_identity_id?, variable_schema?, test_recipient_mode?, test_recipient_addresses?}`. Slug is kebab-case. `test_recipient_*` only valid in env=test. Subject to `allow_workspace_local_templates` policy if workspace ≠ `_system`. |
| GET | `<ws>/template-types` | workspace_viewer+ | Returns local + inherited from `_system` (dedup by slug). No real cursor. |
| GET | `<ws>/template-types/:slug` | workspace_viewer+ | Resolves down the chain. |
| PUT | `<ws>/template-types/:slug` | workspace_admin | Patch parcial. Only mutable if owned by current workspace. |
| DELETE | `<ws>/template-types/:slug` | workspace_admin | Owned-only (inherited returns 403 `READ_ONLY_INHERITED_TEMPLATE_TYPE`). Subject to `allow_workspace_local_templates` for non-`_system` workspaces; blocked with 403 `WORKSPACE_LOCAL_TEMPLATES_DISABLED` when the policy is off. |

### Template types — global

`/api/v1/manage/global/template-types[/:slug]` — same operations, RBAC =
`superadmin`, cursor pagination on list.

### Templates — workspace-scoped

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `<ws>/template-types/:slug/templates` | workspace_viewer+ | Returns 0 or 1 template (resolved via chain). |
| POST | `<ws>/templates` | workspace_admin | `{template_type_id}`. Subject to `allow_workspace_local_templates`. Returns 403 `WORKSPACE_LOCAL_TEMPLATES_DISABLED` when blocked. |
| POST | `<ws>/templates/:template_id/fork` | workspace_editor+ | Forks an inherited template into the current workspace. 409 `TEMPLATE_ALREADY_LOCAL` if it's already owned. Subject to `allow_workspace_inherited_template_forks`; 403 `WORKSPACE_INHERITED_TEMPLATE_FORKS_DISABLED` when blocked. |
| POST | `<ws>/templates/:template_id/disable` | workspace_admin | Kill switch on. |
| POST | `<ws>/templates/:template_id/enable` | workspace_admin | Kill switch off. |
| DELETE | `<ws>/templates/:template_id` | workspace_admin | 409 `HAS_PUBLISHED_VERSION` if any version is published. |
| GET | `<ws>/templates/:template_id/bulk-send-config` | workspace_viewer+ | `{max_items, version_strategy, request_shape}`. Default `max_items = 100`. |
| POST | `<ws>/templates/:template_id/bulk-send` | workspace_editor+ | Up to `max_items` items. Returns 202 with `{accepted_count, suppressed_count, failed_count, template_resolved}`. Requires the template to be local to the workspace. |
| POST | `<ws>/templates/:template_id/test-send` | workspace_editor+ | `{recipient_email, variables?, injectors?, locale?}`. Requires a published version. 501 if test-send service not configured. |
| POST | `<ws>/templates/:template_id/preview-mjml` | workspace_viewer+ | `{mjml}`. Compiles + applies static injector preview values. See `versions-locales-and-builder.md`. |
| GET | `<ws>/templates/:template_id/screenshot` | workspace_viewer+ | Returns a full-page PNG of the template. Optional query params: `viewport` (`desktop`\|`mobile`), `version_id`, `locale`. Placeholders NOT resolved. 503 `SCREENSHOT_DISABLED` when feature flag is off. |

### Screenshot endpoint

`GET /api/v1/manage/tenants/{tenant_code}/workspaces/{workspace_code}/templates/{template_id}/screenshot`

Query params:
- `viewport`: `desktop` (default, 1280 px wide) or `mobile` (390 px wide, mobile UA + touch emulation)
- `version_id` (optional, UUID): a specific version. Default = latest published.
- `locale` (optional, string): the locale-specific MJML override. Default = version's `default_locale`.

Returns `Content-Type: image/png`. Full-page capture, height capped at `screenshot.max_height_px` (default 6000).

Errors:
- 400 `INVALID_VIEWPORT` / `INVALID_TEMPLATE_ID` / `INVALID_VERSION_ID`
- 403 RBAC (workspace_viewer required)
- 404 `TEMPLATE_NOT_FOUND` / `NO_PUBLISHED_VERSION`
- 409 `CONFLICT` (template disabled)
- 503 `SCREENSHOT_DISABLED` (feature flag off) / `SCREENSHOT_BUSY` (slot pool full)
- 504 `SCREENSHOT_TIMEOUT`
- 500 `SCREENSHOT_INTERNAL` (browser crash; auto-restarted)

Placeholders (`{{ event.X }}`, `{{ injector.Y.Z }}`) are NOT resolved — they appear literally in the image. To preview with real values, use `test-send`.

Available only on the management workspace surface. Not on `/global/` or `/external/`.

### Templates — global

`/api/v1/manage/global/templates[/...]` — same shape, RBAC = `superadmin`.

## Operational rules

- **Resolution at send-time**: `TemplateResolver` walks
  `[workspace, _system]`. The first scope that has a `(template_type_id,
  scope)` row wins; then it picks the template's currently-published
  version. **Global templates don't enter this chain** — they only matter to
  Superadmin and to global test-sends.
- **Inheritance is implicit**: a workspace without a local template
  automatically uses the `_system` one. No grant operation; it's by chain
  position.
- **Fork** snapshots the source template plus all its versions and locales
  (including drafts and archived) into the new template; sets `is_fork = true`,
  `origin_template_id = source.id`. Forks do NOT get re-synced if `_system`
  publishes new versions.
- **Disable** is workspace-local: disabling a forked template only affects
  the local template. Disabling a `_system` template propagates to inheriting
  workspaces (they see the disabled flag because they read the `_system` row).
- **DELETE blocking is permanent**: `DeleteTemplate` rejects with 409
  `HAS_PUBLISHED_VERSION` whenever the template has any version in the
  `published` state. Publishing another version archives the previous one but
  immediately puts the new version in `published`, so the guard never
  releases. Practical consequence: a template that has *ever* been published
  cannot be deleted via the API. Use `disable` to take it out of service; if
  you truly need to remove it, that requires a DB-level intervention.

## Inheritance & sharing semantics

| Source scope | Visible in | Mutable from | Notes |
|---|---|---|---|
| Global (`workspace_id = NULL`) | Superadmin only | Superadmin | Not in workspace runtime chain. |
| Tenant `_system` | All workspaces of that tenant | `_system` only (or after fork) | Inherited reads are read-only. |
| Workspace local | That workspace | That workspace | One per (template_type, workspace). |

There is no template ACL ("share with workspaces A, B"). All `_system`
templates are visible to every child workspace of the tenant. To restrict, use
a different `template_type_id` or block by policy.

## Builder syntax (cheat sheet)

Only two namespaces. Anything else renders as empty string silently.

- `{{ event.<name> }}` — values come from the request `variables`. Schema
  declared on the `template_type.variable_schema`.
- `{{ injector.<name>.<field> }}` — values come from the merged injector tree
  (see `injectors.md`).

`subject`, `from_name`, `preview_text` are plain text rendered by the same
engine. `body_mjml` is rendered, then compiled by gomjml.

## Cuándo consultar OpenAPI / MCP

- Body of `POST /template-types` (especially `variable_schema`).
- Bulk-send config `{request_shape}` for the actual list of accepted item
  fields.

## Gotchas

- Listing template types from a workspace returns *both* local and inherited
  rows; check `inherited_from_system` / `owner_scope` to know whether you can
  mutate.
- Forking is allowed at `workspace_editor`; publishing requires
  `workspace_admin`. A forked draft version still requires admin to publish.
- DELETE template requires admin; DELETE version requires admin AND `draft`
  status. There is no API path to delete a template that has ever been
  published — `disable` is the supported way to retire it.
- `bulk-send` from management is meant for tooling. Production sends go
  through the data plane (`POST /api/v1/send`).
