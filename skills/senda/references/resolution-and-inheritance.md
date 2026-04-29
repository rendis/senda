# Resolution & Inheritance

How Senda resolves resources at runtime, and the precise inheritance rules.

## Scope chain (runtime, for a workspace request)

```
[ workspace, tenant_system_workspace ]
```

Two scopes only. The chain is built by `ChainResolver.Resolve(workspace_id)`
and cached for ~5 minutes.

**Globals (`workspace_id IS NULL`) are NOT in the workspace runtime chain.**
Templates, template types, and injectors at global scope are reachable only
via the `/api/v1/manage/global/...` surface and via test-sends invoked at
global scope. They do not propagate to a tenant's `_system` automatically.

## Three states of a resource in a given workspace

- **owned** — `workspace_id` matches the calling workspace. Fully mutable
  (subject to RBAC and policy).
- **inherited** — visible in the calling workspace because the chain reaches
  the row in `_system` (or because of an explicit grant for shared
  adapters/identities). **Read-only** in the child; mutation attempts
  return e.g. 403 `READ_ONLY_INHERITED_INJECTOR` or `inherited templates are
  read-only in workspace scope`.
- **shared** — special case for adapters and SES email identities: the
  `_system` workspace explicitly grants access to child workspaces via
  `<ws>/adapters/:id/workspace-access` (and the identity equivalent). The
  list is replace-all. Domain identities cannot be shared.

These three states are not interchangeable. Always check `owner_scope` and
`inherited_from_system` before attempting writes.

## Resolution algorithms by resource

### Templates

1. `TemplateResolver.Resolve(workspace_id, type_slug, locale)` looks for a
   `TemplateType` row whose `(slug, scope)` matches in chain order
   (workspace first, `_system` second).
2. Then resolves the `Template` for `(template_type_id, scope)` with the
   same priority.
3. If `template.is_disabled = true` → `TEMPLATE_DISABLED`.
4. Picks the version with `status = 'published'`. None → `NO_PUBLISHED_VERSION`.
5. Locale fallback: requested → language prefix → `default_locale`.

The result is cached as `resolved_template:<workspace_id>:<type_slug>:<locale>`
for 5 minutes.

### Injectors

1. `ChainResolver` builds the same `[workspace, _system]` chain.
2. `InjectorMerger` loads definitions visible in those scopes; deduplicates
   by lowest scope index (workspace masks `_system` for the same `name`).
3. `InitFunc` runs once.
4. Code injectors run in dependency-topological order.
5. Per-field value resolution applies precedence:
   - With `allow_overwrite = false`: `injector_values` row (workspace >
     `_system`) > field `default_value`. Request and code overrides ignored.
   - With `allow_overwrite = true`: request body > code injector > field
     `default_value`. **The `injector_values` rows are not consulted in this
     branch** — see `injectors.md` for the gotcha.

### Template types

Same chain (`[workspace, _system]`), no global. Listed merged with
`include_inherited = true`; deduplicated by `slug` with workspace winning.

### Adapters & identities

- Workspace's own adapters/identities are owned.
- Adapters and SES email identities granted from `_system` via
  `workspace-access` are shared (read-only in the child).
- Domain identities are never shared.
- The default identity for the adapter wins when the template type does not
  pin one (`sender_identity_id` is empty).

## Forks

Forks exist for templates only.

- `POST <ws>/templates/:tid/fork` copies the source template plus all
  versions and locales (any status) into a new template owned by the child.
- Marks `is_fork = true`, `origin_template_id = source.id`.
- After forking, the child resolves to its own template (chain-priority); the
  inherited path is no longer consulted for that `template_type_id`.
- There is no re-sync. Updates to `_system` after the fork do not propagate.

There is no "fork" for injectors, adapters, identities, or template types.

## Environments

Resolution happens within one environment at a time. The runtime never mixes
prod and test resources. The environment comes from:

| Surface | Source |
|---|---|
| Data plane | API key prefix (`senda_prod_` / `senda_test_`) |
| Management | Path `/manage/environments/:environment/...` (env-scoped) or implied by the request semantics (shared CRUD ignores env) |
| External integration | Header `X-Senda-Environment` |
| SDK code | `injCtx.Environment()` |

Cross-environment data plane requests fail with 403 `FORBIDDEN` (message
"API key scope does not match template workspace").

## Known caveats

- **Cache lag after publish**: `PublishVersion` does not invalidate the
  resolved-template cache. Up to 5 minutes after publishing, sends may use
  the previously published version. `ForkTemplate` and `disable/enable` do
  invalidate.
- **Global resources shadowed by `_system`**: a global injector and a
  `_system` injector with the same `name` do not collide because globals
  are not in the workspace chain. The `_system` row simply wins for
  workspace runtime.
- **Logical workspace vs environment row**: shared CRUD writes to both
  rows; env-scoped CRUD writes to one. The resolver picks the row matching
  the active environment.
- **`_system` policy controls children**: `allow_workspace_local_templates`,
  `allow_workspace_inherited_template_forks`, and
  `allow_workspace_local_injectors` live on `_system`'s policy. A child can
  read them but only `tenant_admin` operating on `_system` can change them.

## Agent rules

- Don't conflate inherited with shared. Inherited reaches you by chain
  position; shared reaches you only when `_system` granted access.
- Before mutating, check `owner_scope` (and `read_only` in external surface
  responses).
- For "tenant-wide" resources, use `_system`. For "tenant + only some
  workspaces", use shared adapters/identities.
- For "this workspace only", create owned rows.
- When precedence matters, consult `injectors.md` for the field-level rules
  and `templates-types-and-templates.md` for the version-level rules.
