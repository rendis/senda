# Template Builder Contract

This reference explains how to work safely with Senda template versions when the task touches the
visual builder, `editor_data`, MJML preview, or color values.

## What problem this solves

Use this reference when you need to avoid these mistakes:

- treating a visual-builder draft like plain MJML-only content
- overwriting or dropping `editor_data` during version updates
- sending the wrong request shape to preview endpoints
- assuming colors are always stored as hex

## Quick reference

| Task | Request shape | Key rule |
|---|---|---|
| Create/update version | version payload | Preserve `editor_data` if the version is visual-builder based |
| Create/update locale | locale payload | Preserve `editor_data` for visual locale edits |
| Preview MJML | `{ "mjml": "..." }` | Do not send `editor_data` |
| Visual draft edit | read current version first | Do not replace visual state with `body_mjml` alone by default |

## Payload contract

### Template version create or update

Use these fields when creating or updating a version:

- `subject`
- `preview_text`
- `from_name`
- `reply_to?`
- `body_mjml`
- `default_locale`
- `editor_data?`

The HTTP request types in the product accept exactly that shape for version mutations.

### Locale upsert

Locale create/update accepts:

- `subject?`
- `preview_text?`
- `from_name?`
- `body_mjml?`
- `editor_data?`

### Preview

MJML preview is **not** the same contract as a version update.

Use:

```json
{
  "mjml": "<mjml>...</mjml>"
}
```

Do **not** send `editor_data` to preview endpoints.

## Operating rules

### MJML-only mode

Use MJML-only mode when the task is explicitly source/MJML editing and there is no requirement to
preserve visual-builder state.

- update `body_mjml`
- omit `editor_data` if the work is intentionally MJML-only
- use preview after changes

### Visual mode

Use visual mode when the current version already has `editor_data` or when the user is working with
the builder/editor experience.

- read the current version first
- if `editor_data` already exists, preserve it during updates
- send `body_mjml` plus `editor_data` for visual-version mutations
- do **not** replace a visual draft with `body_mjml` alone unless the goal is to discard builder state

### Safe mutation sequence

For existing versions:

1. read the current version
2. inspect whether `editor_data` is present
3. choose MJML-only or visual mode deliberately
4. update the version or locale with the matching payload shape
5. run MJML preview
6. only then proceed to test-send or publish

### Public-contract boundary

Do **not** treat `editor_data` as a stable public schema block-by-block.

- It is safe to preserve and round-trip it.
- It is safe to reason about it operationally.
- It is **not** safe to document or hardcode every internal builder block as if it were a public API contract.

## Color contract

### Preferred manual input

For examples, prompts, and manual entry, prefer:

- `#RRGGBB`
- `#RGB`

These are the clearest formats for agents and operators.

### Persisted builder reality

Do **not** assume stored colors are hex-only.

Builder content can persist colors through different paths:

- HEX from structured color pickers
- CSS `rgb(...)` / `rgba(...)` from rich-text or inline style sources

### Agent rule

Agents working with template payloads or downstream renderers must follow these rules:

- do **not** claim that Senda enforces hex-only colors unless a specific endpoint or validator says so
- do **not** normalize colors with fragile string hacks
- if a consumer/renderer touches color values, it must tolerate common CSS color strings

## Common mistakes

- Sending preview requests with version-style bodies that include `editor_data`
- Updating a visual-builder draft with only `body_mjml` and accidentally discarding builder state
- Documenting builder internals block-by-block as if they were a stable public API
- Claiming that the system only supports HEX colors without checking the actual endpoint or renderer behavior

## Examples

### Update a visual version while preserving `editor_data`

```json
{
  "subject": "Welcome, {{ user.first_name }}",
  "preview_text": "Your account is ready",
  "from_name": "Senda",
  "reply_to": "support@example.com",
  "body_mjml": "<mjml>...</mjml>",
  "default_locale": "en",
  "editor_data": {
    "...": "preserve the existing builder document"
  }
}
```

### Update in MJML-only mode

```json
{
  "subject": "Welcome",
  "preview_text": "Your account is ready",
  "from_name": "Senda",
  "body_mjml": "<mjml>...</mjml>",
  "default_locale": "en"
}
```

### Preview request

```json
{
  "mjml": "<mjml><mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>"
}
```

### Minimal locale upsert

```json
{
  "subject": "Bienvenido",
  "preview_text": "Tu cuenta ya está lista",
  "body_mjml": "<mjml>...</mjml>",
  "editor_data": {
    "...": "only include this when preserving visual-builder state"
  }
}
```
