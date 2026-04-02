# PRD: Senda — Email Orchestration Platform

**Version:** 5.0 (Draft for review)

**Date:** 2026-02-16

**Author:** Rey + Claude (collaborative iteration)

**Status:** Under review — nothing in this document is final

---

## 1. Problem Statement

Companies that manage multiple brands, regions, or clients need to send transactional and notification emails from different contexts, each with its own visual identity, domain data, and sending provider. Today they solve this in three ways, all of them flawed:

Connecting each application directly to a provider (SES, Gmail), scattering sending logic, duplicating templates, and making centralized visibility impossible. Using a SaaS such as SendGrid or Resend, giving up control over the infrastructure and accepting customization limits. Using open source tools such as Listmonk or Postal, which do not provide hierarchical multi-tenancy with configuration inheritance or traceability by external business ID.

**Who experiences this problem?** Companies with multiple products, brands, regions, or clients that need to send transactional emails centrally.

**Cost of not solving it?** Duplicate templates, emails that fail without visibility, inability to audit email communication, dependency on SaaS with growing costs, and technical debt from direct integrations.

---

## 2. Goals

**User Goals:**

1. A centralized place where any application sends emails, with full lifecycle visibility.
2. Hierarchical management at 3 levels (Global → Tenant → Workspace) with automatic inheritance and overrides where needed.
3. Traceability for all emails in a business case via `external_id`, across workspaces and tenants.
4. Reusable templates with a visual editor (drag-and-drop + inline) for non-technical users.
5. Typed data injectors that automatically provide context according to the hierarchy.
6. Deterministic addressing: `tenantCode:workspaceCode:templateType` always resolves to the published template, without handling internal IDs.
7. i18n support in templates with AI assistance for translations and content creation.

**Business Goals:**

1. Reduce N direct integrations to a single centralized platform.
2. Audit 100% of email communication in one place.
3. Open source project with no SaaS dependency.

---

## 3. Non-Goals

1. **Senda is NOT an email sending provider.** It does not compete with SES or Gmail. It is an orchestration layer that uses those providers as transport through adapters.

2. **Senda does NOT control which event triggers an email.** The logic for "when to send" lives outside Senda. An external service decides what to send and calls the API with the data.

3. **Senda is NOT an email marketing tool.** It does not manage subscriber lists, segmentation, campaign A/B testing, or marketing scheduling.

4. **Senda is NOT an Identity Provider.** Dashboard authentication is delegated to an external OIDC provider. Senda manages membership and roles (authorization), but not authentication (there is no signup, password recovery, or native MFA).

5. **Senda does NOT expose an SMTP relay.** Communication is exclusively through the REST API.

---

## 4. Core Concepts

### 4.1. Hierarchy: Global → Tenant → Workspace

Senda operates with **three hierarchical levels**. The names are generic — each company decides what each level means.

**Global** is the root layer. It is managed by superadmins. It defines defaults inherited across the platform: base templates, corporate injectors, default sending adapters, verified domains, and platform settings.

**Tenant** is the first grouping level (country, region, division, business line). Each installation has at least 1 tenant. Each tenant is identified by a **unique code** (slug): `latam`, `europe`, `northern-division`.

**Workspace** is the granular operational unit (client, brand, product, team). Each tenant has at least 1 workspace. Each workspace is identified by a **code unique within its tenant**: `acme-corp`, `brand-x`, `support`.

#### System Workspace (`_system`)

Each tenant has an auto-created special workspace called `_system`. This workspace:

- Is where templates, injectors, adapters, and domains are configured so that they **inherit to all workspaces in the tenant**.
- Cannot be deleted or renamed.
- Cannot send emails directly.
- Is the equivalent of "tenant-level configuration" but managed uniformly as a workspace.

#### Resolution Chain

When the system needs to resolve any resource (template, injector, adapter, domain, setting), it searches this chain:

```
Workspace → Tenant _system → Global
```

The first match wins. This allows defaults to be defined above and overrides below.

**Error if unresolved:** If a required resource is not found at any level in the chain (for example, no adapter is configured), Senda returns `422 Unprocessable Entity` with a descriptive message: `"No email adapter configured for workspace 'acme' in tenant 'latam'. Configure an adapter at workspace, tenant (_system), or global level."` The workspace remains unusable for sending until the missing resource is configured.

**Concrete example:**

```
Global
├── Injector "brand": {logo: corp-logo.png, name: "MyCompany", email: support@mycompany.com}
├── Template "welcome" (global)
├── Adapter: SES us-east-1
├── Verified domain: mycompany.com
│
├── Tenant "latam" (code: latam)
│   ├── _system workspace
│   │   ├── Injector "brand": {logo: latam-logo.png}  ← partial override, inherits name and email
│   │   ├── Template "welcome" (LATAM)                 ← override of the global one
│   │   ├── Adapter: SES sa-east-1                      ← override of the global one
│   │   └── Verified domain: latam.mycompany.com        ← additional to the global one
│   │
│   ├── Workspace "acme" (code: acme)
│   │   ├── Injector "brand": {name: "Acme Corp"}     ← partial override
│   │   ├── Template "welcome" (Acme)                  ← override of the tenant one
│   │   └── (inherits adapter from _system: SES sa-east-1)
│   │
│   └── Workspace "beta" (code: beta)
│       └── (inherits everything from _system and global)
```

In this example, a "welcome" email sent from `latam:acme` uses:
- The Acme "welcome" template (workspace).
- The merged `brand` injector: `{logo: latam-logo.png, name: "Acme Corp", email: support@mycompany.com}`.
- The SES sa-east-1 adapter (from `_system`).
- The domain: latam.mycompany.com (from `_system`) or mycompany.com (from global), depending on the from-address configuration.

A "welcome" email sent from `latam:beta` uses:
- The LATAM "welcome" template (`_system`).
- The `brand` injector: `{logo: latam-logo.png, name: "MyCompany", email: support@mycompany.com}`.
- The SES sa-east-1 adapter (from `_system`).

#### Cross-Tenant Isolation

Non-superadmin roles are **strictly isolated** by tenant:
- A `tenant-admin` for "latam" has NO visibility over the "europe" tenant or its workspaces.
- A `workspace-admin` for "latam:acme" cannot see or access "latam:beta".
- Only `superadmin` users have cross-tenant visibility.
- API Keys are scoped to a workspace and cannot operate outside it.

### 4.2. Codes (Slugs)

Tenants and workspaces are identified by **codes** in slug format:

- Lowercase alphanumeric plus hyphens: `[a-z][a-z0-9-]*`
- Minimum 2, maximum 50 characters.
- Must start with a letter.
- No consecutive hyphens (`--`).
- No trailing hyphen.
- Reserved: `_system`, `global`, `admin`, `api`, `system` (they cannot be used as workspace or tenant codes).

**Uniqueness:**
- Tenant code: **globally unique** (you cannot have two tenants with the same code).
- Workspace code: **unique within its tenant** (two tenants can both have a workspace called `main`, but a single tenant cannot have two `main` workspaces).

**Immutability:** Codes are immutable after creation. Changing a code would break external integrations that use the addressing `tenantCode:workspaceCode:templateType`.

### 4.3. Injectors (Data Injectors)

An **injector** is a named set of typed key-value pairs that are injected automatically into templates at compile time. They are context data — information that does not come from the event, but from the workspace/tenant/global identity.

**Supported field types:**

| Type | Description | Example value |
|---|---|---|
| `text` | Text string | "Acme Corporation" |
| `number` | Numeric value | 42, 3.14 |
| `bool` | True/false | true, false |
| `img` | Image URL | "https://cdn.acme.com/logo.png" |
| `url` | URL | "https://acme.com/support" |
| `html` | Sanitized HTML block | "<p>Legal footer</p>" |

#### Top-Down Fixed Schema Model

The level that **creates** an injector defines its **schema** (field names and types). This is immutable:

- If the superadmin creates the `brand` injector at global level with fields `{logo: img, name: text, email: text}`, that is the contract.
- Lower levels (`_system`, workspace) can **override values** for those fields, but they **CANNOT add new fields or remove existing fields**.
- If a `tenant-admin` needs additional fields for their context, they must create a **new injector** (for example, `brand-extra`) in their scope, not extend the global injector.

This guarantees:
- Predictability: a template that uses `{{ injector.brand.logo }}` knows the field exists at all levels.
- Validation: at compile time, if a referenced field does not exist in the injector schema, it is a template error, not a data problem.
- Clarity: there are no "ghost fields" that appear only in some workspaces.

#### Field-by-field inheritance

Field resolution follows the workspace → `_system` → global chain, field by field:
- If the workspace defines `brand.logo`, that value wins.
- Otherwise, it searches in the tenant's `_system`.
- Otherwise, it searches globally.
- Unoverridden fields are inherited from the higher level.
- A field **cannot be set to null** to "remove" an inherited value. If you need an empty field, use an empty value for that type (for example, `""` for text, `0` for number).

**Difference from event variables:**
- **Injectors:** Static/semi-static context data (brand, logos, URLs, contact details). Resolved automatically by Senda according to the hierarchy.
- **Event variables:** Dynamic instance data (user name, order number, amount). Sent on every API request by the external service.

### 4.4. Ports and Adapters (Email Sending)

Senda uses **hexagonal architecture** for sending:

**Port (interface):** Contract that any email provider must satisfy.

**Adapter (implementation):** Concrete implementation for a provider. Initially: SESAdapter and GmailAdapter.

Adapters are configured hierarchically (global → tenant `_system` → workspace) and follow the same resolution chain. Adding new adapters (SendGrid, Mailgun) requires implementing a single adapter without touching the core.

**Adapter resolution:** When sending an email, Senda looks for the adapter in: workspace → `_system` → global. If none exists at any level → `422 Unprocessable Entity`. A workspace can have its own adapter (for example, a dedicated SES account) or inherit from the tenant/global.

### 4.5. Template System

#### Template Types

A **template type** defines a contract: name (slug), description, and JSON schema of the event variables it expects. Types are defined hierarchically:
- Global types: available across the platform.
- Types in the tenant `_system`: available to all workspaces in the tenant.
- Types in a workspace: available only in that workspace.

Visibility follows the same chain: a workspace can use its own types + types from its `_system` + global types. A workspace **can create new types** that do not exist in `_system` or global — those types are visible and usable only within that workspace.

#### Templates and Versions

A **template** is the visual implementation of a type. Each template has **versions** with states:

| State | Description |
|---|---|
| `draft` | Being edited. Multiple drafts can exist. Not used for sending. |
| `published` | Active for sending. **Only one published version may exist per template.** |
| `archived` | Previous version preserved for history. Not used for sending. |

When a draft is published, the previously published version is automatically archived.

#### Template Content

Each template contains:

- **Body:** MJML code (edited visually or manually) that defines the email content.
- **Subject:** Subject line. Supports injector and event variables (for example, `"Welcome {{ event.user_name }} to {{ injector.brand.name }}"`).
- **Preview text:** Preview text shown in the inbox after the subject. Supports variables.
- **From address:** Sender address. It is composed of:
  - **From name:** Visible name (for example, "Acme Support"). Configurable per template, with hierarchical fallback. Supports injector variables.
  - **From email:** Sender email (for example, `support@acme.com`). It must correspond to a verified domain in the resolution chain. Configurable per template, with hierarchical fallback.
- **Reply-to:** (Optional) Reply email. Configurable by workspace/tenant/global with inheritance.
- **Locale tags:** (Optional) Language tags in text blocks for i18n support.

#### Type uniqueness by scope

Within any scope (workspace, `_system`, or global), a template type **can only have one assigned template**. This rule applies uniformly at the three levels:

- Workspace "acme" has type "welcome" → only ONE "welcome" template can exist in that workspace.
- "latam" `_system` has type "welcome" → only ONE "welcome" template can exist in that `_system`.
- Global has type "welcome" → only ONE "welcome" template can exist at global level.

This rule is fundamental to deterministic addressing.

#### Deterministic Addressing

Emails are sent using the reference:

```
tenantCode:workspaceCode:templateType
```

Example: `latam:acme:welcome`

This reference resolves as follows:
1. Find the tenant with code `latam`.
2. Find the workspace with code `acme` within that tenant.
3. Find the template assigned to type `welcome` in that workspace.
4. If none exists in the workspace, look in the `latam` tenant `_system`.
5. If none exists in `_system`, look at global level.
6. Take the `published` version of the found template.
7. If there is no template at any level → `404 Not Found`: `"No published template found for type 'welcome' in resolution chain of workspace 'acme', tenant 'latam'"`.

The external service never needs to know internal IDs, versions, or states. It only needs to know: tenant, workspace, and template type. Senda resolves the rest.

#### Disable Sending by Template

A template can be **disabled** (kill switch) without archiving or deleting it. A disabled template:
- Keeps its `published` version but is not used for sending.
- If an email send is attempted with a disabled template, Senda responds with `409 Conflict`: `"Template 'welcome' is disabled in workspace 'acme'"`.
- **Important:** If the disabled template is at a higher level (`_system` or global) and a workspace has no override, the send fails. This is intentional — the kill switch is an administrative decision that affects lower levels.
- It is an emergency mechanism to stop sends without losing configuration.
- It can be re-enabled at any time by an admin in the corresponding scope.

#### Variables in Templates

MJML templates can use two types of variables:

- **Injector variables** (resolved automatically by the hierarchy):
  ```
  {{ injector.brand.logo_url }}
  {{ injector.brand.company_name }}
  {{ injector.legal.footer_html }}
  ```

- **Event variables** (sent in the API request):
  ```
  {{ event.user_name }}
  {{ event.order_number }}
  {{ event.activation_url }}
  ```

Both types of variables can be used in the **body**, **subject**, and **preview text**.

### 4.6. Visual Editor

The template editor has two complementary modes:

**Drag-and-drop:** Drag components (text, image, button, columns, divider, spacer, social links) to build the structure.

**Inline editing:** Click directly on the preview content to edit text, insert variables, and change styles. WYSIWYG editing on the rendered template.

Both modes work on the same internal representation (editor JSON Schema ↔ MJML source). The user can switch between visual mode and code mode (raw MJML).

#### i18n in Templates

The template's text blocks (non-injectable, meaning the static content of the template) can have **language configuration**:

- Each text block can be marked with a **locale tag** (for example, `es`, `en`, `pt-BR`).
- A template can contain language variants for each text block.
- The language is selected at send time (the external service sends `locale` in the request) or by workspace configuration.
- If there is no variant for the requested locale, the template's default locale is used.

**AI integration:**

The editor integrates AI assistance for:
- **Template creation:** Suggest structure, copy, and design based on the email type and context.
- **Translations:** Automatically translate text blocks into the configured languages.
- **Copy optimization:** Improve text for clarity, tone, and engagement.
- **Consistency:** Check that tone and style are coherent across language variants.

AI integration is assistance — the editor always retains control over the final content.

### 4.7. Sending Flow

1. An external service calls the API: `POST /api/v1/send` with:
   - `ref`: `"latam:acme:welcome"` (deterministic addressing)
   - `to`: array of recipients (max 50 per request)
   - `variables`: event variables (`user_name`, `order_number`, etc.)
   - `external_id`: (optional) business case ID
   - `cc`: (optional) array of carbon-copy emails
   - `bcc`: (optional) array of blind-carbon-copy emails
   - `locale`: (optional) language code for i18n (for example, `"es"`, `"pt-BR"`)

2. Senda parses the reference → resolves tenant, workspace, and template type.
3. Senda verifies that the template is not disabled.
4. Senda resolves the template (workspace → `_system` → global) and takes the `published` version.
5. Senda validates event variables against the template type schema.
6. Senda resolves injectors (field-by-field merge: workspace → `_system` → global).
7. Senda resolves `from_email` using the default identity of the effective adapter.
8. Senda validates that the identity is enabled/verified according to the provider.
9. Senda resolves subject and preview text (from the template, applying variables).
10. Senda selects a language variant if `locale` was provided.
11. Senda compiles MJML with injectors + event variables → responsive HTML.
12. Senda enqueues the email as a transactional job (River/PostgreSQL). One job per recipient in `to`.
13. The worker sends through the configured adapter, respecting rate limits.
14. Senda records the result and updates lifecycle tracking. Each recipient gets its own `tracking_id`.
15. If webhooks are configured, it notifies the workspace/tenant.

#### API Request and Response

**Request:**
```json
POST /api/v1/send
Authorization: Bearer senda_live_abc123...

{
  "ref": "latam:acme:welcome",
  "to": ["user@example.com", "other@example.com"],
  "cc": ["manager@example.com"],
  "bcc": ["audit@internal.com"],
  "variables": {
    "user_name": "Maria Garcia",
    "order_number": "ORD-12345",
    "activation_url": "https://app.acme.com/activate/xyz"
  },
  "external_id": "order_12345",
  "locale": "es"
}
```

**Response (202 Accepted):**
```json
{
  "status": "accepted",
  "tracking_ids": [
    {"to": "user@example.com", "tracking_id": "trk_a1b2c3d4"},
    {"to": "other@example.com", "tracking_id": "trk_e5f6g7h8"}
  ],
  "external_id": "order_12345",
  "template_resolved": "latam:acme:welcome",
  "template_version": 3
}
```

**Error Responses:**

| Code | When | Example |
|---|---|---|
| `400` | Malformed request | Missing fields, invalid JSON |
| `401` | Invalid or revoked API Key | |
| `403` | API Key has no access to the workspace | |
| `404` | Template not found in chain | `"No published template for 'welcome' in chain"` |
| `409` | Template disabled | `"Template 'welcome' is disabled in workspace 'acme'"` |
| `422` | Configuration or validation error | No adapter, invalid variables, unverified domain |
| `429` | Rate limit exceeded | Retry-After header included |

### 4.8. External ID and Traceability

Each email can carry an **external_id**: the business system identifier (for example, `order_12345`, `ticket_789`). The field is **optional** — if it is not sent, the email is tracked only by its internal `tracking_id`.

This enables:
- Querying all emails for a business case. **Scope by role:** workspace-admin/editor/viewer only see external_ids from their workspace. Tenant-admin sees all workspaces in their tenant. Superadmin sees cross-workspace and cross-tenant data.
- Search by recipient/sender email address (same scope by role).
- Bulk export with filters.
- Cursor-based pagination in all query APIs.
- API Keys: they only return data from the workspace they belong to.

### 4.9. Email Lifecycle Tracking

Lifecycle states:

| State | Description |
|---|---|
| `queued` | Request received, job enqueued |
| `processing` | Worker compiling and preparing the send |
| `sent` | Delivered to the provider (SES/Gmail accepted) |
| `delivered` | Provider confirmed delivery to the destination |
| `opened` | Recipient opened the email (only if tracking is active) |
| `bounced` | Bounce (soft or hard) |
| `complained` | Marked as spam |
| `failed` | Send error |
| `suppressed` | Address is on the suppression list, email not sent |

#### Open Tracking (Open Pixel)

Open tracking is **disabled by default** and is enabled **opt-in by workspace**:

- Each `workspace-admin` decides whether to enable open tracking for their workspace.
- If enabled, Senda automatically inserts a tracking pixel into the email.
- A configurable disclaimer is automatically added to the email footer when tracking is enabled.
- The `opened` state is only recorded if tracking is active in the email's workspace.
- GDPR considerations: the disclaimer informs the recipient. Each company is responsible for complying with local regulations.

### 4.10. Members and Roles (Authorization)

Senda separates **authentication** (who you are) from **authorization** (what you can do):

- **Authentication:** Delegated to an external OIDC provider (Google Workspace, Keycloak, etc.). Senda does not manage passwords or identities.
- **Authorization:** Managed internally by Senda. An OIDC-authenticated user only gets access to Senda if they are registered as a **member**.

#### Members

A **member** is a record in Senda identified by email. To access the dashboard:
1. The user authenticates against the OIDC provider.
2. Senda verifies that the email in the OIDC token exists as a registered member.
3. If it does not exist → access denied (even if OIDC succeeded): `"Access denied. Your email is not registered as a member. Contact your administrator."`.
4. If it exists → Senda loads the roles assigned to the member.

Members are added by invitation (an admin registers the email). There is no self-signup.

#### Roles

A member can have **different roles in different scopes**. Roles are fixed (not customizable):

| Role | Scope | Capabilities |
|---|---|---|
| `superadmin` | Global | Full access. Manages the whole platform: tenants, global settings, domains, members at any level. |
| `tenant-admin` | Specific tenant | Manages the tenant: `_system` workspace, workspaces, injectors, templates, adapters, domains, and members within their tenant. |
| `workspace-admin` | Specific workspace | Manages the workspace: injectors, templates, domains, webhooks, and workspace members. |
| `workspace-editor` | Specific workspace | Edits templates and injectors in the workspace. Does not manage members, domains, or adapters. |
| `workspace-viewer` | Specific workspace | Read-only. Sees templates, metrics, and email status for the workspace. Cannot modify anything. |

**Multiple superadmins:** There can be multiple superadmins. The original superadmin (created during onboarding) can create other superadmins. This is useful for companies where more than one person needs full platform access.

**Multiple roles:** A member can be `tenant-admin` in the "latam" tenant and `workspace-viewer` in a workspace in the "europe" tenant. Roles are not inherited — being a `tenant-admin` does not automatically grant `workspace-admin` access in the tenant's workspaces, but the `tenant-admin` has access to all workspaces in their tenant as part of that role.

**Permissions table:**

| Action | superadmin | tenant-admin | ws-admin | ws-editor | ws-viewer |
|---|---|---|---|---|---|
| Create/manage tenants | ✓ | — | — | — | — |
| Global settings (adapters, injectors, templates, domains) | ✓ | — | — | — | — |
| Add members at any level | ✓ | — | — | — | — |
| Create other superadmins | ✓ | — | — | — | — |
| Configure tenant `_system` | ✓ | ✓ | — | — | — |
| Create/manage tenant workspaces | ✓ | ✓ | — | — | — |
| Manage tenant domains | ✓ | ✓ | — | — | — |
| Add members to the tenant and its workspaces | ✓ | ✓ | — | — | — |
| View tenant metrics (all workspaces) | ✓ | ✓ | — | — | — |
| Workspace settings (adapters, domains, webhooks) | ✓ | ✓ | ✓ | — | — |
| Add members to the workspace | ✓ | ✓ | ✓ | — | — |
| Manage workspace API Keys | ✓ | ✓ | ✓ | — | — |
| Edit workspace templates and injectors | ✓ | ✓ | ✓ | ✓ | — |
| Publish/archive/disable templates | ✓ | ✓ | ✓ | — | — |
| View templates, metrics, and email status | ✓ | ✓ | ✓ | ✓ | ✓ |
| Send test emails from dashboard | ✓ | ✓ | ✓ | ✓ | — |

Note: `workspace-editor` can create and edit template drafts, but **cannot publish them** — that requires `workspace-admin` or above. This enables a review flow where the editor prepares and the admin approves.

#### Initial Onboarding

When Senda is first installed and the database is empty:

1. The first user who completes OIDC login is automatically registered as **superadmin**.
2. The system prompts that user to create the first tenant (code + name).
3. The `_system` workspace is created automatically.
4. The first regular workspace is created (the system prompts for code + name).
5. From that point on, the superadmin manages everything manually.

This flow happens only once. If at least one member already exists, OIDC login is validated against the members table normally.

### 4.11. API Keys

Access to the send and query API (data plane) does not use OIDC but long-lived **API Keys**:

- Each API Key belongs to a specific workspace.
- Identifiable prefix format: `senda_live_<random>` (production) and `senda_test_<random>` (sandbox).
- They are stored hashed in the database (never in plaintext).
- A workspace can have multiple active API Keys.
- API Keys can be revoked individually.
- An API Key can only send emails and query data within its workspace (and hierarchical resolution applies from that workspace).

**Allowed API Key operations:**
- Send emails (`POST /api/v1/send`)
- Query email status by `tracking_id`
- Query emails by `external_id`
- Search emails by recipient/sender
- Export records with filters

**Test/sandbox mode:** Test and simulation functionality (sending a test email without actually delivering it) is performed from the **dashboard** authenticated via OIDC, not from the API. This lets editors and admins test templates without needing test API Keys.

### 4.12. Soft Delete and Dependency Management

When an admin deletes a resource (template, injector, adapter) that may be inherited by lower levels:

**Soft delete:** The resource is not physically removed. It is marked as `deleted` with a timestamp.

**Behavior:**
- The resource remains available in the resolution chain for scopes that inherited it.
- Inheriting scopes show a **visual warning** in the dashboard: `"This resource is marked as deprecated at [level]. Configure an override."`.
- Sends continue to work normally — soft delete does not interrupt operations.
- An admin can perform a **purge** (permanent delete) after verifying there are no dependencies, or that dependents already have overrides.

**Before purge**, the system shows:
- A list of scopes inheriting the resource without their own override.
- Estimated impact: how many workspaces/templates would be affected.
- Explicit confirmation required.

**Audit trail:** All soft delete and purge operations are recorded with: who, when, which resource, and the dependency state at the time of the action.

### 4.13. Sending Identities (Provider-Managed)

Senda uses a **provider-managed** model for email authentication:

- SPF/DKIM/DMARC are the provider's responsibility (SES/Gmail).
- Senda does not generate or sign DKIM in the application.
- Senda synchronizes and validates available identities in the provider for each adapter.

**Flow:**
1. The admin configures the SES/Gmail adapter in the corresponding scope.
2. Senda syncs identities from the provider (verified emails/domains, sending status).
3. The admin selects the default identity for the adapter.
4. `POST /send` uses the effective default identity of the resolved adapter.

**Sending rule:** if the adapter has no valid/verified default identity, sending fails with a functional error (`422`).

### 4.14. Suppression Lists

Senda maintains two suppression levels:

**Global suppression (hard bounces):**
- An email that hard-bounces is automatically added to the global suppression list.
- It blocks sends to that address in **all workspaces** across the entire platform.
- Only a `superadmin` can remove an address from global suppression (with a recorded justification).

**Workspace suppression (complaints):**
- If a recipient marks a workspace-specific email as spam, it is suppressed only for that workspace.
- The `workspace-admin` can see their suppression list.
- It does not affect other workspaces (a complaint in "acme" does not suppress in "beta").

**Interaction:**
- On send, Senda checks both lists. If the email is in either one → `suppressed`, email not sent.
- Global suppression is absolute: a workspace-admin CANNOT unsuppress an address that is on the global list.

**Soft bounces:**
- Soft bounce → automatic retry (max 3 attempts with exponential backoff).
- After 3 failed attempts → `failed` (not automatically suppressed).
- If an email generates 3+ soft bounces in 7 days → alert the workspace-admin.

**Alerts:**
- If bounce rate > 5% in 24h for a workspace → alert the workspace-admin and tenant-admin.
- If complaint rate > 0.1% → immediate alert.

### 4.15. Audit Logging

Senda records an **audit log** of all administrative actions:

- **Who:** Email of the member who performed the action.
- **When:** UTC timestamp.
- **What:** Action performed (create, update, delete, publish, disable, etc.).
- **Where:** Scope (global, tenant, workspace).
- **Details:** Specific changes (for example, `"field 'logo' changed from 'old.png' to 'new.png'"`).

The audit log is append-only (it cannot be modified or deleted). It is visible to admins in the corresponding scope and to superadmins.

---

## 5. User Stories

### Global Administrator (Superadmin)

**US-01:** As a superadmin, I want to define base injectors at the global level with their schema (fields and types) so that all tenants and workspaces inherit those values as defaults.

**US-02:** As a superadmin, I want to define global template types with their variable schema so that I can establish validated data standards.

**US-03:** As a superadmin, I want to create global templates for each type (including subject, preview text, and from address) so that workspaces without customization have a functional base design.

**US-04:** As a superadmin, I want to configure global sending adapters (SES, Gmail) as the default transport.

**US-05:** As a superadmin, I want to see aggregated metrics for the entire platform.

**US-06:** As a superadmin, I want to manage provider adapters and verified identities at the global level.

**US-07:** As a superadmin, I want to configure global parameters (rate limits, retries, log retention).

**US-08:** As a superadmin, I want to create tenants with unique codes and manage their base configuration.

**US-09:** As a superadmin, I want to add members (by email) and assign roles to them at any level (global, tenant, workspace), including creating other superadmins.

**US-10:** As a superadmin, I want the first OIDC login to automatically register me as a superadmin when Senda is installed for the first time and guide me through creating the first tenant and workspace.

**US-11:** As a superadmin, I want to see the audit log of administrative actions across the entire platform.

### Tenant Administrator

**US-12:** As a tenant admin, I want to configure templates, injectors, and adapters in the `_system` workspace so they are inherited by all my workspaces.

**US-13:** As a tenant admin, I want to create workspaces with unique codes (within my tenant) to organize my clients/brands/products.

**US-14:** As a tenant admin, I want to create additional template types in `_system` for my business context.

**US-15:** As a tenant admin, I want to see aggregated metrics for all my workspaces.

**US-16:** As a tenant admin, I want to manage provider sending identities at the tenant (`_system`) level so my workspaces can inherit them.

**US-17:** As a tenant admin, I want to add members to my tenant and its workspaces, assigning appropriate roles.

**US-18:** As a tenant admin, I want to soft-delete resources in `_system` and see which workspaces would be affected before purging.

### Workspace Administrator

**US-19:** As a workspace admin, I want to override inherited injector values (field by field, without changing the schema) to customize the identity of my emails.

**US-20:** As a workspace admin, I want to create my own templates for existing types, knowing that I can only have one template per type, with subject, preview text, and from address.

**US-21:** As a workspace admin, I want to manage my template versions: create drafts, publish (replacing the previous published version), and view archived version history.

**US-22:** As a workspace admin, I want to disable a template to stop emergency sends without losing the configuration.

**US-23:** As a workspace admin, I want to configure webhooks to receive email status change notifications.

**US-24:** As a workspace admin, I want to see metrics exclusive to my workspace.

**US-25:** As a workspace admin, I want to add members to my workspace with editor or viewer roles.

**US-26:** As a workspace admin, I want to manage my workspace API Keys: create, view (partially masked), and revoke them.

**US-27:** As a workspace admin, I want to enable/disable open tracking for my workspace.

**US-28:** As a workspace admin, I want to send test emails from the dashboard to try templates before publishing them.

### External Service (API Consumer)

**US-29:** As an external service, I want to send an email through the API using the `tenantCode:workspaceCode:templateType` reference with an array of up to 50 recipients, and receive a `202 Accepted` with `tracking_ids`.

**US-30:** As an external service, I want to query the status of an email by `tracking_id`.

**US-31:** As an external service, I want to query all emails for an `external_id` with pagination.

**US-32:** As an external service, I want to search emails by recipient or sender address with pagination.

**US-33:** As an external service, I want to bulk export email records with filters.

**US-34:** As an external service, I want to receive a clear error if I try to send with a disabled template (`409 Conflict`).

**US-35:** As an external service, I want to specify a `locale` in the request to select the template's language variant.

### Workspace Editor

**US-36:** As a workspace editor, I want to create and edit template drafts using the visual editor or MJML code, including subject and preview text.

**US-37:** As a workspace editor, I want to edit text directly on the preview (inline editing).

**US-38:** As a workspace editor, I want to preview on desktop and mobile.

**US-39:** As a workspace editor, I want to insert injector and event variables from a visual selector.

**US-40:** As a workspace editor, I want to save as draft, knowing that an admin must publish it.

**US-41:** As a workspace editor, I want to configure language variants for the template's text blocks.

**US-42:** As a workspace editor, I want to use AI assistance to translate text blocks and improve copy.

**US-43:** As a workspace editor, I want to send test emails to verify how the template looks.

### Members

**US-44:** As a user whose email is registered as a member, I want to authenticate via OIDC and automatically access the tenants and workspaces where I have an assigned role.

**US-45:** As a user who is not registered as a member, I want to receive a clear "access denied, contact your administrator" message when I try to access after authenticating via OIDC.

---

## 6. Requirements

### Must-Have (P0)

**R-01: 3-level hierarchy with inheritance and `_system`**
Global → Tenant → Workspace. Each tenant has an auto-created `_system` workspace for inheritable configuration. Resolution: workspace → tenant `_system` → global.
- [x] Isolation: resources from workspace A are not visible from workspace B (except via `_system`/global inheritance).
- [x] `_system` is created automatically when a tenant is created. It cannot be deleted or renamed.
- [x] Emails cannot be sent from `_system`.
- [x] If a workspace does not define a resource, it inherits from `_system`; if `_system` does not define it, it inherits from global.
- [x] If a resource does not exist at any level → clear error (422).
- [x] Minimum: 1 tenant + 1 workspace (in addition to `_system`).
- [x] Cross-tenant isolation: non-superadmin roles have no visibility outside their scope.

**R-02: Slugs for tenants and workspaces**
Human-readable identifiers in slug format.
- [x] Format: `[a-z][a-z0-9-]*`, 2-50 chars, no `--`, no trailing hyphen.
- [x] Tenant code: globally unique.
- [x] Workspace code: unique within its tenant.
- [x] Reserved: `_system`, `global`, `admin`, `api`, `system`.
- [x] Immutable after creation (to avoid breaking external integrations).

**R-03: Typed injectors with fixed top-down schema and field-by-field inheritance**
Named typed key-value sets. Schema defined by the creating level; lower levels only override values.
- [x] Validated types: text, number, bool, img, url, html.
- [x] Fixed schema: the level that creates the injector defines fields + types. Lower levels cannot add or remove fields.
- [x] Partial override: workspace overrides only the values it needs.
- [x] A field cannot be set to null (use an empty value of the corresponding type).
- [x] If a level needs extra fields, it creates a new injector with a different name.
- [x] Validation: during compilation, if a template references a field that does not exist in the schema → error.

**R-04: Send API with deterministic addressing**
`POST /api/v1/send` with the `tenantCode:workspaceCode:templateType` reference.
- [x] Parses the reference → resolves tenant, workspace, and template.
- [x] `to`: array of emails, maximum 50 per request. Each one generates an independent email with its own `tracking_id`.
- [x] `cc` and `bcc`: optional arrays of emails.
- [x] `external_id`: optional.
- [x] `locale`: optional, for language variant selection.
- [x] Validates variables against the template type schema.
- [x] Verifies the template is not disabled.
- [x] Verifies the verified domain for the from address.
- [x] Response < 100ms p99 with `tracking_ids`.
- [x] 400 malformed, 401 invalid API Key, 403 no access, 404 template not found, 409 template disabled, 422 config/validation error, 429 rate limit.

**R-05: MJML templates with subject, preview text, from address, and hierarchical resolution**
MJML compiled via gomjml. Injector + event variables in body, subject, and preview text.
- [x] Compilation < 10ms p99.
- [x] XSS prevention in variables.
- [x] Subject and preview text support injector and event variables.
- [x] From address: `from_name` + `from_email` configurable per template with hierarchical fallback.
- [x] From email must correspond to a verified domain in the chain.
- [x] Reply-to configurable by workspace/tenant/global with inheritance.
- [x] Renders in Gmail, Outlook, and Apple Mail.

**R-06: Template types with variable schema**
Contract: name (slug), description, JSON schema of the event.
- [x] Variable validation at send time.
- [x] Hierarchical visibility (global, `_system`, workspace).

**R-07: Template versioning with states**
draft → published → archived. Only ONE published version per template.
- [x] Multiple drafts allowed.
- [x] On publish: draft → published, previous published version → archived.
- [x] Only the published version is used for sending.
- [x] Archived version history is visible.
- [x] Revert: create a new draft from an archived version.

**R-08: Type uniqueness per scope**
A template type can have only ONE assigned template within a scope (workspace, `_system`, or global).
- [x] Applies uniformly at the three levels.
- [x] Trying to create a second template for the same type in a scope → error.
- [x] This guarantees that deterministic addressing always resolves to exactly one template.

**R-09: Disable template (kill switch)**
Ability to disable a template without archiving it.
- [x] A disabled template keeps its published version but is not sent.
- [x] API returns `409 Conflict` with a clear message.
- [x] Re-enable at any time by an admin in the scope.
- [x] If disabled at a higher level and the workspace has no override, sending fails.
- [x] Visible status in the dashboard.

**R-10: Ports and adapters for sending (SES + Gmail)**
Hexagonal architecture. The port defines the contract; adapters implement it.
- [x] SESAdapter: region, access key, secret key (encrypted).
- [x] GmailAdapter: OAuth credentials (encrypted).
- [x] Hierarchical adapter inheritance.
- [x] If there is no adapter at any level → 422 with a descriptive message.
- [x] Extensible without modifying the core.

**R-11: Email lifecycle tracking**
queued → processing → sent → delivered → opened → bounced → complained → failed → suppressed.
- [x] Query by `tracking_id`.
- [x] History with timestamps.
- [x] Provider webhooks update the state automatically.
- [x] `opened` is only available if tracking is active in the workspace.

**R-12: Traceability by `external_id`**
- [x] Optional field in the send request.
- [x] Paginated query with filters.
- [x] Indexed for efficiency.
- [x] Cross-workspace/cross-tenant for superadmins.

**R-13: Email authentication (provider-managed)**
- [x] SPF/DKIM/DMARC delegated to the provider (SES/Gmail).
- [x] Senda validates the adapter's default identity before sending.
- [x] If the identity is not verified/enabled, sending fails.
- [x] Senda does not implement in-app DKIM signing.

**R-14: Identity synchronization with inheritance**
- [x] Manual/API sync of identities by adapter.
- [x] Identity states reflect the provider's state.
- [x] Default identity configurable by scope.
- [x] Adapter/identity inheritance: workspace → `_system` → global.
- [x] Explicit fallback for adapters that do not list identities (for example, SMTP/manual).

**R-15: Bounce and complaint handling with suppression lists**
- [x] Global suppression (hard bounces): blocks in ALL workspaces. Only a superadmin can remove.
- [x] Workspace suppression (complaints): affects only that workspace.
- [x] On send: checks both lists. If the email is in either one → `suppressed`.
- [x] Soft bounce → retry (max 3, exponential backoff). After that → `failed`.
- [x] Alert if bounce rate > 5% in 24h per workspace.
- [x] Alert if complaint rate > 0.1%.

**R-16: Dashboard with metrics**
- [x] Views: global, by tenant, by workspace.
- [x] Trends: 7d, 30d, 90d.
- [x] Detail of individual emails with lifecycle.

**R-17: OIDC authentication + membership**
Authentication delegated to OIDC. Authorization managed by Senda through membership.
- [x] OIDC provider configuration with discovery URL.
- [x] Post-OIDC: Senda verifies that the email exists as a registered member.
- [x] If not a member → access denied with a clear message.
- [x] If a member → load the member's roles and scopes.

**R-18: Members and roles system**
Members registered by email. Fixed roles with hierarchical scopes.
- [x] Roles: superadmin, tenant-admin, workspace-admin, workspace-editor, workspace-viewer.
- [x] Multiple superadmins allowed.
- [x] A member can have different roles in different scopes.
- [x] Superadmin can add members and assign any role at any level, including creating other superadmins.
- [x] Tenant-admin can add members to their tenant and its workspaces.
- [x] Workspace-admin can add members to their workspace (roles: editor, viewer).
- [x] The permissions table (section 4.10) applies strictly on every endpoint.
- [x] workspace-editor can create/edit drafts but CANNOT publish templates.

**R-19: Initial onboarding**
The first user automatically becomes superadmin.
- [x] If the DB is empty and a user completes OIDC → they are registered as superadmin.
- [x] The system guides the superadmin to create the first tenant (code + name) and first workspace.
- [x] `_system` is created automatically with the first tenant.
- [x] This flow happens only once (if members already exist, login is normal).

**R-20: API Keys for the data plane**
Authentication for the send and query API uses long-lived API Keys.
- [x] Each API Key belongs to a workspace.
- [x] Format: `senda_live_<random>` (prod). It is only shown in full at creation time.
- [x] Stored hashed (never plaintext).
- [x] A workspace can have multiple API Keys.
- [x] Individual revocation.
- [x] API Key allows: send, query status, search emails, export. Only within its workspace.

**R-21: Query and search API**
- [x] By `external_id`, email, combined filters.
- [x] Cursor-based pagination.
- [x] Bulk export.
- [x] Scoped by API Key (workspace-only data).

**R-22: Soft delete and dependency management**
- [x] Deleted resources are marked as `deleted` (soft delete).
- [x] They remain available in inheritance, with a visual warning.
- [x] Purge requires: view dependency list, explicit confirmation.
- [x] Audit trail for deletes and purges.

**R-23: Audit logging**
- [x] Record of all administrative actions.
- [x] Who, when, what, where, change details.
- [x] Append-only (not editable).
- [x] Visible to scope admins and superadmins.

### Nice-to-Have (P1)

**R-24: Drag-and-drop visual editor + inline editing with i18n**
- [ ] Draggable components.
- [ ] Inline editing in preview.
- [ ] Visual variable selector.
- [ ] Toggle visual mode / code mode.
- [ ] Text blocks with locale tags for language variants.
- [ ] Default locale selector for the template.

**R-25: AI integration in the editor**
- [ ] Assistance for template creation (suggest structure and copy).
- [ ] Automatic translation of text blocks.
- [ ] Copy optimization (clarity, tone, engagement).
- [ ] Consistency checks between language variants.

**R-26: Open tracking (opt-in per workspace)**
- [ ] Disabled by default.
- [ ] Workspace-admin enables/disables it.
- [ ] Tracking pixel inserted automatically.
- [ ] Configurable disclaimer in the footer when active.

**R-27: Event webhooks**
- [ ] Configuration per workspace.
- [ ] Exponential backoff retry (max 5 attempts).
- [ ] HMAC-SHA256 signature for verification.
- [ ] Events: sent, delivered, opened, bounced, complained, failed.
- [ ] Payload: `{ event, tracking_id, external_id, workspace, tenant, timestamp, data }`.

**R-28: Rate limiting by adapter**
- [ ] Distributed token bucket (Redis).
- [ ] Configurable by adapter, tenant, workspace.
- [ ] Backpressure without failing: enqueue and wait, do not drop.
- [ ] Note: basic rate limiting to respect SES/Gmail limits must be in Phase 1.

**R-29: Test emails from dashboard**
- [ ] Send a test email from the editor (authenticated via OIDC, not API Key).
- [ ] Executes the full pipeline except: does not register in the real lifecycle, marks as "test".
- [ ] Recipient: the email of the member performing the test, or another specified email.

### Future Considerations (P2)

**R-30:** SMTP Relay.
**R-31:** Additional adapters (SendGrid, Mailgun, Postmark).
**R-32:** Template A/B testing.
**R-33:** Send scheduling.
**R-34:** Template sharing between workspaces.
**R-35:** Customizable roles (full RBAC with permissions configurable per installation).
**R-36:** Batch endpoint `/api/v1/send/batch` for bulk sends (up to 10K recipients, returns `batch_id`).

---

## 7. Technology Stack

### Backend

| Component | Technology | Rationale |
|---|---|---|
| Language | Go 1.23+ | Performance, static binary, native concurrency |
| Web Framework | Echo 4.x | HTTP/2, net/http middleware, mature ecosystem |
| Database | PostgreSQL 16+ | ACID, RLS, partitioning |
| Job Queue | River (PostgreSQL) | Transactional, ~10K jobs/sec, Web UI |
| Templates | gomjml (pure Go) | ~3ms compile, ~2MB RAM |
| Email Auth (SPF/DKIM/DMARC) | Provider-managed (SES/Gmail) | Avoids duplicating signing/validation in the app |
| Cache/Rate Limit | PostgreSQL (UNLOGGED + PL/pgSQL) | No Redis, unified operations in PG |

### Frontend

| Component | Technology |
|---|---|
| Core | React 19 + React Compiler |
| Build | Vite 6 |
| Styles | Tailwind CSS v4 |
| State | TanStack Query v5 + Zustand |

### Deployment

Docker + Docker Compose. Caddy optional for HTTPS.

### Architecture Diagram

```
                         ┌──────────────────────────┐
   External Service ───► │      REST API (Echo)      │
                         │  tenantCode:wsCode:type   │
                         └────────────┬─────────────┘
                                      │
                         ┌────────────▼─────────────┐
                         │     Application Core      │
                         │                           │
                         │  ┌─ Hierarchy Resolver    │
                         │  ├─ Injector Resolver     │
                         │  ├─ Template Engine       │
                         │  ├─ Lifecycle Tracker     │
                         │  ├─ Suppression Manager   │
                         │  ├─ Domain Verifier       │
                         │  └─ Audit Logger          │
                         └────────────┬─────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
     ┌────────▼────────┐   ┌─────────▼────────┐   ┌─────────▼────────┐
     │  Port: Sender    │   │  Port: Store     │   │  Port: Queue     │
     │  ┌─ SESAdapter   │   │  ┌─ PostgreSQL   │   │  ┌─ River        │
     │  └─ GmailAdapter │   │  └─              │   │  └─              │
     └─────────────────┘   └──────────────────┘   └──────────────────┘
```

---

## 8. Success Metrics

### Leading

| Metric | Target |
|---|---|
| API response time (send) | < 100ms p99 |
| Delivery rate | > 98% |
| Template compilation | < 10ms p99 |
| Injector resolution | < 5ms p99 |
| Addressing resolution | < 2ms p99 |

### Lagging

| Metric | Target |
|---|---|
| Bounce rate | < 2% |
| Complaint rate | < 0.1% |

---

## 9. Open Questions

### Blocking

**OQ-01 [Engineering]:** ~~Open tracking.~~ **RESOLVED:** Opt-in by workspace. Disabled by default. Automatic disclaimer when enabled.

**OQ-02 [Engineering]:** Visual editor: custom, GrapeJS, Unlayer, or another open-source tool? Decision deferred to Phase 3 (visual editor).

**OQ-03 [Product]:** ~~OIDC → tenant/workspace mapping~~ **RESOLVED:** Mapping is by explicit membership. An admin registers the member email and assigns roles in specific scopes. There is no automatic mapping by domain or claim.

**OQ-04 [Engineering]:** Gmail: 2K msg/day/account. Rotation? Document limitation? → Document as a known limitation of the GmailAdapter. Account rotation as a future feature.

**OQ-05 [Product]:** ~~external_id: required or optional?~~ **RESOLVED:** Optional. If not sent, the email is tracked only by the internal `tracking_id`.

**OQ-06 [Engineering]:** ~~Injector schema~~ **RESOLVED:** Fixed top-down schema. The level that creates the injector defines fields + types. Lower levels only override values; they cannot add or remove fields.

### Non-Blocking

**OQ-07:** PostgreSQL partitioning (month + tenant).
**OQ-08:** Additional RLS or application-only.
**OQ-09:** Dashboard: SPA or server-rendered.
**OQ-10:** AI integration: which model/provider? Self-hosted or external API?
**OQ-11:** API versioning: URL prefix (`/v1/`, `/v2/`) or header? Recommended: URL prefix.

---

## 10. Phasing

### Phase 1 — Core
3-level hierarchy + `_system`, slug codes, typed injectors (top-down schema), API with deterministic addressing (array `to`, `cc`, `bcc`, `locale`), code-based MJML templates (with subject, preview text, from address), versioning with states, kill switch, lifecycle tracking, optional `external_id`, SES/Gmail adapters, provider-managed email authentication (no in-app DKIM), members and roles (multiple superadmins), initial onboarding, API Keys (send + query), OIDC, suppression lists (global + workspace), soft delete + dependencies, audit logging, basic rate limiting (respect SES limits), basic i18n support (field `locale` in the API + template variant resolution).

### Phase 2 — Observability
Dashboard, metrics, advanced search, bulk export, bounce/complaint alerts, test emails from dashboard.

### Phase 3 — UX
Drag-and-drop visual editor + inline editing, visual management of language variants in the editor, AI integration (translation + template creation + copy), event webhooks.

### Phase 4 — Hardening
Advanced rate limiting (distributed token bucket), Gmail adapter, open tracking (opt-in), performance optimization, open source documentation.

---

*Draft for iteration. Nothing is final.*
