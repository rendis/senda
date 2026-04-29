# External Integration

The external surface lets a third-party portal or embedded builder talk to
Senda without OIDC and without raw API keys. Profiles configure how a request
is authenticated and which workspace it maps to.

## Concepts

- **Profile** — a named integration (`profile_slug`) with auth rules and a
  workspace resolver. Configured globally via the global config endpoint.
- **External auth method** — Go SDK extension implementing
  `sdk.ExternalAuthMethod` (`Name`, `Description`, `Authenticate`). The
  profile selects one by `auth_method_name`. (Internally backed by
  `internal/port`, but consumers must import the `sdk` package — the
  `internal/...` tree is not importable from outside the module.)
- **External workspace resolver** — Go SDK extension implementing
  `sdk.ExternalWorkspaceResolver` (`Name`, `Description`, `ResolveWorkspace`).
  Selected by `resolver_name`. Same import rule.
- **Capabilities** — granular permissions returned by the auth method:
  `builder_access`, `list_templates`, `view_versions`, `edit_versions`,
  `publish_versions`, `locale_access`, `test_send`, `metadata_access`.
  All eight are configurable in profiles and exposed in `/session`;
  `metadata_access` exists as a flag but currently has no endpoint that
  guards on it in `routes_external.go`.
- **Read-only fallback** — when the resolver returns `ReadOnly = true`, the
  middleware forces the effective workspace to `_system` and blocks any
  mutation (`RequireExternalMutation` rejects).

## Endpoints

### Bootstrap (no auth, no scope)

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/api/v1/external/:profile_slug/bootstrap` | None | Returns CSP `frame_ancestors` + profile metadata. |
| GET | `/api/v1/external/:profile_slug/environments/:environment/bootstrap` | None | Same handler, with environment in path. |

### Scoped (`/api/v1/external/:profile_slug/tenants/:tenant_code/workspaces/:workspace_code/...`)

Required headers on every scoped request:

```
X-Senda-Environment: prod | test
x-senda-external-token: <token>
<other headers required by the registered auth_method_name>
```

The middleware always reads the token from `x-senda-external-token` (the
constant `ExternalIntegrationTokenHeader`) and rejects requests that put it
elsewhere or as a `?token=` query param. Profile-required headers are
additional, not a substitute. The auth method receives the token via
`ExternalIntegrationRequest.Token` and the rest via `Headers`.

| Method | Path | Capability | Notes |
|---|---|---|---|
| GET | `.../session` | `builder_access` | `{read_only, effective_workspace_code, permissions: {...}}`. |
| GET | `.../template-types` | `list_templates` | |
| GET | `.../template-types/:slug` | `list_templates` | |
| GET | `.../template-types/:slug/templates` | `list_templates` | |
| GET | `.../templates/:tid/versions` | `view_versions` | |
| GET | `.../templates/:tid/versions/:vid` | `view_versions` | |
| PUT | `.../templates/:tid/versions/:vid` | `edit_versions` + mutation | Rejected when `read_only`. |
| POST | `.../templates/:tid/versions/:vid/publish` | `publish_versions` + mutation | |
| GET | `.../templates/:tid/versions/:vid/locales` | `locale_access` | |
| GET | `.../templates/:tid/versions/:vid/locales/:locale` | `locale_access` | |
| POST | `.../templates/:tid/versions/:vid/locales/:locale` | `locale_access` + mutation | |
| PUT | `.../templates/:tid/versions/:vid/locales/:locale` | `locale_access` + mutation | |
| DELETE | `.../templates/:tid/versions/:vid/locales/:locale` | `locale_access` + mutation | |
| POST | `.../templates/:tid/preview-mjml` | `builder_access` | |
| POST | `.../templates/:tid/test-send` | `test_send` + mutation | |
| GET | `.../injectors` | `builder_access` | |
| GET | `.../injectors/:name` | `builder_access` | |
| GET | `.../policies` | `builder_access` | |

There are no mutation endpoints on injectors or policies in the external
surface — mutation lives in the management plane.

## Profiles — managed via the global config

There is no dedicated CRUD for profiles. They are part of the global config:

```
GET  /api/v1/manage/config            (superadmin)
PUT  /api/v1/manage/config            (superadmin)
```

The `external_integrations` field is an **object with a `profiles` array**
(not a bare list). Sending it replaces the full profile list (no per-item
merge). Body shape (JSON):

```json
{
  "external_integrations": {
    "profiles": [
      {
        "slug": "<profile_slug>",
        "name": "…",
        "description": "…",
        "enabled": true,
        "auth_method_name": "<registered ExternalAuthMethod name>",
        "resolver_name": "<registered ExternalWorkspaceResolver name>",
        "allowed_origins": ["https://my.portal"],
        "allowed_headers": ["X-Custom-Token", "X-Tenant-Hint"],
        "required_headers": ["X-Tenant-Hint"],
        "capabilities": {
          "list_templates":   true,
          "view_versions":    true,
          "edit_versions":    false,
          "publish_versions": false,
          "test_send":        false,
          "builder_access":   true,
          "metadata_access":  false,
          "locale_access":    true
        }
      }
    ]
  }
}
```

Sending `external_integrations` as a bare array fails with 400 `BAD_REQUEST`
during JSON binding. Every entry in `required_headers` must also be listed
in `allowed_headers` — otherwise `Validate()` rejects the body with 422
`VALIDATION_ERROR`.

`auth_method_name` and `resolver_name` must match SDK-registered
implementations. There is no API to enumerate them; the deployer knows what
the codebase ships.

## Flujo end-to-end — wire an embedded builder

1. As Superadmin: `senda_call_endpoint PUT /api/v1/manage/config` with the
   `external_integrations` list including a new profile.
2. Confirm: `GET /api/v1/manage/config` shows the profile.
3. From the embed: `GET /api/v1/external/:slug/environments/:env/bootstrap`
   to fetch CSP frame ancestors.
4. From the embed: `GET /api/v1/external/:slug/tenants/:t/workspaces/:w/session`
   with `X-Senda-Environment` and the auth headers.
5. Drive the builder using only endpoints whose capability is granted.

## Cuándo consultar OpenAPI / MCP

- Exact body for `PUT /config` — particularly the nested `external_integrations`
  list and the capability flag names.
- The session response shape (capability map varies by deployment).

## Gotchas

- **Read-only fallback** does not return 403 on read operations; it forces
  the effective workspace to `_system`. UI must check `read_only` before
  showing edit affordances.
- **Mutations require capability AND `read_only = false`**. Capability alone
  is not enough.
- **Profile is configured by replacing the list**: be careful with concurrent
  PUTs to `/config` from different operators — last writer wins.
- **`X-Senda-Environment`** is mandatory on scoped routes; missing → 400.
- **`auth_method_name` / `resolver_name`** must exist in the running binary.
  If either is missing at request time, the middleware returns 500
  `INTERNAL_ERROR` (with a descriptive message). This is a deployment
  configuration error, not a token/proxy problem — verify the SDK
  registrations match the profile fields exactly.
- **No external-surface mutations on injectors or policies**. If your portal
  needs them, expose a server-side proxy that calls the management plane.
- **Bootstrap is unauthenticated**; do not return secrets there. The
  bootstrap response is meant only for CSP setup.
