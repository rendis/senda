# Template Versions, Locales & Builder

The version is what actually renders an email. Templates can have many
versions; only one is `published` at any moment.

## Data model

### `TemplateVersion`

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK (UUIDv7). |
| `template_id` | uuid | FK. |
| `version_number` | int | Auto-assigned `MAX + 1` per template, inside a tx with row lock. **Not semver.** |
| `status` | enum | `draft` \| `published` \| `archived`. |
| `subject` | string | Plain-text Senda template. |
| `from_name` | string | Plain-text Senda template. |
| `preview_text` | string | Plain-text Senda template. |
| `reply_to` | string? | Optional. |
| `body_mjml` | text | MJML source; rendered then compiled at send/preview time. |
| `default_locale` | string | e.g. `"es"`. |
| `editor_data` | jsonb | UI builder state; opaque to the server. |
| `created_by` | uuid? | FK to `members`. |
| `published_at`, `archived_at` | ts | Set at status transitions. |
| `created_at`, `updated_at` | ts | |

DB constraints:

- `tv_version_unique UNIQUE (template_id, version_number)`.
- `tv_one_published EXCLUDE (template_id WITH =) WHERE (status = 'published')`
  — Postgres prevents two `published` versions per template at any moment.

### `TemplateVersionLocale`

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | PK. |
| `template_version_id` | uuid | FK with `ON DELETE CASCADE`. |
| `locale` | string | e.g. `"es"`, `"pt-BR"`. |
| `subject?`, `preview_text?`, `from_name?` | string? | `null` = inherit version default. |
| `body_mjml?` | text? | `null` = inherit version default. |
| `editor_data` | jsonb | |
| `created_at`, `updated_at` | ts | |

There is no `html` column (compiled at send time) and no `text` column
(plain-text alternative not stored separately). `reply_to` exists only at
version level, not per locale.

## Endpoints

Apply to `<ws>` and `<envWS>` (workspace) and to `/api/v1/manage/global/...`
(global, RBAC = superadmin). Workspace RBAC shown.

### Versions

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `<ws>/templates/:tid/versions` | viewer+ | No pagination; returns all. |
| POST | `<ws>/templates/:tid/versions` | editor+ | `{subject, from_name, body_mjml, default_locale, preview_text?, reply_to?, editor_data?}`. Created as `draft`. |
| GET | `<ws>/templates/:tid/versions/:vid` | viewer+ | |
| PUT | `<ws>/templates/:tid/versions/:vid` | editor+ | **Draft only**; 409 `CONFLICT` ("only draft versions can be updated") otherwise. Patch parcial. |
| POST | `<ws>/templates/:tid/versions/:vid/clone` | editor+ | Exact clone: copies all fields and ALL locales of source into a new draft with the next `version_number`. |
| POST | `<ws>/templates/:tid/versions/:vid/publish` | admin | Locks the template, archives the previous published version, sets target to `published`. Source version must be `draft`. |
| DELETE | `<ws>/templates/:tid/versions/:vid` | admin | Draft only; 409 `VERSION_NOT_DRAFT` otherwise. Cascades locales. |

### Locales

| Method | Path | RBAC | Notes |
|---|---|---|---|
| GET | `<ws>/templates/:tid/versions/:vid/locales` | viewer+ | `{items: [...]}`. |
| GET | `<ws>/templates/:tid/versions/:vid/locales/:locale` | viewer+ | |
| POST | `<ws>/templates/:tid/versions/:vid/locales/:locale` | editor+ | Upsert (`ON CONFLICT DO UPDATE`). 201 on insert. |
| PUT | `<ws>/templates/:tid/versions/:vid/locales/:locale` | editor+ | Same upsert handler; 200. |
| DELETE | `<ws>/templates/:tid/versions/:vid/locales/:locale` | editor+ | |

### Builder helpers

| Method | Path | RBAC | Notes |
|---|---|---|---|
| POST | `<ws>/templates/:tid/preview-mjml` | viewer+ | `{mjml}`. Substitutes static injector preview values, then compiles via gomjml. Returns `{html}`. Stateless. |
| POST | `<ws>/templates/:tid/test-send` | editor+ | `{recipient_email, variables?, injectors?, locale?}`. Requires published version. 501 if test-send service not configured. |
| POST | `<ws>/templates/:tid/bulk-send` | editor+ | `{items: [...]}` up to `max_items` (100 default). Local templates only. |

> Looking for ready-to-use MJML blocks (text, button, image, banner, list,
> video, divider, spacer) and a step-by-step "compose a body from zero"
> workflow? Load `building-a-template.md`. This file owns the lifecycle and
> reference shape of versions/locales; that one owns authoring.

## Builder syntax — exhaustive

Senda has its own substitution engine in
`internal/service/variable_renderer.go`. Pattern:

```
{{ <namespace>.<key>[.<field>] }}
```

Two namespaces only:

- `event.<name>` — comes from the request `variables` map. Declared (not
  enforced) by the template type's `variable_schema`; missing/unknown
  variables render as empty string silently.
- `injector.<name>.<field>` — from the resolved injector tree. See
  `injectors.md` for precedence.

Anything else (e.g. `{{ recipient.email }}`, `{{ variables.x }}`,
`{{ tenant.code }}`) is **silently replaced with empty string**. There is no
`text/template`, no `html/template`, no Pongo, no helpers, no conditionals,
no loops, no partials.

`subject`, `preview_text`, `from_name`, and `reply_to` are processed by the
same engine but are NOT compiled by gomjml. `body_mjml` is rendered by the
engine first, then handed to gomjml; the compiler additionally rewrites video
thumbnails and caches by SHA-256 of post-rewrite MJML.

`editor_data` is the visual builder's serialized state — block tree (text,
button, image, divider, spacer, banner, video, list, plus preserved MJML code
blocks for syntax the visual editor cannot represent). Treat it as opaque from
the agent perspective.

## Resolution chain at send time

1. Request hits `POST /api/v1/send`. Workspace and environment derived from
   the API key. `ref` is split into `tenant:workspace:template_type_slug`.
2. `ChainResolver` builds `[workspace, _system]` (cached 5 min).
3. `TemplateResolver` finds the type, then the template, then the published
   version, applying scope priority (workspace > `_system`).
4. Locale fallback: requested `locale` → language prefix (`"es-CO"` → `"es"`)
   → `default_locale`.
5. Body is rendered, MJML compiled, and the email is queued.

If no published version exists → 422 `NO_PUBLISHED_VERSION`. If the template
is disabled → 409 `TEMPLATE_DISABLED`. If `_system` is the resolved workspace
through a buggy `ref` → 422 `SYSTEM_WORKSPACE_BLOCKED`.

## Operational rules

- **One published version per template** at any time, enforced at the DB
  level. Publishing one auto-archives the previous one in the same transaction.
- **No unpublish endpoint**. To "stop using" a published version, publish a
  new one or `disable` the template.
- **Drafts are mutable**; published and archived are immutable.
- **Cloning** is the canonical "edit from existing": clone → edit draft → publish.
- **Locale upsert**: same handler for POST and PUT; safe to retry.

## Flujo end-to-end

1. `senda_call_endpoint POST <ws>/template-types` (if needed).
2. `senda_call_endpoint POST <ws>/templates` with the type id.
3. `senda_call_endpoint POST <ws>/templates/:tid/versions` (draft).
4. Optional: `senda_call_endpoint POST <ws>/.../locales/:locale` for each
   localization.
5. `senda_call_endpoint POST <ws>/.../preview-mjml` to validate body and
   compute static-injector preview.
6. `senda_call_endpoint POST <ws>/.../publish` (admin role).
7. Send via data plane — see `api-keys-and-data-plane.md`.

## Cuándo consultar OpenAPI / MCP

- Body for `POST /versions` and `POST/PUT /locales/:locale` — the partial
  PATCH semantics differ between create and update.
- Bulk-send and test-send response shapes.

## Gotchas

- **No placeholder validation**: a typo in `{{ injector.foo.bar }}` is not
  flagged at save or publish; it just silently empties at render. Run
  `preview-mjml` before publishing.
- **MJML errors fail at send time**, not at save: the worker fails the email
  permanently. Always `preview-mjml` first.
- **Cache after publish**: known bug — the resolved-template cache is not
  invalidated after `publish`. New sends may keep using the previously
  published version for up to 5 minutes (`TTL`). Workarounds: warm-restart of
  the resolver cache (deploy-time only) or accept eventual consistency.
- **`from_email` is NOT on the version** anymore (column dropped in
  migration 023). The sender comes from the adapter's default identity (or
  `template_type.sender_identity_id`).
- **`text` plain-text alternative is not stored**. Sends include only the
  HTML body produced from MJML.
- **DELETE version** is draft-only; archived versions stay around for audit.
- **Publish flow is concurrent-safe** at the DB level (EXCLUDE constraint).
  Two concurrent publishes will fail one of them with a constraint violation.
