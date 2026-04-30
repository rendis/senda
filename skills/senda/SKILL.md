---
name: senda
description: >-
  Use when operating, integrating, or extending Senda — a multi-tenant email
  orchestration platform — over its HTTP API or as a Go SDK. Triggers: tenants,
  workspaces, prod/test environments, _system workspace, OIDC, API keys with
  senda_prod_/senda_test_ prefix, template types, templates, template versions,
  draft/published, MJML builder, locales, injectors, adapters (SES, Gmail),
  identities, sender_identity, send, send batch, emails query, webhooks,
  external integration, embed, X-Senda-Environment, capabilities, fork,
  inheritance, RBAC, scope chain, template screenshot, image preview,
  desktop and mobile preview, is_bulk, unsubscribe, List-Unsubscribe,
  RFC 8058, one-click opt-out, preference center, suppression.
allowed-tools:
  - mcp__senda__*
---

# Senda

Operational reference for agents driving **Senda** through `mcp-openapi-proxy`
or embedding it as a Go library. This skill is intentionally self-contained.
Domain detail lives in `references/*`; load only what the current task needs.

## What Senda is

Multi-tenant email orchestration platform with five operational surfaces:

1. **Management plane** — OIDC-authenticated CRUD: tenants, workspaces,
   templates, versions, injectors, adapters, identities, API keys, webhooks,
   policies, members, audit, config.
2. **Data plane** — workspace-scoped sending and email query, authenticated
   with raw API keys (`senda_prod_…` / `senda_test_…`).
3. **Public unsubscribe surface** — unauthenticated endpoints under
   `/api/v1/u/{token}`, used for RFC 8058 one-click opt-out, opt-out-all,
   preference center reads/writes, and resubscribe. No auth required; the
   HMAC-signed token is the credential. Only relevant for bulk template types
   (`is_bulk = true`).
4. **External integration surface** — embeddable builder/editor APIs at
   `/api/v1/external/:profile_slug/...`, custom auth + workspace resolver,
   `X-Senda-Environment` required.
5. **Go SDK / embedder** — register code injectors, per-request init, external
   auth methods, external workspace resolvers, lifecycle hooks.

## Topology

```
tenant
├── _system          (default workspace, auto-created)
├── <workspace A>
├── <workspace B>
└── ...
```

- The hierarchy is **`tenant → workspace`**. There is no extra layer.
- Every tenant owns workspaces. Each workspace exists as a `prod` / `test`
  pair (same `LogicalWorkspaceID`, two environment rows).
- **Every tenant is created with one workspace called `_system`** —
  automatically, no opt-in. `_system` is the **default workspace** of the
  tenant: where shared/default resources live and where tenant-wide policy
  is configured.
- Saying "the tenant default workspace" and "`_system`" is the same thing.
  Other workspaces are regular business workspaces.
- `_system` is **not** a business workspace: it cannot receive sends
  (`SYSTEM_WORKSPACE_BLOCKED`), it is protected against shared-CRUD edits
  (`SYSTEM_WORKSPACE_PROTECTED`), and it is not reusable as an end-user
  destination.

What `_system` is for:

- **Tenant-wide defaults**: template types, templates, injectors, and
  adapters created in `_system` are visible to every other workspace of
  the tenant via the runtime chain (`[workspace, _system]`). A regular
  workspace inherits whatever it does not own locally.
- **Selective sharing**: SES email identities and adapters can be granted
  from `_system` to specific child workspaces (`workspace-access`).
- **Tenant policy**: the `_system` workspace owns the policy toggles
  (`AllowWorkspaceLocalTemplates`, `AllowWorkspaceInheritedTemplateForks`,
  `AllowWorkspaceLocalInjectors`) that decide what other workspaces are
  allowed to do.
- **Sole-workspace tenants**: a tenant that only uses `_system` is valid —
  everything just lives at the default. Adding more workspaces is opt-in.

## Mental model in one glance

- **Scope chain (runtime)**: `[workspace, tenant _system]`. Global scope is
  reachable only via Superadmin global-surface endpoints; it does NOT enter
  the runtime chain of a workspace request (see `resolution-and-inheritance.md`).
- **Environments**: every logical workspace has paired `prod` and `test`
  instances sharing one `LogicalWorkspaceID`. Environment is **never** part of
  the send `ref`. It comes from: API key prefix (data plane), URL path
  `/manage/environments/:environment/...` (management), or `X-Senda-Environment`
  header (external).
- **`_system`**: a special workspace per tenant. It is the control plane for
  tenant-wide defaults, sharing of injectors/adapters/identities, and policy
  toggles (`AllowWorkspaceLocalTemplates`, `AllowWorkspaceInheritedTemplateForks`,
  `AllowWorkspaceLocalInjectors`). It is NOT a normal business workspace.
- **Auth**: OIDC for humans (management plane); raw API key for machines
  (data plane); profile-scoped custom auth for the external surface.
- **RBAC roles** (single role per scope assignment, after migration 045):
  `superadmin > tenant_admin > workspace_admin > workspace_editor > workspace_viewer`.
- **Builder syntax**: only `{{ event.<name> }}` and `{{ injector.<name>.<field> }}`.
  No `{{ recipient.* }}`, no `{{ variables.* }}`. Unknown placeholders
  silently render as empty string.

## Golden rules

- Use only `senda_prod_…` / `senda_test_…` for raw API keys.
- Never put environment in `ref`. `ref` stays `tenant:workspace:templateType`.
- Never infer environment from workspace code/name. Use route, header, or token prefix.
- Raw API key is shown ONLY on creation. Webhook secret is shown ONLY on creation.
- `_system` is a control workspace; it cannot receive sends (`SYSTEM_WORKSPACE_BLOCKED`).
- Test recipient policies and runtime reset exist only in `env=test`.
- The token prefix decides the environment first; if the workspace resolved from `ref` does not match the API key's workspace, the send fails 403 `FORBIDDEN` with message "API key scope does not match template workspace".
- Inherited **injectors** are read-only in child workspaces; mutation
  attempts return 403 `READ_ONLY_INHERITED_INJECTOR`. To diverge, create a
  local one with the same `name` — the workspace will win by scope priority.
- Shared **adapters and identities** are read-only too, but use a different
  signal: 403 `FORBIDDEN` with message "shared resource is read-only". They
  are *granted* by `_system` (not inherited by name), so the divergence
  path is to ask `_system` for access changes — not to create a same-name
  adapter locally.

## First moves when in doubt

1. `senda_list_endpoints` to scan available operations.
2. `senda_describe_endpoint` for body shape, query params, response.
3. `senda_call_endpoint` to execute.

The OpenAPI source of truth lives at `cmd/senda/docs/openapi.yaml` for
cross-checks; the generated MCP descriptions are usually enough.

## Reference loader

Load the file matching the user's intent. Each is small and self-contained.

| If the task is about… | Load |
|---|---|
| Auth selection, error model, pagination, common headers | `references/operating-via-mcp.md` |
| Onboarding, tenants, workspaces (shared & env-scoped), policies, runtime reset, dashboards | `references/management-tenants-workspaces.md` |
| Roles, permissions matrix, members CRUD per scope | `references/rbac-and-members.md` |
| Template types, templates, fork, disable/enable, sharing, inheritance, `is_bulk`, bulk vs transactional | `references/templates-types-and-templates.md` |
| Template versions, locales, lifecycle, preview, bulk-send, test-send | `references/versions-locales-and-builder.md` |
| Authoring a template body — copy-paste MJML blocks, where to drop variables, gomjml version, system variables (`{{ system.unsubscribe_url }}`, `{{ system.preferences_url }}`), end-to-end workflow | `references/building-a-template.md` |
| Injectors (DB and code) — definitions, fields, precedence, inheritance | `references/injectors.md` |
| Adapters (SES, Gmail, SMTP), identities, sharing, auto-provisioning | `references/adapters-and-identities.md` |
| API keys, `POST /send`, `POST /send/batch`, emails query, CSV export | `references/api-keys-and-data-plane.md` |
| Webhooks (outbound HMAC), tracking pixel, SES inbound provider webhook | `references/webhooks-and-events.md` |
| External integration (profiles, bootstrap, session, capabilities, embed) | `references/external-integration.md` |
| Unsubscribe flow, List-Unsubscribe headers, one-click opt-out, preference center, suppression levels, RFC 8058 | `references/templates-types-and-templates.md` (bulk vs transactional) + `docs/EMAIL_FLOWS.md#unsubscribe` |
| Audit log, global config, suppression, media thumbnail proxy | `references/audit-config-and-extras.md` |
| Embedding Senda as a Go library, SDK extension points | `references/sdk-extension-points.md` |
| Scope chain, resolution algorithm, inheritance edge cases | `references/resolution-and-inheritance.md` |
| Mental model overview, environments, scopes basics | `references/platform-overview.md` |

If a task spans several domains, load the relevant references together — they
are designed to compose.

## Skill maintenance

If you change repo behavior in any of these areas, you MUST update the
corresponding skill files in the same PR — otherwise this skill will
silently misguide its consumers.

| When this changes in the repo… | Update in the skill |
|---|---|
| MJML output of any visual-builder block (`web/src/components/templates/mjml-editor.tsx`, `text-block-mjml.ts`, `video-block.ts`, …) or the variable engine (`internal/service/variable_renderer.go`) | `references/building-a-template.md`, `references/versions-locales-and-builder.md` |
| Allowed/forbidden MJML or HTML tags in `body_mjml` (e.g. adding a new `mj-*` tag the builder emits, changing `<mj-raw>` semantics) | `scripts/mjml-check.sh` (rule patterns) and `scripts/mjml-check.test.sh` (fixtures) |

The repo's `AGENTS.md` (`CLAUDE.md`) carries the canonical hard-rule
phrasing; this table is a per-skill mirror so consumers loading only
`skills/senda/` still see the obligation.
