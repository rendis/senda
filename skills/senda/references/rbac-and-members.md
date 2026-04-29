# RBAC & Members

Single source of truth for roles, permissions, and member CRUD.

## Roles

Five roles, single role per `(member, scope_type, tenant_id, workspace_id)`
since migration 045. Higher level satisfies lower-level requirements.

| Role | Level | Where it can be assigned |
|---|---:|---|
| `superadmin` | 100 | `scope_type = global` (no tenant/workspace) |
| `tenant_admin` | 80 | `scope_type = tenant` |
| `workspace_admin` | 60 | `scope_type = workspace` |
| `workspace_editor` | 40 | `scope_type = workspace` |
| `workspace_viewer` | 20 | `scope_type = workspace` |

Schema enforces the assignment shape; the API rejects mismatched bodies.

API keys bypass RBAC entirely; they only reach the data plane.

## Permission matrix (per domain × scope)

`R = read`, `C = create`, `U = update`, `D = delete`, `P = publish`,
`F = fork`, `S = set values / set default`, `X = execute`.

### Global scope (`/api/v1/manage/global/...`)

| Domain | superadmin |
|---|:---:|
| Tenants | CRUD + delete-blocked guard |
| Members (global) | CRUD + role mgmt |
| Template types | CRUD |
| Templates | CRUD + disable + preview-mjml + test-send |
| Versions / locales | CRUD + clone + publish |
| Injectors | CRUD + UpdateField |
| Adapters | CRUD + identities + auto-provision |
| Audit log | R |
| Config | RU |
| Dashboard | R |

Anything other than `superadmin` on `/global/...` → 403.

### Tenant scope (`/api/v1/manage/tenants/:tenant_code/...`, no workspace)

| Domain | tenant_admin | superadmin |
|---|:---:|:---:|
| Tenant fields (`name`, `is_active`) | RU | RU |
| Tenant DELETE | — | D |
| Tenant members | CRUD + role mgmt | CRUD + role mgmt |
| Workspaces (logical) | CRUD | CRUD |
| Dashboard (tenant) | R | R |

### Workspace scope (`<ws>` and `<envWS>`)

| Domain | viewer | editor | admin | tenant_admin |
|---|:---:|:---:|:---:|:---:|
| Workspace (read) | R | R | R | R |
| Workspace shared PUT | — | — | U (`code`,`name`) | U |
| Workspace env-scoped PUT | — | — | U (full incl. `test_recipient_*` only in env=test) | U |
| Workspace runtime reset (test only) | — | — | X | X |
| Workspace policies (only on `_system`) | R | R | R | RU |
| Workspace members | R | R | C/U/D + role mgmt | + |
| Template types — read | R | R | R | R |
| Template types — write | — | — | C/U/D (subject to `allow_workspace_local_templates`) | + |
| Templates — read | R | R | R | R |
| Templates — create | — | — | C (subject to policy) | + |
| Templates — fork | — | F (subject to `allow_workspace_inherited_template_forks`) | + | + |
| Templates — disable / enable / DELETE | — | — | U/D (DELETE blocked by published version) | + |
| Versions — list / read | R | R | R | R |
| Versions — create / update (draft) / clone | — | C/U | + | + |
| Versions — publish | — | — | P | + |
| Versions — DELETE | — | — | D (only if draft) | + |
| Locales | R | C/U/D | + | + |
| Injectors — read | R | R | R | R |
| Injectors — create / update / DELETE | — | — | C/U/D (subject to `allow_workspace_local_injectors`) | + |
| Injectors — `UpdateField` / `SetValues` | — | U/S | + | + |
| Adapters — read | R | R | R | R |
| Adapters — `test` (connectivity probe) | X | X | X | X |
| Adapters — create / update / DELETE / auto-provision | — | — | + | + |
| Adapters — `validate-ses` | — | — | X | X |
| Adapters/identities — workspace-access (only on `_system`) | — | — | RU | RU |
| Identities — sync / set-default / DELETE | — | — | + | + |
| API keys | — | — | C / list / revoke | + |
| Webhooks — read | R | R | R | R |
| Webhooks — create / update / DELETE / test | — | — | + | + |
| Emails query | R | R | R | R |
| Suppression — add / get | — | — | C/R | + |
| Suppression — DELETE | — | — | — | superadmin only (handler-level) |
| Audit log (workspace) | R | R | R | R |
| Bulk-send (management) | — | X | X | X |
| Test-send | — | X | X | X |
| Preview MJML | R | R | R | R |
| Dashboard | R | R | R | R |

`+` means "and above". `superadmin` always satisfies.

## Members CRUD

Three groups of routes. All require OIDC + `OIDCOnly()`.

### `/api/v1/members/me`

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `/api/v1/members/me` | any authenticated member | Returns member + every role with `tenant_code` and `workspace_code` enriched. |

### Global members (`/api/v1/manage/members/...`)

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `/api/v1/manage/members` | superadmin | Cursor pagination. |
| POST | `/api/v1/manage/members` | superadmin | Idempotent on `email`; returns existing on conflict. |
| GET | `/api/v1/manage/members/:member_id` | superadmin | |
| DELETE | `/api/v1/manage/members/:member_id/access` | superadmin | Revokes all global roles. Cannot self-revoke own superadmin (409). |
| PUT | `/api/v1/manage/members/:member_id/role` | superadmin | Replace single role for a derived scope. |
| POST | `/api/v1/manage/members/:member_id/roles` | superadmin | Add a role with `{role, scope_type, tenant_id?, workspace_id?}`. |
| DELETE | `/api/v1/manage/members/:member_id/roles/:role_id` | superadmin | |

### Tenant members

`/api/v1/manage/tenants/:tenant_code/members[/...]` — same shape, RBAC =
`tenant_admin+`. Only `tenant_admin` is a valid role at this scope.

### Workspace members

`<ws>/members[/...]` and `<envWS>/members[/...]` — same shape, RBAC =
`workspace_admin+` for mutations, `workspace_viewer+` for reads. Valid roles:
`workspace_admin`, `workspace_editor`, `workspace_viewer`.

## Operational rules

- A member can hold multiple roles in different scopes (e.g. tenant_admin on
  one tenant, workspace_editor on a workspace under another tenant).
- Migrations 045+: cannot have two roles for the same `(scope_type, tenant_id,
  workspace_id)`. Updates replace the existing one.
- The `_system` workspace has no special role. Membership uses regular
  `workspace_*` roles bound to that workspace's UUID.
- Self-revocation of own global superadmin is blocked (409). Revoke another
  Superadmin first or have the system Superadmin do it.
- Global member POST is idempotent on email; safe to retry.

## Gotchas

- The matrix above is the *registered* RBAC. A few handlers add extra checks:
  - `DELETE .../suppression/:email` requires Superadmin (not just
    `workspace_admin` despite the route registration).
  - Workspace policies PUT only mutates when `workspace_code = _system`.
  - Tenant DELETE is also blocked by SES adapter presence.
- A `workspace_editor` on a regular workspace can fork inherited templates
  (`POST .../templates/:id/fork`) but cannot publish versions
  (`POST .../versions/:vid/publish` requires `workspace_admin`).
- A `tenant_admin` always satisfies workspace-level RBAC for that tenant.
- API keys appear nowhere in this matrix — they bypass RBAC and only access
  the data plane.
