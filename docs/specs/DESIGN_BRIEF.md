# Design Brief — Senda Dashboard

**For:** UX/UI Team

**Reference:** PRD v5.0 | TECH_SPEC v1.4

**Platform:** Responsive web, mobile-first

**UI Stack:** Tailwind CSS + shadcn/ui

**Auth:** External OIDC (Google Workspace, Keycloak, etc.)

---

## 1. Product Summary

Senda is an open-source transactional email orchestration platform. The dashboard lets administrators manage a 3-level hierarchy (Global → Tenant → Workspace), configure email templates with inheritance, manage domains, email-sending adapters, and monitor the full lifecycle of every email.

**Dashboard audience:** Technical and semi-technical administrators at companies that manage multiple brands/regions/customers. This is not a consumer product — it is an internal operations tool.

---

## 2. Roles and Permissions (Affects the Entire UI)

Every screen must respect the active role's permissions. Unauthorized elements are **hidden** (not disabled).

| Role | Scope | Sees | Can mutate |
|-----|-------|-----|-------------|
| **superadmin** | Global | Everything | Everything |
| **tenant_admin** | Their tenant | Everything in the tenant + inherited global items | _system config, workspaces, tenant members |
| **workspace_admin** | Their workspace | Everything in the workspace + inherited items | Workspace config, members, API keys, webhooks |
| **workspace_editor** | Their workspace | Everything in the workspace | Templates (draft only), injectors |
| **workspace_viewer** | Their workspace | Everything in the workspace | Nothing (read-only) |

**Multiple scopes:** A user can have roles in different tenants/workspaces. The dashboard must allow navigation across scopes.

---

## 3. Information Architecture

### 3.1. Primary Navigation

```
┌──────────────────────────────────────────────────────┐
│  HEADER                                              │
│  [Logo Senda] [Scope Switcher ▼] [...] [User Menu ▼]│
├──────────┬───────────────────────────────────────────┤
│ SIDEBAR  │  CONTENT AREA                             │
│          │                                           │
│ Dashboard│                                           │
│ Emails   │                                           │
│ Templates│                                           │
│ Injectors│                                           │
│ Adapters │                                           │
│ Domains  │                                           │
│ Webhooks │                                           │
│ Members  │                                           │
│ API Keys │                                           │
│ Audit Log│                                           │
│ Settings │                                           │
│          │                                           │
│ ──────── │                                           │
│ [?] Help │                                           │
└──────────┴───────────────────────────────────────────┘
```

**Mobile:** Sidebar collapses into a hamburger menu. Header stays sticky.

### 3.2. Scope Switcher

Central navigation element. Shows the active scope and lets the user switch.

**Three UI scopes** (the "tenant" level is merged into the `_system` workspace):

| Scope | URL pattern | Icon | Description |
|-------|------------|------|-------------|
| Global | `/global` | Globe | Platform administration (superadmin) |
| Tenant | `/t/{code}/w/_system` | Building2 | Tenant management via `_system` workspace |
| Workspace | `/t/{code}/w/{code}` | Layers | Operational workspace |

**Navigation flow:**

- Clicking **Global** navigates to `/global`
- Clicking a **Tenant** navigates directly to `/t/{code}/w/_system` (no intermediate step)
- Once inside a tenant, the switcher shows all workspaces for switching
- "Manage Workspaces" button navigates to `/t/{code}/w/_system/workspaces`

**Scope indicator** shows the tenant name (not "_system") when in the `_system` workspace.

Legacy URLs like `/t/{code}` or `/t/{code}/adapters` redirect to `/t/{code}/w/_system/...`.

### 3.3. Screen Map

```
Onboarding (first use)
├── Welcome
├── Create first tenant
└── Create first workspace

Login (OIDC redirect)
├── Success → Dashboard
└── Not a member → Access denied screen

Dashboard (home)
├── Sending metrics (current scope)
├── Recent activity
└── Alerts (bounce rate, domains with errors)

Emails
├── Email list (paginated table + filters)
├── Email detail (event timeline)
└── Search (by tracking_id, external_id, recipient)

Templates
├── Template Types (list + CRUD)
│   └── Assign adapter
├── Templates by type (list)
│   ├── Create/edit template
│   ├── Versions (list with states)
│   │   ├── Version editor (MJML visual + code)
│   │   ├── Preview (desktop + mobile)
│   │   ├── Locales (i18n per version)
│   │   └── Publish / Archive
│   └── Disable template (kill switch)
└── Test send (preview + test delivery)

Injectors
├── Definitions list (shows scope + inheritance)
├── Create/edit definition (schema: fields + types)
└── Edit values (per scope, field by field)

Adapters
├── Adapters list (shows scope + default)
├── Create/edit adapter (type + credentials)
└── Test connection

Domains
├── Domains list (with status badge)
├── Register domain (generates DNS records)
├── View DNS records (copyable)
└── Verify domain (manual + status)

Webhooks
├── Webhooks list
├── Create/edit webhook (URL + events)
└── Test webhook (send ping)

Members
├── Members list (with roles per scope)
├── Invite member (email + role + scope)
└── Edit roles / Revoke access

API Keys
├── Keys list (hint visible, key hidden)
├── Generate new key (shows key once)
└── Revoke key

Audit Log
├── Events list (paginated table + filters)
└── Event detail (changes, before/after)

Settings (global scope only)
├── General configuration
├── OIDC settings
└── Email defaults (retries, retention, rate limits)
```

### 3.4. Sidebar Visibility Matrix

The sidebar shows different panels depending on the current scope. This is the source of truth for what appears at each level.

| Panel | Global | Tenant (`_system`) | Workspace |
|-------|--------|-------------------|-----------|
| Dashboard | Yes | Yes | Yes |
| Tenants | Yes | - | - |
| Workspaces | - | Yes | - |
| Emails | Yes | Yes (aggregated) | Yes |
| Templates | - | - | Yes |
| Injectors | - | - | Yes |
| Adapters | - | Yes | Yes |
| Webhooks | - | - | Yes |
| Members | Yes | Yes | Yes |
| API Keys | - | - | Yes |
| Audit Log | Yes | Yes | Yes |
| Settings | Yes | Yes | Yes |

**Panel categories:**

- **Observation** (Dashboard, Emails, Audit Log): aggregate by scope -- show wherever there is data to observe
- **Configuration** (Adapters, Templates, Injectors, Webhooks, API Keys): live at operational scope only, require sharing mechanisms to span levels
- **Management** (Tenants, Workspaces, Members, Settings): appear where organizationally relevant

**Per-scope summary:**

- **Global** (6 panels): Dashboard, Tenants, Emails, Members, Audit Log, Settings -- pure platform administration
- **Tenant** (7 panels): Dashboard, Workspaces, Emails, Adapters, Members, Audit Log, Settings -- organizational management + shared infrastructure
- **Workspace** (10 panels): everything except Tenants and Workspaces -- full operational scope

**Implementation:** The sidebar computes `effectiveLevel` as `"system"` when `workspaceCode === "_system"`, `"workspace"` for regular workspaces, or `"global"`. Each nav item carries a `vis` object with boolean flags per level.

---

## 4. User Flows

### 4.1. Onboarding (First Use)

**Trigger:** First time someone accesses Senda with an empty DB.

```
[User opens Senda] → [Welcome screen]
    "Welcome to Senda. Set up your platform in 3 steps."

→ Step 1: Login with OIDC
    [Button: "Connect with your OIDC provider"]
    → Redirect to OIDC provider → Callback
    → Auto-registration as superadmin

→ Step 2: Create first tenant
    - Field: Code (slug, auto-suggest from name)
    - Field: Name
    [Button: "Create Tenant"]
    → Creates tenant + _system workspace automatically

→ Step 3: Create first workspace
    - Field: Code (slug)
    - Field: Name
    [Button: "Create Workspace"]
    → Redirect to the Dashboard

→ [Dashboard with contextual guide]
    Visible checklist:
    ☐ Configure sending adapter
    ☐ Verify a domain
    ☐ Create first template type
    ☐ Create first template
    ☐ Send first test email
```

**State:** The checklist persists until all steps are complete. It can be dismissed.

### 4.2. Sending from API (no screen — context reference)

```
[External service] → POST /api/v1/send
    {
      to: "user@example.com",
      template_type: "latam:acme:welcome",
      variables: { user_name: "Juan" },
      locale: "es"
    }
→ Response: { tracking_id: "abc123", status: "queued" }
```

The dashboard shows the result in the Emails section.

### 4.3. Create and Publish a Template

```
[Navigate to Templates] → [View Template Types]
→ Select type "welcome"
→ [View templates for that type]
    - If no template exists in the current scope:
      [Button: "+ Create Template"]
    - If a template already exists:
      [View versions]

→ [Create new version (draft)]
    → Opens editor with 2 modes:
      - Visual (drag-and-drop MJML blocks)
      - Code (MJML editor with syntax highlighting)
    → Side panel: subject, preview text, from_name, from_email, reply_to
    → Variables bar: selector for template type variables + injectors
    → Preview: desktop/mobile toggle
    → Locales: tab to add translations

→ [Save Draft]
    → Draft appears in the versions list

→ [Publish] (admin+ only)
    → Confirms: "Publish version 3? Version 2 will be archived."
    → Previous version → archived
    → New version → published
```

### 4.4. Configure Adapter + Assign to Template Type

```
[Navigate to Adapters] → [+ Create Adapter]
    - Name: "Production SES"
    - Type: SES | Gmail (selector)
    - Dynamic form by type:
      SES: Region, Access Key ID, Secret Access Key
      Gmail: OAuth Client ID, Client Secret, Refresh Token, Delegate Email
    - Rate limit: emails/second (default: 14)
    [Button: "Save"]
    → Credentials are encrypted automatically
    → [Button: "Test Connection"] → Sends a test email to the current user

→ [Navigate to Template Types] → Select type
    → Field: "Assigned Adapter" [Dropdown of available adapters]
    → Save
```

### 4.5. Register and Verify Domain

```
[Navigate to Domains] → [+ Register Domain]
    - Field: domain (e.g. example.com)
    [Button: "Register"]
    → Senda generates: DKIM key, DNS records

→ [View registered domain]
    Status: ⏳ Pending

    Section "DNS records to configure":
    ┌──────────────────────────────────────────────────────┐
    │ Type │ Name                     │ Value        │ 📋 │
    │ TXT  │ senda._domainkey.example… │ v=DKIM1; ... │ 📋 │
    │ TXT  │ example.com               │ v=spf1 ...   │ 📋 │
    │ TXT  │ _dmarc.example.com        │ v=DMARC1; .. │ 📋 │
    └──────────────────────────────────────────────────────┘
    (Each value has a copy-to-clipboard button)

    [Button: "Verify Now"]
    → Check DNS → Status: ✅ Verified | ❌ Error (with details)
```

### 4.6. Manage Injectors (Field-by-Field Inheritance)

```
[Navigate to Injectors]
→ List shows: definitions in the current scope + inherited ones
    Each row indicates: name, source scope, number of fields

    ┌──────────────────────────────────────────────┐
    │ Name       │ Scope      │ Fields │ Actions   │
    │ brand      │ 🌐 Global  │ 3      │ [Values]  │
    │ footer     │ ⚙️ _system │ 2      │ [Values]  │
    │ support    │ 📦 Current  │ 4      │ [Edit]    │
    └──────────────────────────────────────────────┘

→ [Edit values for "brand" in the current scope]
    Shows schema fields with current value per level:

    ┌──────────────────────────────────────────────────────┐
    │ Field: logo                                         │
    │ Type: img                                           │
    │ Global value:    corp-logo.png (🌐)                │
    │ _system value:   — (inherits global)               │
    │ Workspace value: [_____________] [Save]            │
    │                  "Leave blank to inherit from above"│
    ├─────────────────────────────────────────────────────┤
    │ Field: company_name                                 │
    │ Type: text                                          │
    │ Global value:    "MiEmpresa" (🌐)                  │
    │ _system value:   "MiEmpresa LATAM" (⚙️)          │
    │ Workspace value: [_____________] [Save]            │
    └──────────────────────────────────────────────────────┘
```

### 4.7. Monitor Emails

```
[Navigate to Emails]
→ Paginated table with filters:
    Filters: status, date (range), recipient, template type, external_id
    Search: by tracking_id or email

    ┌───────────────────────────────────────────────────────────────┐
    │ To             │ Template    │ Status       │ Date            │
    │ user@test.com  │ welcome     │ ✅ delivered │ 2026-02-16 14:30│
    │ bob@corp.com   │ invoice     │ 📨 sent      │ 2026-02-16 14:28│
    │ bad@invalid.xx │ welcome     │ ❌ bounced   │ 2026-02-16 14:25│
    │ spam@user.com  │ notification│ ⚠️ complained │ 2026-02-16 14:20│
    └───────────────────────────────────────────────────────────────┘

→ [Click email] → Detail:
    Panel with email info:
    - To, From, Subject, Template (link), used Adapter
    - Tracking ID, External ID (if any)
    - Variables snapshot, Injectors snapshot (collapsible)

    Event timeline:
    ┌────────────────────────────────────────────┐
    │ ● Queued        │ 14:30:01                 │
    │ ● Sent          │ 14:30:03 (SES, msg-id)   │
    │ ● Delivered     │ 14:30:15                 │
    │ ● Opened        │ 14:35:22 (if tracking on)│
    └────────────────────────────────────────────┘
```

### 4.8. Manage Members and Roles

```
[Navigate to Members]
→ List of members with their roles in the current scope:

    ┌──────────────────────────────────────────────────────────┐
    │ Email              │ Name      │ Role             │      │
    │ rey@empresa.com    │ Rey       │ superadmin (🌐)  │ [...]│
    │ maria@empresa.com  │ Maria     │ tenant_admin (🏢)│ [...]│
    │ dev@empresa.com    │ Dev Team  │ ws_editor (📦)   │ [...]│
    └──────────────────────────────────────────────────────────┘

→ [+ Invite Member]
    - Field: Email
    - Selector: Role
    - Selector: Scope (based on role)
    [Button: "Invite"]
    → Stored in DB. After OIDC login, the user gets access.
    (No invitation email is sent in P1 — the admin notifies them manually)
```

---

## 5. Detailed Screens

### 5.1. Login / Access Denied

**Login:**
- Centered screen, Senda logo
- Single button: "Sign in with [Provider]"
- Redirect to OIDC → callback → dashboard

**Access Denied (authenticated user who is not a member):**
- Centered screen
- Message: "Access denied. Your email (user@example.com) is not registered as a member. Contact your administrator."
- Button: "Sign out"

### 5.2. Dashboard (Home)

**Content by scope:**

**Global scope (superadmin):**
- Global metrics: total emails today, delivery rate, bounce rate, complaint rate
- Chart: emails sent over the last 7/30 days (line chart)
- Top 5 tenants by volume
- Active alerts (domains with errors, high bounce rate)
- Onboarding checklist (if not completed)

**Tenant scope:**
- Tenant metrics: same metrics, filtered
- Top 5 workspaces in the tenant by volume
- Tenant alerts

**Workspace scope:**
- Workspace metrics
- Last 10 sent emails (mini-table)
- Templates with drafts pending publication
- Workspace alerts

### 5.3. Emails — List

- **Responsive table:** On mobile, collapses into cards
- **Columns:** Recipient, Template Type, Status (color badge), Date, Tracking ID
- **Filters (collapsible panel on mobile):**
  - Status: multiselect (queued, sent, delivered, bounced, complained, failed, suppressed)
  - Date: date range picker
  - Template Type: dropdown
  - Search: text input (searches tracking_id, external_id, recipient, sender)
- **Pagination:** Cursor-based ("Previous" / "Next" buttons, no page numbers)
- **Empty state:** "No emails in this period. Emails will appear here when they are sent via the API."

### 5.4. Emails — Detail

- **Header:** Large status badge + recipient + subject
- **Info section:**
  - To, CC, BCC
  - From (email + name)
  - Template: link to the template type + used version
  - Adapter: name of the adapter used
  - Tracking ID (copyable)
  - External ID (copyable, if any)
  - Locale
  - Sent date
- **Timeline section:** Vertical, icons by event type, timestamp
- **Snapshots section (collapsible):**
  - Sent variables (formatted JSON)
  - Resolved injectors (formatted JSON)
- **Error section (if failed/bounced):**
  - Provider error message
  - Bounce type (soft/hard)
  - Retry count

### 5.5. Template Types — List

- **Table:** Slug, Name, Assigned Adapter (badge or "⚠️ No adapter"), Source scope, Templates count
- **Row actions:** Edit, Assign adapter
- **Empty state:** "No template types configured. Create a type to define the variable contract your templates will use."
- **Inheritance indicator:** Icon showing whether it comes from global, _system, or is local

### 5.6. Templates — List and Versions

**Template list by type:**
- Shows whether there is a template in the current scope, _system, or global
- If there is a local template: link to the editor
- If there is none: button "Create template for this scope"
- If it is inherited: badge "Inherited from [scope]" with an option to create an override

**Template versions:**
- Table: Version #, Status (badge: draft/published/archived), Created by, Date, Actions
- Only one version can be "published" (highlighted)
- Actions: Edit (if draft), Publish (if draft + admin), Archive, Preview

### 5.7. Template Editor

**Two-panel layout (or tabs on mobile):**

```
┌─────────────────────────┬──────────────────────────┐
│ EDITOR                  │ PREVIEW                  │
│                         │                          │
│ [Visual] [Code]         │ [Desktop] [Mobile]       │
│                         │                          │
│ ┌─────────────────────┐ │ ┌──────────────────────┐ │
│ │ Drag-and-drop MJML  │ │ │ Rendered HTML        │ │
│ │ blocks:             │ │ │ preview              │ │
│ │                     │ │ │                      │ │
│ │ [Header]            │ │ │                      │ │
│ │ [Text Block]        │ │ │                      │ │
│ │ [Image]             │ │ │                      │ │
│ │ [Button]            │ │ │                      │ │
│ │ [Divider]           │ │ │                      │ │
│ │ [Columns]           │ │ │                      │ │
│ │                     │ │ │                      │ │
│ └─────────────────────┘ │ └──────────────────────┘ │
│                         │                          │
│ ── Metadata ─────────── │                          │
│ Subject: [___________]  │                          │
│ Preview: [___________]  │                          │
│ From:    [___________]  │                          │
│ Reply-to:[___________]  │                          │
│                         │                          │
│ ── Variables ────────── │                          │
│ {{user_name}}  [Insert] │                          │
│ {{brand.logo}} [Insert] │                          │
│                         │                          │
│ ── Locales ──────────── │                          │
│ [es] [en] [pt] [+ Add] │                          │
└─────────────────────────┴──────────────────────────┘

[Save Draft]  [Full Preview]  [Send Test]  [Publish ▼]
```

**UX/UI notes:**
- The visual editor is an MJML block editor (similar to Stripo, Unlayer)
- Code mode: MJML syntax highlighting with variable autocomplete
- Preview updates in real time as the user edits
- Available variables are shown in a sidebar/panel with click-to-insert
- Locales: tabs to edit subject + body per locale. Base = default_locale
- "Send Test": opens a modal to enter recipient email + test variables

- "Publish": visible only to admin+ and requires confirmation.

### 5.8. Injectors — Edit Values

**Field-by-field editing layout:**

Each injector field shows its visual inheritance chain:

```
Field: logo (type: img)
┌──────────────────────────────────────────────┐
│ 🌐 Global:     corp-logo.png                │
│ ⚙️ _system:    — (inherits global)          │
│ 📦 Workspace:  [___________] [Save] [🗑️]   │
│                                              │
│ Effective value: corp-logo.png (from Global)│
└──────────────────────────────────────────────┘
```

- Input type changes based on field_type: text → text input, img → URL + preview, html → rich text, url → URL input, number → number input, bool → toggle
- When an override is saved, the "effective value" changes
- When an override is deleted (🗑️), inheritance is restored
- Clear visual indication of which level provides the current value

### 5.9. Adapters — CRUD

**List:**
- Name, Type (badge: SES/Gmail), Scope, Default (✓ or —), Rate limit, Status
- Indicator: "Used by N template types"

**Create/Edit:**
- Name
- Type: selector (SES, Gmail) — changes the credentials form dynamically
- Credentials (form by type):
  - SES: Region, Access Key ID, Secret Access Key
  - Gmail: OAuth Client ID, Client Secret, Refresh Token, Delegate Email
- Rate limit: number input (emails/sec), default 14
- Mark as default: toggle
- [Test Connection]: sends a test email to the current admin email

**Visual security:** Credentials fields show "••••••••" after saving. They cannot be viewed again, only replaced.

### 5.10. Domains

**List:**
- Domain, Status badge (⏳ Pending / ✅ Verified / ❌ Error), Scope, Last verification
- Click → detail

**Detail:**
- Large status with last verification
- Table of copyable DNS records (copy button per row)
- If error: specific error message ("TXT record not found", "DKIM key mismatch")
- [Verify Now] → loading → result
- Verification timeline (last 5)

### 5.11. Webhooks

**List:**
- URL (truncated), Subscribed events (badges), Status (active/disabled), Failures
- If consecutive_failures > 0: warning badge

**Create/Edit:**
- URL (HTTPS required)
- Events: multiselect checkboxes (sent, delivered, bounced, complained, failed)
- Auto-generated: Secret (shown once on create)
- [Test Webhook]: sends ping → shows response (status code + latency)

### 5.12. API Keys

**List:**
- Name, Hint (last 8 chars), Created by, Date, Last used, Status
- Never shows the full key

**Generate new:**
- Field: Name (descriptive label)
- [Generate]
- Modal: "Your API Key has been generated. Copy it now — it will never be shown again."
  ```
  senda_live_a3f8b9c2d4e5f6a7b8c9d0e1f2a3b4c5
  ```
  [Copy] [Close]

**Revoke:**
- Confirmation: "Revoke key '{name}' (hint: ...b4c5)? Services using it will stop working."

### 5.13. Audit Log

**Paginated table with filters:**
- Columns: Date, Who (email), Action (badge), Resource, Scope
- Filters: date (range), action (multiselect), resource type, member
- Click → detail with changes (before/after JSON diff)

### 5.14. Settings (Global)

Visible only to superadmin in global scope:
- **OIDC:** Discovery URL, Client ID (client secret hidden)
- **Email defaults:** Max retries, backoff base, log retention days
- **Alerts:** Bounce threshold %, Complaint threshold %
- **Domain:** Recheck interval hours

---

## 6. UI Patterns and Components

### 6.1. Status Badges

| Status | Color | Icon |
|--------|-------|------|
| queued | gray | ⏳ |
| processing | blue | ⏳ |
| sent | blue | 📨 |
| delivered | green | ✅ |
| opened | green | 👁️ |
| bounced | red | ❌ |
| complained | orange | ⚠️ |
| failed | red | ❌ |
| suppressed | gray | 🚫 |
| draft | gray | 📝 |
| published | green | ✅ |
| archived | gray | 📦 |
| pending | yellow | ⏳ |
| verified | green | ✅ |
| error | red | ❌ |
| active | green | ● |
| disabled | gray | ● |
| revoked | red | ● |

### 6.2. Scope Indicators

Always indicate where an inherited resource comes from:
- 🌐 = Global
- ⚙️ = _system (tenant)
- 📦 = Current workspace

### 6.3. Visual Inheritance

When a resource is inherited, show it with:
- Source scope badge
- Subtle text: "Inherited from [scope]"
- If soft-deleted: yellow warning "This resource is marked as deprecated in [scope]. Creating an override is recommended."
- Visible action: "Create override" to replace it in the current scope

### 6.4. Tables

- Sticky header
- Row hover highlight
- Cursor pagination (no page numbers): [← Previous] [Next →]
- Responsive: collapses into cards on mobile
- Empty state always includes a descriptive message + suggested action
- Loading: skeleton rows

### 6.5. Forms

- Inline validation (do not wait for submit)
- Errors below the field in red text
- Required fields: red asterisk
- Slugs: auto-generate from name (with edit option)
- Credentials: password fields with show/hide toggle

### 6.6. Destructive Confirmations

For actions with consequences (publish, revoke, delete, purge):
- Confirmation modal
- Clear impact text
- For purge: show the list of affected dependencies
- Confirmation input for critical actions: "Type DELETE to confirm"

### 6.7. Toasts / Notifications

- Success: green toast (auto-dismiss after 3s)
- Error: red toast (persists until dismissed)
- Warning: yellow toast (auto-dismiss after 5s)
- Position: bottom-right (desktop), bottom-center (mobile)

### 6.8. Loading States

- Tables: skeleton rows
- Forms: disabled button + spinner
- Pages: centered spinner
- Inline actions: spinner replaces the button

---

## 7. Responsive Breakpoints

| Breakpoint | Name | Layout |
|-----------|--------|--------|
| < 640px | Mobile | Single column, hamburger nav, cards instead of tables |
| 640–1024px | Tablet | Collapsible sidebar, tables with horizontal scroll |
| > 1024px | Desktop | Visible sidebar, full tables, 2-panel editor |

**Mobile-first:** Design the mobile experience first, then expand it.

---

## 8. Empty States

Every empty screen must have a useful empty state:

| Screen | Message | Action |
|----------|---------|--------|
| Dashboard (new) | "Welcome to Senda. Complete the initial setup." | Link to checklist |
| Emails | "No emails have been sent yet. Configure a template and send your first email via the API." | Link to API docs |
| Templates | "No template types configured. Create a type to define your email contract." | [+ Create Template Type] |
| Injectors | "No injectors in this scope. Injectors provide automatic data to your templates." | [+ Create Injector] |
| Adapters | "No adapters configured. You need at least one adapter to send emails." | [+ Create Adapter] |
| Domains | "No domains verified. Register a domain to be able to send emails." | [+ Register Domain] |
| Webhooks | "No webhooks configured. Webhooks notify your services about email events." | [+ Create Webhook] |
| API Keys | "No active API Keys. Generate a key so your services can send emails." | [+ Generate API Key] |
| Audit Log | "No events have been recorded yet." | — |

---

## 9. Accessibility Considerations

- Minimum WCAG 2.1 AA contrast
- All forms have associated labels (not placeholder-only)
- Visible focus on all interactive elements
- Full keyboard navigation (tab order)
- Screen reader: ARIA labels on badges, icons, and scope indicators
- Color is not the only status indicator (always pair it with an icon or text)

---

## 10. Visual Template Editor (Phase 3 Specification)

The visual template editor is the dashboard's most complex feature. In P1/Phase 1, it can be only an MJML code editor with preview. The full drag-and-drop editor comes in Phase 3.

**Phase 1 (P1) — Minimal editor:**
- Textarea with MJML syntax highlighting
- Preview panel (HTML rendered from MJML)
- Panel of available variables (click to insert)
- Metadata fields (subject, from, etc.)
- Locale tabs

**Phase 3 — Full visual editor:**
- Drag-and-drop MJML blocks
- Inline text editing on top of the preview
- Visual variable picker (dropdown at cursor position)
- AI assistance for translations and copy
- Undo/redo
- Version diff (compare two versions)

---

## 11. Expected UX/UI Deliverables

1. **Wireframes** for all screens listed in §3.3 (mobile + desktop)
2. **Navigable prototypes** for the main flows (§4.1 to §4.8)
3. **Design system** based on Tailwind + shadcn/ui:
   - Color palette (primary, status colors, scope colors)
   - Typography (headings, body, code)
   - Customized components: scope switcher, visual inheritance, status badges
4. **Component specification** for: scope switcher, inheritance indicator, template editor (phase 1), event timeline
5. **Responsive specs** for the breakpoints defined in §7
