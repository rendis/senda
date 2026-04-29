# Injectors

Injectors are the data sources a template body references. Two flavors share
the `injector.<name>.<field>` namespace at runtime:

- **DB injectors** — managed via API; rows in `injector_definitions`,
  `injector_fields`, `injector_values`.
- **Code injectors** — registered through the Go SDK
  (`engine.RegisterInjector(...)`). See `sdk-extension-points.md`.

## Data model

### `injector_definitions`

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK. |
| `name` | varchar(100) | Unique per scope (`UNIQUE NULLS NOT DISTINCT (name, workspace_id)`). |
| `workspace_id` | uuid? | `NULL` = global; non-null = a workspace (incl. `_system`). |
| `description` | text | |
| `created_at`, `updated_at`, `deleted_at` | ts | Soft-delete via `deleted_at`. |

There is no `code` column; the identifier is `name`.

### `injector_fields`

| Field | Type | Notes |
|---|---|---|
| `injector_definition_id` | uuid | FK. |
| `field_name` | string | Used in templates as `{{ injector.<name>.<field_name> }}`. |
| `field_type` | enum | `text` \| `number` \| `bool` \| `img` \| `url` \| `html`. |
| `description`, `position` | | |
| `default_value` | jsonb | Used when nothing else provides a value. |
| `allow_overwrite` | bool | `false` ⇒ DB chain wins (`injector_values` workspace > `_system` > `default_value`); request and code overrides are ignored. `true` ⇒ request body > code injector > `default_value`, and `injector_values` rows are skipped. See the precedence section for the full rules. |

No `object` or `array` field type. Complex shapes go in `default_value` as
JSON blobs but lose schema enforcement.

### `injector_values`

Per-scope override for a specific `(injector_definition_id, field_name,
workspace_id)`. Stored as JSONB in `value`.

## Endpoints

Apply to `<ws>` and `<envWS>` (workspace, including `_system`) and to
`/api/v1/manage/global/...` (RBAC = `superadmin`). Workspace RBAC shown below.

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST | `<ws>/injectors` | workspace_admin | `{name, description?, fields:[{field_name, field_type, description?, position?, default_value?, allow_overwrite?}]}`. Subject to `allow_workspace_local_injectors` policy when workspace ≠ `_system`. 403 `WORKSPACE_LOCAL_INJECTORS_DISABLED`. |
| GET | `<ws>/injectors` | workspace_viewer+ | `?include_inherited=true` to merge `_system` injectors and the static built-in catalog. No real cursor. |
| GET | `<ws>/injectors/:name` | workspace_viewer+ | Returns definition + fields + resolved values for the chain. |
| PUT | `<ws>/injectors/:name` | workspace_admin | Owned-only. 403 `READ_ONLY_INHERITED_INJECTOR` on inherited rows. |
| PUT | `<ws>/injectors/:name/fields/:field_name` | workspace_editor+ | Patch a single field: `{default_value?, allow_overwrite?}`. Owned-only. |
| PUT | `<ws>/injectors/:name/values` | workspace_editor+ | `{values: [{field_name, value}]}`. Owned-only; 204 on success. |
| DELETE | `<ws>/injectors/:name` | workspace_admin | Soft-delete. Owned-only. |

Globally:

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST/GET/GET-by-name/PUT/PUT-field/DELETE | `/api/v1/manage/global/injectors[/...]` | superadmin | No `PUT .../values` at global level (intentional — global injectors don't have value overrides). |

## Resolution at send time

`InjectorMerger.ResolveWithContext()` runs once per send:

1. **Build the chain**: `ChainResolver` returns
   `[current_workspace_id, system_workspace_id]`. **Globals (workspace_id =
   NULL) are NOT in the chain** for normal workspace sends; only test-sends
   at global scope traverse the global path.
2. **Load DB definitions** for those scopes; dedup by lowest scope index
   (workspace wins over `_system`).
3. **Run `InitFunc` once** (SDK; see `sdk-extension-points.md`).
4. **Run code injectors** in topo order of their dependencies. For each
   field, the merger applies the precedence below.

### Precedence per field

The merger first resolves DB-only values per scope (`workspace > _system >
field default_value`) into a base map, then layers request body and code
injectors on top, gated by `allow_overwrite`.

**`allow_overwrite = false`** (locked field):

1. `injector_values` row for the workspace.
2. `injector_values` row for `_system`.
3. Field's `default_value`.

Request body and code injectors are **ignored** for these fields. This is the
intended use: brand assets, signatures, invariants the caller cannot tamper with.

**`allow_overwrite = true`** (overwrite-able field):

1. **Request body override** — `req.injectors.<name>.<field>`.
2. **Code injector** — value from `Injector.Resolve()`.
3. Field's **`default_value`**.

⚠️ **`injector_values` rows are NOT consulted in this branch**. Setting a
workspace value via `PUT /injectors/:name/values` only takes effect when
`allow_overwrite = false`. If you need a per-workspace baseline that callers
can override, model it as a code injector or accept that the request must
carry the value every time.

### Dedup across scopes for definitions with the same `name`

If both workspace and `_system` have an injector with the same `name`, the
workspace row wins entirely (its fields, including `allow_overwrite` and
`default_value`, mask the `_system` ones). This is how a workspace
"overrides" an inherited injector without a fork.

## Operational rules

- **No fork operation for injectors**. Inherited rows are read-only. To
  diverge, create a workspace-local one with the same `name` and set the
  fields/values you need.
- **Inheritance is implicit**: workspace `GET /injectors` shows local rows
  plus `_system` rows when `include_inherited=true`.
- **Static built-in catalog**: included in `?include_inherited=true` listings
  (e.g. `tenant`, `workspace` if registered as code injectors). They appear
  read-only.
- **Code injector vs DB injector with same `name`**: they merge at the field
  level. The DB defines the contract (fields + `allow_overwrite`); the code
  injector contributes either values for known fields (subject to
  precedence) or **extra fields not declared in the DB** (which still flow
  through to the renderer).
- **Static preview** (`POST /preview-mjml`) only substitutes fields with
  `allow_overwrite = false` (and a few static cases). Override-able fields
  remain as `{{ injector.X.Y }}` in the preview HTML so the user can see
  where they will be filled.

## Flujo end-to-end (workspace sharing via `_system`)

1. As `tenant_admin`, create the injector in `_system`:
   `senda_call_endpoint POST .../tenants/:t/workspaces/_system/injectors`.
2. From any child workspace, `GET .../injectors?include_inherited=true` —
   the row appears with `inherited_from_system = true`, read-only.
3. To customize, `POST .../injectors` in the child with the same `name`. The
   child's row will mask `_system` for that workspace.
4. Set per-workspace values with `PUT .../injectors/:name/values`.
5. Reference in templates as `{{ injector.<name>.<field> }}`.

## Cuándo consultar OpenAPI / MCP

- Body shape for `POST /injectors` (especially the `fields` list and the
  `default_value` JSON formats per `field_type`).
- Response shape for the resolved-values branch of `GET /injectors/:name`.

## Gotchas

- **Globals are not in the runtime chain** of a workspace. A global injector
  is only useful for global test-sends or as a Superadmin reference; it does
  NOT propagate down to a tenant's `_system` automatically. To make
  something tenant-wide, place it in `_system`.
- **`allow_overwrite = false`** is absolute against request/code overrides;
  it preserves the DB chain (`injector_values` workspace > `_system` > field
  `default_value`). Use it for invariants (logos, signature, brand colors)
  configured per workspace.
- **`allow_overwrite = true`** silently bypasses `injector_values`: the
  configured workspace/`_system` values are NOT applied. Only request body,
  code injectors, and the field's `default_value` participate.
- **`default_value` is JSONB**: respect the field type. For `bool`, send
  `true`/`false`, not `"true"`. For `number`, send a JSON number. For
  `html`, send a JSON string.
- **Inherited mutation attempts** return 403 `READ_ONLY_INHERITED_INJECTOR`,
  not 404. The injector exists; you just can't write it.
- **Definition lifecycle**: deleting a definition does not nullify
  template references; `{{ injector.gone.x }}` simply renders as empty
  string at send time. There is no compile-time validation.
- **Naming collisions** between code and DB injectors are not errors. Plan
  for them: pick `code` injector names that intentionally extend or override
  DB ones, or namespace them (`crm_user`, `crm_org`).
