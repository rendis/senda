# Building a Template — End-to-End

A practical guide to actually composing a Senda email body. Use this when
the task is "create / edit a template version", not when configuring
infrastructure. For lifecycle (draft → published, fork, locales), see
`versions-locales-and-builder.md`. For injector rules, see `injectors.md`.

## Engine and version

- Compiler: `gomjml` v0.11.0 (Go port of MJML; MJML 4.x semantics).
- Senda **renders Senda variables first**, then hands the result to gomjml.
- Validated tags from the visual builder (always work):
  `mjml`, `mj-body`, `mj-section`, `mj-column`, `mj-text`, `mj-button`,
  `mj-image`, `mj-divider`, `mj-spacer`, `mj-hero`. Other gomjml-supported
  tags (`mj-head`, `mj-attributes`, `mj-style`, `mj-class`, `mj-raw`,
  `mj-table`, `mj-social`, ...) compile fine and the visual editor preserves
  them as **MJML code** blocks when it cannot map them to structured controls;
  preview them before publishing. **`<mj-raw>` accepts small HTML snippets
  only, never a full HTML document — see "Anti-pattern: HTML wrappers" below.**

## Variable syntax — exhaustive

Senda has its own substitution engine; only three namespaces exist:

| Token | Source | Example |
|---|---|---|
| `{{ event.<name> }}` | request `variables` map (declared by `template_type.variable_schema`, NOT enforced at send) | `{{ event.first_name }}` |
| `{{ injector.<name>.<field> }}` | resolved injector tree (DB + code) | `{{ injector.brand.logo_url }}` |
| `{{ system.* }}` | platform-injected per-send variables (see table below) | `{{ system.unsubscribe_url }}` |

### System variables

These are injected automatically by the send pipeline; the caller does not supply them.

| Token | Description |
|---|---|
| `{{ system.unsubscribe_url }}` | URL to the unsubscribe landing page (`/u/{token}`). Empty string when `is_bulk = false`. Renders to a one-time HMAC-signed link bound to the recipient and template type. |
| `{{ system.preferences_url }}` | URL to the preference center (`/u/{token}/preferences`). Empty string when `is_bulk = false`. Same token base as `unsubscribe_url`; recipient can manage all subscriptions. |
| `{{ system.workspace_name }}` | Human-readable workspace name. Useful for footer branding. Empty string when `is_bulk = false`. |

`variable_schema` documents the contract for callers and tooling but is
**not** validated by the send pipeline. Missing or misspelled `event.*`
values render as empty string at send time. Treat the schema as a contract
your senders must respect; do not rely on the server to reject unknown or
omitted variables.

Anything else (`{{ recipient.email }}`, `{{ variables.x }}`, `{{ tenant.code }}`,
`{{ env }}`) renders as **empty string** silently. There are no helpers,
conditionals, loops, partials, or filters.

### Where you can place variables

- `subject`, `from_name`, `preview_text`, `reply_to` (version-level fields).
- Anywhere in `body_mjml`: text content, attribute values (`href`, `src`,
  `alt`, `background-url`), and locale overrides.
- Inside `<mj-button href="...">` / `<mj-image src="...">` — both attributes
  and inner text.

The renderer rewrites placeholders **before** MJML compilation, so a
variable that yields a malformed URL or invalid HTML can break the compile.
Use `preview-mjml` to validate.

## Discovery workflow before composing

Run these as `senda_call_endpoint` once you know the workspace.

1. **Pick the template type** — `GET <ws>/template-types`. The handler
   always returns local + `_system` types together (no `include_inherited`
   query param needed; that flag exists only for injectors). Note the
   `slug`, the `variable_schema` (your `event.*` contract), and whether it
   has a bound `adapter_id` and `sender_identity_id`.
2. **List visible injectors** —
   `GET <ws>/injectors?include_inherited=true`. Note each `name`, the
   `fields[]` array (with `field_name`, `field_type`, `allow_overwrite`,
   `default_value`), and `inherited_from_system`.
3. **Identify the existing template / version** — `GET <ws>/template-types/:slug/templates`.
   If 0 returned, create one (`POST <ws>/templates`); if 1 returned, you
   either edit a draft or clone a version
   (`POST .../versions/:vid/clone`).
4. **Create the draft version** — `POST <ws>/templates/:tid/versions` with
   `subject`, `from_name`, `preview_text`, `body_mjml`, `default_locale`.
5. **Preview while composing** — `POST <ws>/templates/:tid/preview-mjml`
   with `{mjml: "..."}`. Returns `{html}`. Iterate.
6. **Publish** — `POST <ws>/templates/:tid/versions/:vid/publish`
   (admin role).

## Document skeleton

The minimum that compiles:

```mjml
<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hello {{ event.first_name }}</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>
```

Rules the visual builder follows (and you should too):

- Wrap regular blocks (text, button, image, divider, spacer, video, list)
  inside `<mj-section><mj-column>...</mj-column></mj-section>`.
- Banner blocks sit **directly under `<mj-body>`**. The default fill mode
  renders as `<mj-hero>`; fit modes that need `background-size` render as a
  marked `<mj-section css-class="senda-builder-banner">`. Mixing banners
  between regular blocks closes the current section and opens a new one for
  what follows.
- Rich section blocks also sit **directly under `<mj-body>`** because they own
  their internal columns and mobile stacking. The visual builder currently
  emits four marked sections: `senda-builder-media-content`,
  `senda-builder-cta-group`, `senda-builder-feature-list`, and
  `senda-builder-footer-cta`. Footer CTA is just another section block: it can
  appear more than once and can be placed between any other builder blocks.
- Stack multiple sections vertically. Add `<mj-spacer>` or padding for
  vertical rhythm.
- Unsupported MJML that the visual editor cannot represent is preserved as an
  editable **MJML code** block. Unknown elements inside a single-column
  section stay in that column order; unknown direct `<mj-body>` children stay
  as body-level blocks between the known sections/heroes. If a section shape is
  not a single direct `<mj-column>`, the whole section is preserved as code.

## Anti-pattern: HTML wrappers

**MJML compiles INTO HTML.** Wrapping MJML in HTML is double-wrapping and
breaks the gomjml/XML parser at runtime. Do **not** add any of these to a
`body_mjml`, anywhere in the document, including inside `<mj-raw>`:
`<!DOCTYPE>`, `<html>`, `<head>`, `<body>` (literal — `<mj-body>` is fine),
`<meta>`, `<title>`, `<link>`, `<base>`.

```mjml
<!-- WRONG — runtime XML parsing error -->
<mjml>
  <mj-body>
    <mj-raw>
      <!DOCTYPE html>
      <html><head><meta charset="utf-8"></head>
        <body>Hi {{ event.first_name }}</body>
      </html>
    </mj-raw>
  </mj-body>
</mjml>
```

```mjml
<!-- RIGHT — let MJML produce the HTML wrapper for you -->
<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hi {{ event.first_name }}</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>
```

`<mj-raw>` is for small inline HTML snippets the builder cannot express
(e.g. a one-off `<div class="x">…</div>`), never a full document. If you
need a document title or stylesheet, use `<mj-head>` with `<mj-title>` /
`<mj-style>`.

## Block catalog — copy-paste MJML

Each block below is exactly what the builder emits. Drop it inside an
`<mj-column>` (or, for the banner, directly under `<mj-body>`).

### Text

```mjml
<mj-text>Welcome, {{ event.first_name }}.</mj-text>
<mj-text align="center">Centered headline</mj-text>
```

`align` is **omitted for `left`** (MJML default) and emitted explicitly for
`center`, `right`, or `justify`. Inner content is HTML
(use `<strong>`, `<em>`, `<a href="...">`, `<br />`). Variables work
anywhere inside.

### Button

```mjml
<mj-button href="{{ event.confirmation_url }}">Confirm your email</mj-button>
```

Common attributes: `background-color`, `color`, `align`, `border-radius`,
`font-size`, `inner-padding`. Default styling is gomjml's.

### Image

```mjml
<mj-image src="{{ injector.brand.logo_url }}" width="180px" alt="Brand logo" />
```

`src` must end up an absolute https URL after substitution. `width` is
optional but recommended for predictable rendering. `alt` improves
accessibility and shows when images are blocked.

### Divider

```mjml
<mj-divider />
```

Optional: `border-color`, `border-style`, `border-width`, `padding`.

### Spacer

```mjml
<mj-spacer height="20px" />
```

Vertical whitespace. Use round numbers (10, 20, 32). The builder defaults
to `20px`.

### Video (renders as a clickable image)

There is no `mj-video`. The builder produces an image-with-link plus a play
overlay served by `/public/video-thumbnail`:

```mjml
<mj-image
  src="https://your.host/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FVIDEO_ID%2Fmaxresdefault.jpg"
  href="https://www.youtube.com/watch?v=VIDEO_ID"
  width="600px"
  alt="Watch the video"
  align="center"
  css-class="senda-video" />
```

Senda's compiler rewrites raw thumbnail URLs to go through
`/public/video-thumbnail` so the play-button overlay is composited.
YouTube and Vimeo are auto-detected if you supply only the watch URL via
the visual builder; in raw MJML, build the URL yourself or use the helper
host `img.youtube.com/vi/<id>/maxresdefault.jpg`.

### List

The builder renders lists as an `<mj-text>` containing an `<ul>` / `<ol>`:

```mjml
<mj-text align="left">
  <ul style="list-style-type: disc; padding-left: 20px; margin: 0;">
    <li>{{ event.item_one }}</li>
    <li>{{ event.item_two }}</li>
    <li>Static line</li>
  </ul>
</mj-text>
```

For numbered lists use `<ol>` with `list-style-type: decimal` (or
`upper-alpha`, `lower-alpha`, `upper-roman`).

### Banner / Hero

Default banners render as `<mj-hero>` directly under `<mj-body>`.

```mjml
<mj-hero
  mode="fixed-height"
  height="320px"
  background-url="{{ injector.brand.hero_image }}"
  background-color="{{ injector.brand.hero_color }}"
  background-position="center center"
  vertical-align="middle"
  padding="40px">

  <mj-text align="center" color="#ffffff" font-size="20px">
    {{ event.headline }}
  </mj-text>

  <mj-button
    href="{{ event.cta_url }}"
    background-color="#22c55e"
    align="center">
    {{ event.cta_label }}
  </mj-button>

</mj-hero>
```

`mode` is `fixed-height` (use `height`) or `fluid-height` (height defined
by content). `background-url` is optional; without it, `background-color`
fills. The visual builder supports injector tokens in the banner background
image URL, background color (when switched to injector mode), and overlay text.
It also exposes image fit (`cover`, `contain`, `auto`), horizontal image
alignment (`left`, `center`, `right`), and horizontal overlay text alignment.
When overlay text and button are both empty, the builder emits an empty
`mj-text` line so the banner keeps the same height while editing or previewing.
`cover` uses `<mj-hero>` and may crop; `contain` / `auto` use a marked
`<mj-section>` with `background-size` and `background-repeat="no-repeat"` so
the image can be drawn without the forced hero crop.

### Rich section blocks

The visual builder includes four higher-level components for common newsletter
and marketing layouts:

- **Media + Content** (`senda-builder-media-content`) renders a two-column
  image/text section. On mobile, MJML stacks the image and copy into two rows.
  Use it for benefits, testimonials, screenshots, and narrative sections.
- **CTA Group** (`senda-builder-cta-group`) renders centered copy with one or
  two buttons, or a split text-plus-button box. Split mode stacks copy over
  buttons on mobile.
- **Feature List** (`senda-builder-feature-list`) renders a title, icon/text
  rows, optional footer copy, and an optional accent band for milestones or
  benefit lists.
- **Footer CTA** (`senda-builder-footer-cta`) renders a branded closing band
  with headline copy, button, optional background image, and optional logo. It
  can be used more than once and does not need to be the last section.

Media + Content image/button URLs, CTA Group button URLs, and Footer CTA
button/background/logo URLs may be injector tokens. These blocks are direct
body-level sections, not children of the regular single-column wrapper. At
send time, injector/system placeholders rendered inside MJML attributes are
XML-escaped before gomjml compiles the body, so URLs with query strings remain
valid MJML.

```mjml
<mj-section css-class="senda-builder-media-content" background-color="#ffffff" padding="24px 20px">
  <mj-column width="45%">
    <mj-image src="{{ injector.brand.hero_image }}" alt="Preview" padding="0 12px" />
  </mj-column>
  <mj-column width="55%">
    <mj-text font-size="22px" font-weight="600">Benefits</mj-text>
    <mj-text font-size="15px" line-height="1.5">Short supporting copy.</mj-text>
    <mj-button href="#" background-color="#5429ff" border-radius="24px">Learn more</mj-button>
  </mj-column>
</mj-section>
```

## Composing a real template — full example

```mjml
<mjml>
  <mj-body>

    <mj-hero
      mode="fluid-height"
      background-url="{{ injector.brand.hero_image }}"
      background-color="#0f172a"
      vertical-align="middle"
      padding="48px">
      <mj-text align="center" color="#ffffff" font-size="22px">
        Welcome, {{ event.first_name }}
      </mj-text>
    </mj-hero>

    <mj-section background-color="#ffffff">
      <mj-column>
        <mj-image
          src="{{ injector.brand.logo_url }}"
          width="160px"
          alt="{{ injector.brand.name }}" />
        <mj-spacer height="16px" />
        <mj-text>
          Hi <strong>{{ event.first_name }}</strong>, your account is ready.
        </mj-text>
        <mj-text>
          <ul style="list-style-type: disc; padding-left: 20px; margin: 0;">
            <li>Verify your email below.</li>
            <li>Complete your profile.</li>
            <li>Invite your team.</li>
          </ul>
        </mj-text>
        <mj-spacer height="20px" />
        <mj-button
          href="{{ event.confirmation_url }}"
          background-color="#22c55e">
          Confirm your email
        </mj-button>
        <mj-divider border-color="#e5e7eb" />
        <mj-text align="center" font-size="12px" color="#6b7280">
          If you didn't sign up, ignore this email.
          {{ injector.brand.name }} · {{ injector.brand.address }}
        </mj-text>
      </mj-column>
    </mj-section>

  </mj-body>
</mjml>
```

Use this as a starting point and trim/extend per case.

## Composer's checklist

Before publishing a version:

1. Every `{{ event.X }}` is declared in the `template_type.variable_schema`
   AND every sender (backend service, tests) actually populates it. Senda
   does not enforce the schema at send time — missing variables render as
   empty string with no error.
2. Every `{{ injector.<name>.<field> }}` matches a real injector visible in
   this workspace (`?include_inherited=true`). Same: silent empty if missing.
3. Run `bash skills/senda/scripts/mjml-check.sh <file>` (or pipe the body
   to `mjml-check.sh -`). It must exit 0 before you submit any version
   POST/PUT or locale upsert. The script catches the HTML-wrapper class of
   error (`<!DOCTYPE>`, `<html>`, `<body>`, etc., including inside
   `<mj-raw>`) that gomjml only surfaces at compile time.

   > **MCP-only agents (no shell):** if you operate Senda only through
   > `senda_call_endpoint` and cannot run the script, manually verify the
   > body contains none of `<!DOCTYPE>`, `<html>`, `<head>`, `<body>`
   > (literal — `<mj-body>` is fine), `<meta>`, `<title>`, `<link>`,
   > `<base>` — anywhere, including inside `<mj-raw>`. Then run
   > `POST .../preview-mjml` to catch compile errors. Saved templates that
   > slip through these checks may break the visual editor's XML parser
   > at runtime.

4. Run `POST .../preview-mjml` with the current `body_mjml`. Confirm:
   - HTML output looks right.
   - Static-injector previews (locked fields with `allow_overwrite = false`)
     are filled in. Override-able fields stay as `{{ ... }}` in preview —
     that is expected.
5. Run `POST .../test-send` with realistic `variables` and `injectors`
   payloads. Confirm rendering in the inbox.
6. `POST .../publish` (admin role).

## Cuándo consultar OpenAPI / MCP

- Body shape for `POST /versions` — extra optional fields (`reply_to`,
  `editor_data`).
- Bulk-send / test-send request shapes.
- The `variable_schema` JSON Schema format on `template_types`.

## Gotchas

- **Variables before MJML**: a malformed substitution can break compilation.
  If a token resolves to text containing unescaped `<` / `>`, MJML may
  reject the document. For HTML-bearing values, prefer an injector field
  with `field_type = html` and place it inside `<mj-text>`.
- **Image and video thumbnails**: hosts may be allowlisted by the deployment
  for the `/public/video-thumbnail` proxy. If a thumbnail URL is rejected,
  fall back to a normal `<mj-image>` linking to the video.
- **Banner placement**: banner blocks must NOT be wrapped in another
  `<mj-section>`. Use direct `<mj-hero>` for cover banners, or the builder's
  direct `<mj-section css-class="senda-builder-banner">` shape for fit modes.
- **Placeholder validation**: there is none at save/publish time. Use
  `preview-mjml` and `test-send` aggressively.
- **gomjml is not 100% MJML.io**: the upstream MJML reference at mjml.io
  documents a superset. Stick to the components used by the builder unless
  you've confirmed gomjml supports the tag (run a `preview-mjml` first).
- **Locales inherit fields by `null`**: a locale row leaves
  `subject`/`body_mjml`/etc. as `null` to fall back to the version
  defaults. Set them only when they actually differ.
- **`editor_data`** is the visual builder's serialized state; if you build
  raw MJML, omit it. The renderer ignores it at send time.
