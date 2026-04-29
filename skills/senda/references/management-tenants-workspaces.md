# Management — Tenants & Workspaces

Onboarding, tenants, workspaces (shared and env-scoped), workspace policies,
runtime reset, dashboards.

## Endpoints

### Onboarding

| Method | Path | Auth | RBAC | Notes |
|---|---|---|---|---|
| GET | `/api/v1/onboarding/status` | none | any | `{"needs_onboarding": bool}`. |
| POST | `/api/v1/onboarding/setup` | OIDC bearer (read directly from header) | first-Superadmin bootstrap | Creates first tenant + `_system` pair (prod+test) + Superadmin member. Returns 201. |

### Tenants

| Method | Path | Auth | RBAC | Notes |
|---|---|---|---|---|
| POST | `/api/v1/manage/tenants` | OIDC | superadmin | Body `{code, name}`. Auto-creates `_system` pair (prod+test). |
| GET | `/api/v1/manage/tenants` | OIDC | superadmin | Cursor pagination. Each item carries `delete_blocked_reason`. |
| GET | `/api/v1/manage/tenants/:tenant_code` | OIDC | tenant_admin+ | Includes `delete_blocked_reason`. |
| PUT | `/api/v1/manage/tenants/:tenant_code` | OIDC | tenant_admin | Patch `{name?, is_active?}`. |
| DELETE | `/api/v1/manage/tenants/:tenant_code` | OIDC | superadmin | Soft-delete. Blocked (409) if any SES adapter is active. |

### Workspaces — shared (logical CRUD)

| Method | Path | Auth | RBAC | Notes |
|---|---|---|---|---|
| POST | `/api/v1/manage/tenants/:tenant_code/workspaces` | OIDC | tenant_admin | Body `{code, name, default_locale?}`. Creates prod+test pair sharing one `LogicalWorkspaceID`. |
| GET | `/api/v1/manage/tenants/:tenant_code/workspaces` | OIDC | tenant_admin | Cursor pagination. Optional `?environment=prod|test` (defaults `prod`). |
| GET | `.../workspaces/:workspace_code` | OIDC | workspace_viewer+ | Defaults to `prod`. |
| PUT | `.../workspaces/:workspace_code` | OIDC | workspace_admin | **Logical-only fields**: `code`, `name`. Rejects `is_active`, `open_tracking_enabled`, `default_locale`, `test_recipient_*` here — those are env-scoped. |
| DELETE | `.../workspaces/:workspace_code` | OIDC | tenant_admin | Soft-delete; blocked for `_system` (409 `SYSTEM_WORKSPACE_PROTECTED`). |
| GET | `/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces` | OIDC | tenant_admin | Same handler as list, env explicit in path. |

### Workspaces — environment-scoped (runtime state)

Group prefix: `/api/v1/manage/environments/:environment/tenants/:tenant_code/workspaces/:workspace_code`.

| Method | Path | Auth | RBAC | Notes |
|---|---|---|---|---|
| GET | `<envWS>` | OIDC | workspace_viewer+ | Reads the env-specific instance. |
| PUT | `<envWS>` | OIDC | workspace_admin | Accepts the full set: `code`, `name`, `is_active`, `open_tracking_enabled`, `default_locale`, `test_recipient_mode`, `test_recipient_addresses`. The two `test_recipient_*` fields only valid when `:environment = test`; otherwise 409. |
| POST | `<envWS>/runtime/reset` | OIDC | workspace_admin | Test only. Returns 409 `TEST_ENVIRONMENT_REQUIRED` if `:environment != test`. Wipes runtime/business data (emails, events). Functional config (templates, adapters, injectors, API keys) survives. |

### Workspace policies

The toggles consulted by every child workspace live on the tenant's `_system`.
Querying any workspace returns the `_system` policy in scope (alias paths exist
at `/manage/tenants/:tenant/workspaces/:workspace_code/policies` and the
env-scoped equivalent).

| Method | Path | Auth | RBAC | Notes |
|---|---|---|---|---|
| GET | `<ws>/policies` and `<envWS>/policies` | OIDC | workspace_viewer+ | Returns the `_system` policy regardless of which workspace path is used. |
| PUT | `<ws>/policies` and `<envWS>/policies` | OIDC | tenant_admin | **Only takes effect when `workspace_code = _system`**; any other workspace returns 404. Body `{allow_workspace_local_templates?, allow_workspace_inherited_template_forks?, allow_workspace_local_injectors?}`. |

### Dashboards (read-only stats)

| Method | Path | Auth | RBAC | Notes |
|---|---|---|---|---|
| GET | `<ws>/dashboard-stats` and `<envWS>/dashboard-stats` | OIDC | workspace_viewer+ | `?range=7d|30d`. Returns totals, time series, recent emails (5), recent audit (10). |
| GET | `/api/v1/manage/tenants/:tenant_code/dashboard-stats` | OIDC | tenant_admin | Tenant-scoped aggregate. |
| GET | `/api/v1/manage/global/dashboard-stats` | OIDC | superadmin | Global aggregate. |

## Operational rules

- **Logical pair semantics**: a workspace lives in two rows (prod, test), both
  pointing at the same `LogicalWorkspaceID`. Shared CRUD updates both rows in
  the logical fields; env-scoped CRUD updates one row.
- **Default locale**: stored per environment row, not on the logical pair.
  Setting it from the shared CRUD is rejected; use env-scoped PUT.
- **`_system` is a workspace too**: addressed by `workspace_code = _system`.
  All workspace-shaped routes accept it. Sharing of adapters/identities is
  managed under `<ws>/adapters/.../workspace-access` (see
  `adapters-and-identities.md`).
- **Cannot delete `_system`**: 409 `SYSTEM_WORKSPACE_PROTECTED`.
- **Cannot delete tenant with active SES adapter**: 409. Deprovision SES first
  (see `adapters-and-identities.md`).

## Flujo end-to-end

1. `senda_describe_endpoint POST /api/v1/onboarding/setup`, then call it with
   the bootstrap OIDC token.
2. `senda_call_endpoint POST /api/v1/manage/tenants` to create a real tenant.
3. `senda_call_endpoint POST /api/v1/manage/tenants/:tenant/workspaces` for
   each business workspace.
4. Configure environment-specific runtime state via the env-scoped PUT.
5. If you intend to send in test, set `test_recipient_mode` and
   `test_recipient_addresses` on the test row.
6. If you need to flip tenant-wide policies (allow local templates, allow
   forks, allow local injectors), PUT them on `<ws>/_system/policies`.

## When to consult OpenAPI / MCP

- Body shape for `POST /tenants` and the workspace endpoints — fields are
  small but evolve.
- Response shapes for dashboard endpoints (totals + time series) — useful when
  building a UI binding.

## Gotchas

- The shared workspace PUT rejects runtime fields. If you "edit a workspace"
  in a UI sense, decide whether you want the logical update (rename) or a
  runtime state change (`is_active`, locale, recipient policy) and call the
  matching path.
- Duplicate listing paths exist (`/manage/tenants/:t/workspaces` and
  `/manage/environments/:e/tenants/:t/workspaces`); they share the same
  handler but the env-prefixed one filters by environment.
- `_system` is created automatically — never `POST` a workspace named
  `_system`. Use the implicit one.
- The shared workspace PUT also rejects `workspace_code = _system` with
  409 `SYSTEM_WORKSPACE_PROTECTED`. To rename or otherwise edit `_system`,
  use the env-scoped PUT (`/manage/environments/:env/.../workspaces/_system`).
- Runtime reset is not a soft-stop. It deletes runtime history. There is no
  "undo".
- Tenant DELETE is soft only; the row remains with `deleted_at`. Reusing the
  same `code` afterwards is not supported.
