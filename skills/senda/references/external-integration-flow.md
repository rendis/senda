
# External Integration Flow

Senda exposes an embeddable external builder/editor surface under:

`/api/v1/external/:profile_slug/...`

This is separate from the management plane and does not use OIDC for normal request auth.

## Building blocks

### External profile

Configured in global config. A profile defines:

- `slug`
- `auth_method_name`
- `resolver_name`
- `allowed_origins`
- `allowed_headers`
- `required_headers`
- capability flags
- enabled/disabled status

### Bootstrap endpoints

Public bootstrap endpoints:

- `GET /api/v1/external/:profile_slug/bootstrap`
- `GET /api/v1/external/:profile_slug/environments/:environment/bootstrap`

Use them to obtain frame ancestry / embed bootstrapping metadata.

### Authenticated external workspace surface

Base path:

`/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/...`

Key endpoints include:

- `/session`
- template type reads
- template version reads/updates/publish
- locale CRUD
- MJML preview
- test-send
- injector reads
- workspace policy read

## Required header

Every authenticated external integration request must include:

`X-Senda-Environment: prod|test`

If the header is missing or invalid, the request is rejected.

## Request pipeline

1. profile slug resolves
2. profile must be enabled
3. required headers are validated
4. `X-Senda-Environment` is validated and normalized
5. configured auth method authenticates
6. configured workspace resolver maps the request to a workspace
7. effective permissions are combined from profile + auth result
8. request continues against the external surface

## Auth method responsibilities

An auth method should:

- validate the external caller's credentials/token
- normalize permissions
- return caller context if needed by the resolver

It should **not** decide the final workspace.

## Workspace resolver responsibilities

A workspace resolver should:

- map the normalized request + auth result to a workspace
- optionally force read-only fallback

Return rules:

- `WorkspaceCode` + `ReadOnly=false` → normal workspace access
- `ReadOnly=true` → middleware forces effective workspace `_system`

## Permissions model

Available capability flags include:

- `ListTemplates`
- `ViewVersions`
- `EditVersions`
- `PublishVersions`
- `TestSend`
- `BuilderAccess`
- `MetadataAccess`
- `LocaleAccess`

External routes are individually guarded by these capabilities.

## Agent rules

- Do not assume OIDC on the external surface.
- Do not assume the path workspace is the effective workspace; the resolver may override or force read-only fallback.
- Always send `X-Senda-Environment`.
- Use `/session` to understand effective permissions and whether the request is read-only.
- If you mutate a visual template version or locale, preserve existing `editor_data` instead of sending `body_mjml` alone by default.
- For builder payloads, preview request bodies, and color-format rules, load `references/template-builder-contract.md`.
