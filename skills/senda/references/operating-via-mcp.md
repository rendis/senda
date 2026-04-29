# Operating Senda via MCP

How an agent drives a deployed Senda instance through `mcp-openapi-proxy`.

## Tooling

- `senda_list_endpoints` — enumerate what's available.
- `senda_describe_endpoint` — fetch path, method, request schema, response
  schema, examples.
- `senda_call_endpoint` — execute the call.

Always discover before guessing. Body shapes, especially in management,
include optional fields the agent will not infer correctly.

## Picking the right plane

| Goal | Plane | Auth |
|---|---|---|
| Configure the platform (tenants, workspaces, templates, keys…) | Management | OIDC bearer |
| Send mail or query delivery from a backend | Data plane | API key (`senda_prod_` / `senda_test_`) |
| Drive an embedded builder / external portal | External | Profile auth + `X-Senda-Environment` |

Mismatches:

- API key on `/api/v1/manage/...` → 403 `FORBIDDEN` ("management API requires
  OIDC authentication"). The auth middleware accepts the API key, then
  `OIDCOnly()` rejects it.
- OIDC on `/api/v1/send` → 401 (the data plane does not accept OIDC).
- External request without `X-Senda-Environment` → 400.

## Auth shapes

### OIDC

```
Authorization: Bearer <id_token>
```

The token must come from the OIDC provider configured under
`config.auth.oidc.discovery_url`. First Superadmin is bootstrapped with the
calling token via `POST /api/v1/onboarding/setup`.

### Raw API key

```
Authorization: Bearer senda_prod_<random>
Authorization: Bearer senda_test_<random>
```

The prefix decides the environment and resolves the workspace. It bypasses
RBAC; whoever holds the key can send and query for that workspace
environment, period.

### External integration

Required:

```
X-Senda-Environment: prod | test
x-senda-external-token: <token>
<other headers required by the registered auth_method_name>
```

The token header name is fixed (`x-senda-external-token`); profile auth
methods receive that value as `ExternalIntegrationRequest.Token`. Profile
`required_headers` are additional context, not the place for the token.

The middleware:

1. Loads the profile by `:profile_slug`.
2. Calls the registered `auth_method_name` with all headers.
3. On success, calls `resolver_name` to map the request to a workspace.
4. Computes effective capabilities. Read-only resolution forces `_system`
   as the effective workspace.

## Pagination

Cursor-based; never offset.

```
GET /api/v1/manage/.../emails?cursor=<base64json>&limit=50
```

- `cursor`: opaque base64 of `{t, id}`. Send back what the previous response
  returned in `next_cursor`. Empty/missing means start from the top.
- `limit`: default 25, max 100.

Endpoints without true pagination (some list-with-inheritance endpoints) still
accept `cursor`/`limit` but ignore them and return the full page.

## Error model

Errors return a JSON body shaped like:

```json
{
  "error": {
    "code": "STRING_CODE",
    "message": "Human-readable text",
    "details": [
      { "field": "field_name", "message": "field-specific message" }
    ]
  }
}
```

`details` is an **array of `{field, message}` objects**, omitted when empty.
There is no nested `field_errors` key — the field-error list IS `details`.

Common codes worth memorizing:

| HTTP | Code | When |
|---|---|---|
| 400 | `BAD_REQUEST` | Malformed request: invalid JSON, missing path param, bad UUID, etc. |
| 422 | `VALIDATION_ERROR` | Body parsed but failed field validation. Response includes `details[]` with `{field, message}` per failing field. |
| 401 | `UNAUTHORIZED` | Missing/invalid token, or API key on management. |
| 403 | `FORBIDDEN` | Role insufficient, or external read-only mutation. |
| 403 | `FORBIDDEN` | API key's workspace does not match the workspace resolved from `ref` — message: "API key scope does not match template workspace". Token prefix selects environment first, so this is typically a `ref` aimed at a different workspace, not a literal prod-vs-test mix-up. Same code is used for other RBAC denials; disambiguate by the message. |
| 403 | `READ_ONLY_INHERITED_INJECTOR` | Trying to edit an inherited injector. |
| 403 | `FORBIDDEN` | Shared resource is read-only — message: "shared resource is read-only". Returned when a child workspace attempts to mutate an adapter or identity granted by `_system`. |
| 404 | `NOT_FOUND` | Or scope hides the resource (e.g. global injector queried from workspace). |
| 409 | `CONFLICT` | Generic; check `details`. |
| 409 | `HAS_PUBLISHED_VERSION` | DELETE template blocked by a published version. |
| 409 | `VERSION_NOT_DRAFT` | Edit/delete on non-draft version. |
| 409 | `TEMPLATE_ALREADY_LOCAL` | Fork on already-local template. |
| 409 | `SYSTEM_WORKSPACE_PROTECTED` | Trying to delete `_system`. |
| 409 | `TEST_ENVIRONMENT_REQUIRED` | Runtime reset on env=prod. |
| 403 | `WORKSPACE_LOCAL_TEMPLATES_DISABLED` | Policy denies local templates in this workspace. |
| 403 | `WORKSPACE_INHERITED_TEMPLATE_FORKS_DISABLED` | Policy denies forks. |
| 403 | `WORKSPACE_LOCAL_INJECTORS_DISABLED` | Policy denies local injectors. |
| 409 | `TEMPLATE_DISABLED` | Send hit a disabled template. |
| 422 | `NO_ADAPTER` / `NO_DEFAULT_IDENTITY` / `DOMAIN_NOT_VERIFIED` | Send misconfiguration. |
| 422 | `NO_PUBLISHED_VERSION` | Send hit a template without a published version. |
| 422 | `TEST_RECIPIENT_POLICY_UNCONFIGURED` | Send in test without a configured policy. |
| 422 | `SUPPRESSED` | Reserved; `POST /send` does NOT return this. Per-recipient suppression appears as `tracking_ids[].status = "suppressed"` inside a 202 response. |
| 422 | `SYSTEM_WORKSPACE_BLOCKED` | Send targeted `_system`. |
| 429 | `RATE_LIMITED` | Adapter rate limit hit. |
| 501 | `NOT_IMPLEMENTED` | Optional feature (e.g. test-send service) not configured on this deployment. |

Always read `details` — it carries the `[{field, message}]` array for
`VALIDATION_ERROR`, the failing step for provisioning, etc.

## Request id correlation

The server attaches `X-Request-ID` to every response. Surface it in logs and
in incident reports for the operator.

## Workflow shapes

### First-time bootstrap

1. `GET /api/v1/onboarding/status` — `{"needs_onboarding": true}`.
2. `POST /api/v1/onboarding/setup` with the OIDC token of the future Superadmin.
3. From now on, manage with OIDC.

### Bring up a sending workspace

1. `POST /api/v1/manage/tenants` (Superadmin) — creates tenant + `_system` pair.
2. `POST /api/v1/manage/tenants/:tenant/workspaces` (TenantAdmin) — creates
   the logical workspace (prod+test pair).
3. `POST .../adapters` (WorkspaceAdmin) — adapter on the desired workspace.
   For SES: `POST .../adapters/:id/auto-provision-tracking`,
   `POST .../adapters/:id/identities/sync`,
   `POST .../adapters/:id/identities/:identity_id/set-default`.
4. `POST .../template-types` then `POST .../templates`.
5. `POST .../templates/:id/versions` (draft) — fill subject, from_name,
   body_mjml, default_locale.
6. Optional: `POST .../versions/:vid/locales/:locale` for translations.
7. `POST .../versions/:vid/publish`.
8. `POST .../api-keys` — store the raw key NOW; you cannot retrieve it later.

### Send and inspect

1. `POST /api/v1/send` with `Authorization: Bearer senda_prod_…`.
2. `GET /api/v1/emails?recipient=...` to find the message.
3. `GET /api/v1/emails/:tracking_id/events` for the lifecycle events.

### Screenshot a template

`GET /api/v1/manage/tenants/{tenant_code}/workspaces/{workspace_code}/templates/{template_id}/screenshot?viewport=desktop|mobile`

The endpoint returns `Content-Type: image/png` with the raw bytes. The MCP proxy (`mcp-openapi-proxy`) detects `image/*` responses and emits a native `mcp.ImageContent` block alongside the JSON envelope, so capable clients render the image inline. To get both desktop and mobile, call the tool twice — one image per call (the proxy surfaces one ImageContent per response).

Placeholders are not interpolated — the image shows the template structure, not what a real recipient would see. For real-data previews, use `test-send`.

If the response comes back as just a base64 blob inside the JSON envelope, the proxy did not detect the image content type — verify the deployment is running a recent `mcp-openapi-proxy` build and the endpoint actually returns `Content-Type: image/png`.

Full parameter reference and error codes: see `templates-types-and-templates.md` → "Screenshot endpoint".

### External builder

1. `GET /api/v1/external/:profile_slug/bootstrap` for CSP / `frame_ancestors`.
2. `GET /api/v1/external/:profile_slug/tenants/:t/workspaces/:w/session` —
   capabilities + read-only flag.
3. Use only the endpoints whose required capability is granted.

## Practical tips

- When unsure of the body shape: `senda_describe_endpoint <method> <path>`.
- When the response indicates a missing capability or scope, recheck
  `rbac-and-members.md`.
- When the OpenAPI looks stale: trust `senda_describe_endpoint` (regenerated
  per build); the YAML in `cmd/senda/docs/openapi.yaml` is for cross-check.
- For destructive ops (delete tenant, delete workspace, revoke key), read the
  domain reference before calling — several have guards (`delete_blocked_reason`).
