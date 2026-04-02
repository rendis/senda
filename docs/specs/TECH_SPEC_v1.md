# Tech Spec: Senda — Database Schema + Application Architecture

**Version:** 1.4 (DB Schema + Architecture + Operations + Full Migrations)

**Date:** 2026-02-16

**Author:** Rey + Claude (collaborative iteration)

**Reference:** PRD v5.0

**Status:** Under review — nothing in this document is final

---

## Addendum 2026-02-25 — Provider-Managed Email Auth (Source of Truth)

This document contains historical early-design sections (domains/in-app DKIM) that **no longer apply** to the current P0 backend.

**Current decision:**
- SPF/DKIM/DMARC are managed by the provider (SES/Gmail).
- Senda validates and syncs provider identities.
- Senda does not generate DKIM keys or sign DKIM in the application.
- References to `domains`, `DomainResolver`, and `DKIM signer` should be read as **deprecated/historical** for P0.

---

## 1. Schema Design Principles

1. **Unified scope:** Primary resources (injectors, adapters, templates) belong to a `workspace_id`. At the global level, `workspace_id = NULL`. Each tenant's `_system` workspace is a normal workspace with `is_system = true`, which unifies hierarchical resolution.

2. **Resolution chain as a query:** The `workspace → _system → global` inheritance is implemented by querying `workspace_id IN (target_workspace_id, system_workspace_id, NULL)` with priority.

3. **Universal soft delete:** Every entity with hierarchical dependency has `deleted_at`. The resource remains available until an explicit purge.

4. **Append-only audit trail:** `audit_logs` is write-only from the app. It has no UPDATE or DELETE.

5. **UTC timestamps:** All timestamps are stored in UTC. Display timezone handling is the frontend's responsibility.

6. **UUIDv7:** Time-ordered IDs, better for B-tree indexes than UUIDv4.

7. **Encrypted at rest:** Adapter credentials are stored encrypted (AES-256-GCM). The column stores the ciphertext + nonce.

---

## 2. Relationship Diagram (Conceptual)

```
                    ┌─────────────┐
                    │   tenants   │
                    │  code (UK)  │
                    └──────┬──────┘
                           │ 1:N
                    ┌──────▼──────┐
                    │ workspaces  │
                    │ code (UK/T) │
                    │ is_system   │
                    └──────┬──────┘
                           │
          ┌────────────────┼────────────────┬──────────────────┐
          │                │                │                  │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌─────▼──────┐  ┌───────▼───────┐
   │  injectors  │  │  adapters   │  │  domains   │  │template_types │
   │(schema+vals)│  │  (config)   │  │  (DKIM)    │  │  (contract)   │
   └─────────────┘  └─────────────┘  └────────────┘  └───────┬───────┘
                                                              │ 1:1/scope
                                                       ┌──────▼──────┐
                                                       │  templates  │
                                                       │ (per scope) │
                                                       └──────┬──────┘
                                                              │ 1:N
                                                       ┌──────▼──────┐
                                                       │  versions   │
                                                       │(draft/pub/…)│
                                                       └──────┬──────┘
                                                              │ 1:N
                                                       ┌──────▼──────┐
                                                       │   locales   │
                                                       │ (i18n opt.) │
                                                       └─────────────┘

   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
   │   members   │  │   api_keys  │  │   emails    │  │ audit_logs  │
   │  (by email) │  │(per ws)     │  │ (tracking)  │  │(append-only)│
   └──────┬──────┘  └─────────────┘  └──────┬──────┘  └─────────────┘
          │ 1:N                              │ 1:N
   ┌──────▼──────┐                    ┌──────▼──────┐
   │member_roles │                    │email_events │
   │(per scope)  │                    │(lifecycle)  │
   └─────────────┘                    └─────────────┘

   ┌──────────────────┐  ┌──────────────────────┐
   │suppression_global│  │suppression_workspace │
   │  (hard bounces)  │  │   (complaints)       │
   └──────────────────┘  └──────────────────────┘
```

---

## 3. Complete SQL Schema

### 3.1. Enumerated Types

```sql
-- Lifecycle states for emails
CREATE TYPE email_status AS ENUM (
    'queued',
    'processing',
    'sent',
    'delivered',
    'opened',
    'bounced',
    'complained',
    'failed',
    'suppressed'
);

-- Template version states
CREATE TYPE version_status AS ENUM (
    'draft',
    'published',
    'archived'
);

-- Adapter types
CREATE TYPE adapter_type AS ENUM (
    'ses',
    'gmail'
);

-- DEPRECATED (provider-managed auth): kept for historical reference only.
-- Domain verification status
CREATE TYPE domain_status AS ENUM (
    'pending',
    'verified',
    'error'
);

-- Member roles
CREATE TYPE member_role AS ENUM (
    'superadmin',
    'tenant_admin',
    'workspace_admin',
    'workspace_editor',
    'workspace_viewer'
);

-- Role scope types
CREATE TYPE scope_type AS ENUM (
    'global',
    'tenant',
    'workspace'
);

-- Injector field types
CREATE TYPE injector_field_type AS ENUM (
    'text',
    'number',
    'bool',
    'img',
    'url',
    'html'
);

-- Bounce types
CREATE TYPE bounce_type AS ENUM (
    'soft',
    'hard'
);

-- Suppression reasons
CREATE TYPE suppression_reason AS ENUM (
    'hard_bounce',
    'complaint',
    'manual'
);

-- Audit log actions
CREATE TYPE audit_action AS ENUM (
    'create',
    'update',
    'delete',
    'purge',
    'publish',
    'archive',
    'disable',
    'enable',
    'revoke',
    'invite',
    'remove_role'
);
```

### 3.2. Tenants

```sql
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT tenants_code_unique UNIQUE (code),
    CONSTRAINT tenants_code_format CHECK (
        code ~ '^[a-z][a-z0-9-]*$'
        AND length(code) BETWEEN 2 AND 50
        AND code NOT LIKE '%-'
        AND code NOT LIKE '%---%'
        AND code NOT IN ('_system', 'global', 'admin', 'api', 'system')
    )
);

CREATE INDEX idx_tenants_code ON tenants (code) WHERE deleted_at IS NULL;
```

### 3.3. Workspaces

```sql
CREATE TABLE workspaces (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    code        VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    is_system   BOOLEAN NOT NULL DEFAULT false,

    -- Open tracking opt-in (default: off)
    open_tracking_enabled BOOLEAN NOT NULL DEFAULT false,

    -- Default locale for templates in this workspace
    default_locale VARCHAR(10),

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT workspaces_code_per_tenant UNIQUE (tenant_id, code),
    CONSTRAINT workspaces_code_format CHECK (
        code = '_system' OR (
            code ~ '^[a-z][a-z0-9-]*$'
            AND length(code) BETWEEN 2 AND 50
            AND code NOT LIKE '%-'
            AND code NOT LIKE '%---%'
            AND code NOT IN ('global', 'admin', 'api', 'system')
        )
    ),
    -- Only one _system per tenant
    CONSTRAINT workspaces_one_system_per_tenant
        EXCLUDE USING btree (tenant_id WITH =) WHERE (is_system = true AND deleted_at IS NULL)
);

CREATE INDEX idx_workspaces_tenant ON workspaces (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_workspaces_code ON workspaces (tenant_id, code) WHERE deleted_at IS NULL;
```

### 3.4. Injectors (Data Injectors)

```sql
-- Schema definition: who created the injector and what fields it has
CREATE TABLE injector_definitions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,

    -- Where the schema was defined. NULL = global.
    workspace_id    UUID REFERENCES workspaces(id),

    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    -- Name unique within its creation scope
    CONSTRAINT injector_def_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (name, workspace_id)
);

CREATE INDEX idx_injector_defs_workspace ON injector_definitions (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_injector_defs_global ON injector_definitions (name) WHERE workspace_id IS NULL AND deleted_at IS NULL;

-- Fields of an injector schema (immutable after creation, defined by the owner level)
CREATE TABLE injector_fields (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    injector_definition_id  UUID NOT NULL REFERENCES injector_definitions(id) ON DELETE CASCADE,
    field_name              VARCHAR(100) NOT NULL,
    field_type              injector_field_type NOT NULL,
    description             TEXT,
    position                INT NOT NULL DEFAULT 0,

    CONSTRAINT injector_field_unique
        UNIQUE (injector_definition_id, field_name)
);

-- Values set at each level of the hierarchy
-- The owner level sets the initial values; lower levels can override
CREATE TABLE injector_values (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    injector_definition_id  UUID NOT NULL REFERENCES injector_definitions(id),
    field_name              VARCHAR(100) NOT NULL,

    -- Where this value is set. NULL = global.
    workspace_id            UUID REFERENCES workspaces(id),

    -- Value stored as JSONB for type flexibility
    -- Validated at app level against the field_type from injector_fields
    value                   JSONB NOT NULL,

    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT injector_value_unique
        UNIQUE NULLS NOT DISTINCT (injector_definition_id, field_name, workspace_id)
);

CREATE INDEX idx_injector_values_lookup
    ON injector_values (injector_definition_id, workspace_id);
```

### 3.5. Adapters (Email Sending)

```sql
CREATE TABLE adapters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,

    -- NULL = global adapter
    workspace_id    UUID REFERENCES workspaces(id),

    adapter_type    adapter_type NOT NULL,

    -- Encrypted credentials (AES-256-GCM ciphertext)
    -- SES: {region, access_key_id, secret_access_key}
    -- Gmail: {oauth_client_id, oauth_client_secret, refresh_token, delegate_email}
    config_encrypted BYTEA NOT NULL,

    -- Is this the default adapter for its scope?
    -- Multiple adapters can exist per scope but only one default.
    is_default      BOOLEAN NOT NULL DEFAULT false,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    -- Only one default adapter per scope
    CONSTRAINT adapters_one_default_per_scope
        EXCLUDE USING btree (workspace_id WITH =) WHERE (is_default = true AND deleted_at IS NULL)
);

-- For global default:
CREATE UNIQUE INDEX idx_adapters_global_default
    ON adapters ((true)) WHERE workspace_id IS NULL AND is_default = true AND deleted_at IS NULL;

CREATE INDEX idx_adapters_workspace ON adapters (workspace_id) WHERE deleted_at IS NULL;
```

### 3.6. Verified Domains *(DEPRECATED in P0 provider-managed)*

```sql
-- DEPRECATED: the domains and in-app DKIM flow is not part of the current P0 backend.
CREATE TABLE domains (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- NULL = global domain
    workspace_id            UUID REFERENCES workspaces(id),

    domain_name             VARCHAR(255) NOT NULL,
    status                  domain_status NOT NULL DEFAULT 'pending',

    -- DKIM
    dkim_selector           VARCHAR(63) NOT NULL,
    dkim_private_key_encrypted BYTEA NOT NULL,
    dkim_public_key         TEXT NOT NULL,

    -- Generated DNS records for the admin to configure
    dns_records             JSONB NOT NULL DEFAULT '[]',
    -- Example: [
    --   {"type": "TXT", "name": "senda._domainkey.example.com", "value": "v=DKIM1; k=rsa; p=..."},
    --   {"type": "TXT", "name": "example.com", "value": "v=spf1 include:amazonses.com ~all"},
    --   {"type": "TXT", "name": "_dmarc.example.com", "value": "v=DMARC1; p=quarantine; ..."}
    -- ]

    verified_at             TIMESTAMPTZ,
    last_check_at           TIMESTAMPTZ,
    next_check_at           TIMESTAMPTZ,
    last_error              TEXT,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ,

    -- Domain unique per scope
    CONSTRAINT domains_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (domain_name, workspace_id)
);

CREATE INDEX idx_domains_workspace ON domains (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_domains_pending ON domains (next_check_at) WHERE status != 'verified' AND deleted_at IS NULL;
CREATE INDEX idx_domains_recheck ON domains (next_check_at) WHERE deleted_at IS NULL;
```

### 3.7. Template Types

```sql
CREATE TABLE template_types (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(100) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,

    -- NULL = global template type
    workspace_id    UUID REFERENCES workspaces(id),

    -- Adapter assigned to this template type.
    -- When sending, this adapter is used (not resolved by chain).
    -- NULL = no adapter assigned yet (send will fail with 422).
    adapter_id      UUID REFERENCES adapters(id),

    -- JSON Schema for event variables that this type expects
    -- Example: {"type": "object", "properties": {"user_name": {"type": "string"}, ...}, "required": ["user_name"]}
    variable_schema JSONB NOT NULL DEFAULT '{}',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    -- Slug unique within its scope
    CONSTRAINT template_types_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (slug, workspace_id)
);

CREATE INDEX idx_template_types_workspace ON template_types (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_template_types_global ON template_types (slug) WHERE workspace_id IS NULL AND deleted_at IS NULL;
```

### 3.8. Templates y Versiones

```sql
CREATE TABLE templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_type_id    UUID NOT NULL REFERENCES template_types(id),

    -- The scope where this template is defined. NULL = global.
    workspace_id        UUID REFERENCES workspaces(id),

    -- Kill switch
    is_disabled         BOOLEAN NOT NULL DEFAULT false,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,

    -- One template per type per scope (R-08 unicidad)
    CONSTRAINT templates_unique_type_per_scope
        UNIQUE NULLS NOT DISTINCT (template_type_id, workspace_id)
);

CREATE INDEX idx_templates_workspace ON templates (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_templates_type ON templates (template_type_id) WHERE deleted_at IS NULL;

CREATE TABLE template_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID NOT NULL REFERENCES templates(id),
    version_number  INT NOT NULL,
    status          version_status NOT NULL DEFAULT 'draft',

    -- Default content (used when no locale match)
    subject         TEXT NOT NULL DEFAULT '',
    preview_text    TEXT NOT NULL DEFAULT '',
    from_name       TEXT NOT NULL DEFAULT '',
    from_email      VARCHAR(255) NOT NULL DEFAULT '',
    reply_to        VARCHAR(255),
    body_mjml       TEXT NOT NULL DEFAULT '',

    -- Default locale of this version
    default_locale  VARCHAR(10) NOT NULL DEFAULT 'en',

    -- Editor internal representation (JSON for drag-and-drop editor)
    editor_data     JSONB,

    -- Metadata
    created_by      UUID REFERENCES members(id),
    published_at    TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Version number unique per template
    CONSTRAINT tv_version_unique UNIQUE (template_id, version_number),

    -- Only one published version per template
    CONSTRAINT tv_one_published
        EXCLUDE USING btree (template_id WITH =) WHERE (status = 'published')
);

CREATE INDEX idx_tv_template_status ON template_versions (template_id, status);
CREATE INDEX idx_tv_published ON template_versions (template_id) WHERE status = 'published';

-- Locale-specific content overrides for a version
CREATE TABLE template_version_locales (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_version_id     UUID NOT NULL REFERENCES template_versions(id) ON DELETE CASCADE,
    locale                  VARCHAR(10) NOT NULL,

    -- Override any of these; NULL means use the default from template_versions
    subject                 TEXT,
    preview_text            TEXT,
    from_name               TEXT,
    body_mjml               TEXT,
    editor_data             JSONB,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tvl_unique UNIQUE (template_version_id, locale)
);
```

### 3.9. Miembros y Roles

```sql
CREATE TABLE members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL,
    display_name    VARCHAR(255),

    -- OIDC subject claim (for matching tokens)
    oidc_subject    VARCHAR(255),
    oidc_issuer     VARCHAR(512),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT members_email_unique UNIQUE (email)
);

CREATE INDEX idx_members_email ON members (email);
CREATE INDEX idx_members_oidc ON members (oidc_issuer, oidc_subject) WHERE oidc_subject IS NOT NULL;

CREATE TABLE member_roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id   UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,

    role        member_role NOT NULL,

    -- Scope of the role
    -- superadmin: scope_type='global', tenant_id=NULL, workspace_id=NULL
    -- tenant_admin: scope_type='tenant', tenant_id=X, workspace_id=NULL
    -- workspace_*: scope_type='workspace', tenant_id=X (denorm), workspace_id=Y
    scope_type  scope_type NOT NULL,
    tenant_id   UUID REFERENCES tenants(id),
    workspace_id UUID REFERENCES workspaces(id),

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID REFERENCES members(id),

    -- Validate scope consistency
    CONSTRAINT mr_scope_check CHECK (
        (scope_type = 'global' AND tenant_id IS NULL AND workspace_id IS NULL AND role = 'superadmin')
        OR (scope_type = 'tenant' AND tenant_id IS NOT NULL AND workspace_id IS NULL AND role = 'tenant_admin')
        OR (scope_type = 'workspace' AND tenant_id IS NOT NULL AND workspace_id IS NOT NULL
            AND role IN ('workspace_admin', 'workspace_editor', 'workspace_viewer'))
    ),

    -- No duplicate role assignments
    CONSTRAINT mr_unique_role
        UNIQUE NULLS NOT DISTINCT (member_id, role, scope_type, tenant_id, workspace_id)
);

CREATE INDEX idx_member_roles_member ON member_roles (member_id);
CREATE INDEX idx_member_roles_tenant ON member_roles (tenant_id) WHERE scope_type = 'tenant';
CREATE INDEX idx_member_roles_workspace ON member_roles (workspace_id) WHERE scope_type = 'workspace';
```

### 3.10. API Keys

```sql
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),

    -- senda_live_<random> — only live keys (test access is via dashboard/OIDC)
    key_prefix      VARCHAR(20) NOT NULL DEFAULT 'senda_live',
    key_hash        VARCHAR(128) NOT NULL,  -- SHA-256 hex
    key_hint        VARCHAR(8) NOT NULL,    -- last 8 chars for identification

    name            VARCHAR(255),           -- optional label ("production", "staging")

    created_by      UUID NOT NULL REFERENCES members(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,

    CONSTRAINT api_keys_hash_unique UNIQUE (key_hash)
);

CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_keys_workspace ON api_keys (workspace_id) WHERE revoked_at IS NULL;
```

### 3.11. Emails y Lifecycle

```sql
CREATE TABLE emails (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Tracking
    tracking_id             VARCHAR(32) NOT NULL,
    external_id             VARCHAR(255),

    -- Context (denormalized for query efficiency)
    workspace_id            UUID NOT NULL REFERENCES workspaces(id),
    tenant_id               UUID NOT NULL REFERENCES tenants(id),

    -- Template snapshot (what was used at send time)
    template_id             UUID REFERENCES templates(id),
    template_version_id     UUID REFERENCES template_versions(id),
    template_type_slug      VARCHAR(100) NOT NULL,
    template_ref            VARCHAR(255) NOT NULL, -- "latam:acme:welcome" original ref

    -- Recipient
    recipient_email         VARCHAR(255) NOT NULL,
    cc                      JSONB,  -- ["email1@x.com", "email2@x.com"]
    bcc                     JSONB,

    -- Sender
    from_email              VARCHAR(255) NOT NULL,
    from_name               VARCHAR(255),
    reply_to                VARCHAR(255),

    -- Content (rendered at send time)
    subject_rendered        TEXT NOT NULL,
    locale                  VARCHAR(10),

    -- Status
    status                  email_status NOT NULL DEFAULT 'queued',

    -- Provider info
    adapter_id              UUID REFERENCES adapters(id),
    provider_message_id     VARCHAR(512),

    -- Snapshots of data used (for audit/debugging)
    variables_snapshot      JSONB,
    injectors_snapshot      JSONB,

    -- Retry tracking
    retry_count             INT NOT NULL DEFAULT 0,
    max_retries             INT NOT NULL DEFAULT 3,
    next_retry_at           TIMESTAMPTZ,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Primary lookups
CREATE UNIQUE INDEX idx_emails_tracking_id ON emails (tracking_id);
CREATE INDEX idx_emails_external_id ON emails (external_id) WHERE external_id IS NOT NULL;

-- Query by workspace (scoped API Key access)
CREATE INDEX idx_emails_workspace_created ON emails (workspace_id, created_at DESC);

-- Query by recipient/sender
CREATE INDEX idx_emails_recipient ON emails (recipient_email, created_at DESC);
CREATE INDEX idx_emails_from ON emails (from_email, created_at DESC);

-- Cross-tenant query for superadmin
CREATE INDEX idx_emails_tenant_created ON emails (tenant_id, created_at DESC);

-- Retry queue
CREATE INDEX idx_emails_retry ON emails (next_retry_at) WHERE status = 'queued' AND retry_count < max_retries;

-- Cursor-based pagination helper
CREATE INDEX idx_emails_workspace_cursor ON emails (workspace_id, id);

-- Email lifecycle events (append-only)
CREATE TABLE email_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_id    UUID NOT NULL REFERENCES emails(id),
    event_type  email_status NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Provider-specific metadata
    -- bounce: {bounce_type: "hard"|"soft", diagnostic_code: "...", bounce_recipient: "..."}
    -- complaint: {complaint_type: "abuse", feedback_id: "..."}
    -- delivered: {delivery_timestamp: "...", remote_mta: "..."}
    metadata    JSONB,

    CONSTRAINT email_events_unique UNIQUE (email_id, event_type, occurred_at)
);

CREATE INDEX idx_email_events_email ON email_events (email_id, occurred_at);
```

### 3.12. Suppression Lists

```sql
-- Global suppression: hard bounces. Blocks sending across ALL workspaces.
CREATE TABLE suppression_global (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    source_email_id UUID REFERENCES emails(id),
    notes           TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- If removed by superadmin
    removed_at      TIMESTAMPTZ,
    removed_by      UUID REFERENCES members(id),
    removal_reason  TEXT,

    CONSTRAINT suppression_global_email_unique
        UNIQUE (email) -- only active entry per email
);

CREATE INDEX idx_suppression_global_email ON suppression_global (email) WHERE removed_at IS NULL;

-- Workspace suppression: complaints. Only blocks within that workspace.
CREATE TABLE suppression_workspace (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    email           VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    source_email_id UUID REFERENCES emails(id),
    notes           TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    removed_at      TIMESTAMPTZ,
    removed_by      UUID REFERENCES members(id),
    removal_reason  TEXT,

    CONSTRAINT suppression_ws_unique UNIQUE (workspace_id, email)
);

CREATE INDEX idx_suppression_ws_lookup ON suppression_workspace (workspace_id, email) WHERE removed_at IS NULL;
```

### 3.13. Webhooks (Schema-ready for P1)

```sql
CREATE TABLE webhooks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    url             VARCHAR(2048) NOT NULL,

    -- HMAC-SHA256 secret for payload signing
    secret          VARCHAR(128) NOT NULL,

    -- Which events trigger this webhook
    events          JSONB NOT NULL DEFAULT '["sent","delivered","bounced","complained","failed"]',

    is_active       BOOLEAN NOT NULL DEFAULT true,

    -- Failure tracking
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_at  TIMESTAMPTZ,
    disabled_at      TIMESTAMPTZ,  -- auto-disabled after N consecutive failures

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhooks_workspace ON webhooks (workspace_id) WHERE is_active = true;
```

### 3.14. Audit Log

```sql
CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Who
    member_id       UUID NOT NULL REFERENCES members(id),
    member_email    VARCHAR(255) NOT NULL, -- denormalized (email could change)

    -- What
    action          audit_action NOT NULL,
    resource_type   VARCHAR(50) NOT NULL,  -- 'tenant', 'workspace', 'template', 'injector', etc.
    resource_id     UUID NOT NULL,

    -- Where
    scope_type      scope_type NOT NULL,
    tenant_id       UUID REFERENCES tenants(id),
    workspace_id    UUID REFERENCES workspaces(id),

    -- Details
    changes         JSONB, -- {"field": {"old": "...", "new": "..."}}
    metadata        JSONB, -- Additional context (IP, user agent, etc.)

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- No UPDATE or DELETE on this table (append-only enforced by app + optional RLS)
CREATE INDEX idx_audit_scope ON audit_logs (scope_type, tenant_id, workspace_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_logs (resource_type, resource_id, created_at DESC);
CREATE INDEX idx_audit_member ON audit_logs (member_id, created_at DESC);
CREATE INDEX idx_audit_created ON audit_logs (created_at DESC);
```

### 3.15. Global Config

```sql
CREATE TABLE global_config (
    key             VARCHAR(100) PRIMARY KEY,
    value           JSONB NOT NULL,
    description     TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      UUID REFERENCES members(id)
);

-- Seed data
INSERT INTO global_config (key, value, description) VALUES
    ('oidc.discovery_url', '"https://accounts.google.com/.well-known/openid-configuration"', 'OIDC provider discovery URL'),
    ('oidc.client_id', '""', 'OIDC client ID'),
    ('oidc.client_secret_encrypted', '""', 'OIDC client secret (encrypted)'),
    ('email.default_retry_count', '3', 'Default max retries for failed sends'),
    ('email.retry_backoff_base_seconds', '60', 'Base seconds for exponential backoff'),
    ('email.log_retention_days', '365', 'Days to retain email logs'),
    ('bounce.alert_threshold_percent', '5', 'Bounce rate % threshold for alerts (24h window)'),
    ('complaint.alert_threshold_percent', '0.1', 'Complaint rate % threshold for alerts'),
    ('domain.recheck_interval_hours', '24', 'Hours between domain re-verification checks'),
    ('onboarding.completed', 'false', 'Whether initial onboarding has been completed');
```

### 3.16. From Address Resolution (Helper View)

```sql
-- Helper: For a given workspace, get the resolution chain (workspace → _system → global)
-- Used by application code to resolve inherited resources
CREATE OR REPLACE FUNCTION get_resolution_chain(p_workspace_id UUID)
RETURNS TABLE(workspace_id UUID, priority INT) AS $$
    SELECT w.id, 1 AS priority
    FROM workspaces w WHERE w.id = p_workspace_id AND w.deleted_at IS NULL
    UNION ALL
    SELECT sys.id, 2 AS priority
    FROM workspaces w
    JOIN workspaces sys ON sys.tenant_id = w.tenant_id AND sys.is_system = true AND sys.deleted_at IS NULL
    WHERE w.id = p_workspace_id AND w.deleted_at IS NULL AND w.is_system = false
    UNION ALL
    SELECT NULL::UUID, 3 AS priority
$$ LANGUAGE sql STABLE;
```

---

## 4. Partitioning Strategy

For high-volume tables (`emails`, `email_events`), apply time-range partitioning:

```sql
-- Partition emails by month
CREATE TABLE emails (
    -- ... (same columns as above)
) PARTITION BY RANGE (created_at);

-- Create partitions (automated via pg_partman or cron)
CREATE TABLE emails_2026_01 PARTITION OF emails
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE emails_2026_02 PARTITION OF emails
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
-- ... etc.

-- Same for email_events
CREATE TABLE email_events (
    -- ... (same columns as above)
) PARTITION BY RANGE (occurred_at);
```

**Note:** In section 3.11, the schema is shown without partitioning for clarity. In the real implementation, `emails` and `email_events` will be partitioned. Indexes are created either globally or per partition depending on the query pattern.

---

## 5. Additional Performance Indexes

```sql
-- Composite index for the most common API query: send flow resolution
-- "Given tenant code + workspace code, find the workspace"
CREATE INDEX idx_workspace_resolution
    ON workspaces (tenant_id, code)
    INCLUDE (id, is_system)
    WHERE deleted_at IS NULL;

-- Template resolution: find published template for a type in a scope
CREATE INDEX idx_template_resolution
    ON templates (template_type_id, workspace_id)
    INCLUDE (id, is_disabled)
    WHERE deleted_at IS NULL;

-- Suppression check: combined check for send flow
-- App does: check suppression_global + suppression_workspace in parallel
-- Indexes already cover this (idx_suppression_global_email, idx_suppression_ws_lookup)
```

---

---

## 6. QA Results: Application-Level Validations Required

The schema was validated against the PRD's 48 main flows. **44 pass entirely at the DB level**, and 4 require additional validation in the application layer:

### VAL-01: Injector Field Ownership (High Priority)

**Problem:** `injector_fields` has an FK to `injector_definitions` but does not prevent a lower scope from adding fields to an injector it does not own.

**Rule:** Only the scope that created the injector (`injector_definitions.workspace_id`) may insert rows into `injector_fields` for that injector.

**Implementation:**
```go
func (s *InjectorService) AddField(ctx context.Context, injDefID uuid.UUID, field Field) error {
    injDef, err := s.repo.GetInjectorDefinition(ctx, injDefID)
    if err != nil { return err }

    currentScope := auth.GetWorkspaceID(ctx) // nil for global
    if injDef.WorkspaceID != currentScope {
        return apperr.Forbidden("only the owner scope can modify injector schema")
    }
    return s.repo.InsertInjectorField(ctx, injDefID, field)
}
```

**Required test:** Attempt to add a field from a lower scope → 422/403.

### VAL-02: Injector Name Uniqueness Across the Resolution Chain (High Priority)

**Problem:** A workspace could create an injector with the same name as one already existing in its `_system` or global scope. The DB constraint only enforces uniqueness within the same scope.

**Rule:** When creating an injector, verify that no other injector with the same name exists at any higher level in the chain.

**Implementation:**
```go
func (s *InjectorService) Create(ctx context.Context, name string, wsID *uuid.UUID) error {
    chain := s.resolver.GetChain(ctx, wsID)
    for _, scopeID := range chain {
        existing, _ := s.repo.FindInjectorByName(ctx, name, scopeID)
        if existing != nil {
            return apperr.Conflict("injector '%s' already defined at higher scope", name)
        }
    }
    return s.repo.CreateInjectorDefinition(ctx, name, wsID)
}
```

### VAL-03: Dependency Check Before Purge (Medium Priority)

**Problem:** There is no explicit FK dependency between inherited resources. Before purge, the system must compute which scopes would be affected.

**Implementation:** Runtime query — acceptable for Phase 1 given that the number of workspaces is typically < 100.

```go
func (s *ResourceService) GetDependents(ctx context.Context, resourceType string, resourceID uuid.UUID) ([]Workspace, error) {
    // Get all workspaces that would lose this resource if purged
    // (those that inherit it without their own override)
    chain := s.resolver.GetInheritingScopes(ctx, resourceType, resourceID)
    return chain, nil
}
```

**Phase 2:** Consider a materialized view to precompute dependencies.

### VAL-04: Template Type Uniqueness Across the Resolution Chain (Medium Priority)

**Problem similar to VAL-02:** A workspace should not be able to create a template type with the same slug as one that already exists in `_system` or global (because they would overwrite each other during resolution).

**Note:** Unlike injectors, it **is** valid here for a workspace to have a template (implementation) for a type defined above. What is not valid is creating a **new type** with the same slug.

**Implementation:** Same logic as VAL-02 but for `template_types`.

---

## 7. Schema Migration Strategy

### Tool: golang-migrate

Each migration is a `{N}_{name}.up.sql` / `{N}_{name}.down.sql` pair. The Senda binary runs migrations on startup if `database.migrate_on_start = true`.

```
migrations/
├── 000001_extensions.up.sql
├── 000001_extensions.down.sql
├── 000002_enums.up.sql
├── 000002_enums.down.sql
├── ...
└── 000019_cron_jobs.down.sql
```

### Principle: Section 3 Is the Source of Truth

The migrations in this section are the **exact implementation** of Section 3 (Full SQL Schema). Every constraint, CHECK, EXCLUDE, index, and column defined in S3 is reflected here. The only additions are:

- **UNLOGGED tables** (cache, token_buckets) — they do not exist in S3 because they are runtime infrastructure
- **PL/pgSQL functions** — `take_send_token()` is custom, `get_resolution_chain()` comes from S3.16
- **pg_cron jobs** — scheduled cleanup and partition creation
- **`rate_limit_per_second`** in adapters and **`adapter_id`** in template_types — architectural changes after S3

### Complete Migration Catalog

#### 000001 — Extensions

Installs the required extensions. Requires superuser permissions or `CREATE EXTENSION`.

```sql
-- UP
CREATE EXTENSION IF NOT EXISTS "pgcrypto";     -- gen_random_uuid(), encrypt/decrypt
CREATE EXTENSION IF NOT EXISTS "pg_cron";       -- scheduled cleanup jobs (cache expiry)

-- DOWN
DROP EXTENSION IF EXISTS "pg_cron";
DROP EXTENSION IF EXISTS "pgcrypto";
```

> **Note about pg_cron:** The extension must be listed in `shared_preload_libraries` in `postgresql.conf`. In Docker, it is configured with the `postgres:16-alpine` image plus `command: postgres -c shared_preload_libraries=pg_cron -c cron.database_name=senda`. The `docker-compose.yml` already includes it.

#### 000002 — Enums

Exact mirror of Section 3.1.

```sql
-- UP
CREATE TYPE email_status AS ENUM (
    'queued', 'processing', 'sent', 'delivered',
    'opened', 'bounced', 'complained', 'failed', 'suppressed'
);
CREATE TYPE version_status AS ENUM ('draft', 'published', 'archived');
CREATE TYPE adapter_type AS ENUM ('ses', 'gmail');
CREATE TYPE domain_status AS ENUM ('pending', 'verified', 'error');
CREATE TYPE member_role AS ENUM (
    'superadmin', 'tenant_admin',
    'workspace_admin', 'workspace_editor', 'workspace_viewer'
);
CREATE TYPE scope_type AS ENUM ('global', 'tenant', 'workspace');
CREATE TYPE injector_field_type AS ENUM ('text', 'number', 'bool', 'img', 'url', 'html');
CREATE TYPE bounce_type AS ENUM ('soft', 'hard');
CREATE TYPE suppression_reason AS ENUM ('hard_bounce', 'complaint', 'manual');
CREATE TYPE audit_action AS ENUM (
    'create', 'update', 'delete', 'purge', 'publish',
    'archive', 'disable', 'enable', 'revoke', 'invite', 'remove_role'
);

-- DOWN (orden inverso)
DROP TYPE IF EXISTS audit_action;
DROP TYPE IF EXISTS suppression_reason;
DROP TYPE IF EXISTS bounce_type;
DROP TYPE IF EXISTS injector_field_type;
DROP TYPE IF EXISTS scope_type;
DROP TYPE IF EXISTS member_role;
DROP TYPE IF EXISTS domain_status;
DROP TYPE IF EXISTS adapter_type;
DROP TYPE IF EXISTS version_status;
DROP TYPE IF EXISTS email_status;
```

#### 000003 — Tenants y Workspaces

Mirror of Sections 3.2 and 3.3.

```sql
-- UP
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT tenants_code_unique UNIQUE (code),
    CONSTRAINT tenants_code_format CHECK (
        code ~ '^[a-z][a-z0-9-]*$'
        AND length(code) BETWEEN 2 AND 50
        AND code NOT LIKE '%-'
        AND code NOT LIKE '%---%'
        AND code NOT IN ('_system', 'global', 'admin', 'api', 'system')
    )
);

CREATE INDEX idx_tenants_code ON tenants (code) WHERE deleted_at IS NULL;

CREATE TABLE workspaces (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    code        VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    is_system   BOOLEAN NOT NULL DEFAULT false,
    open_tracking_enabled BOOLEAN NOT NULL DEFAULT false,
    default_locale VARCHAR(10),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT workspaces_code_per_tenant UNIQUE (tenant_id, code),
    CONSTRAINT workspaces_code_format CHECK (
        code = '_system' OR (
            code ~ '^[a-z][a-z0-9-]*$'
            AND length(code) BETWEEN 2 AND 50
            AND code NOT LIKE '%-'
            AND code NOT LIKE '%---%'
            AND code NOT IN ('global', 'admin', 'api', 'system')
        )
    ),
    CONSTRAINT workspaces_one_system_per_tenant
        EXCLUDE USING btree (tenant_id WITH =) WHERE (is_system = true AND deleted_at IS NULL)
);

CREATE INDEX idx_workspaces_tenant ON workspaces (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_workspaces_code ON workspaces (tenant_id, code) WHERE deleted_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS tenants;
```

#### 000004 — Injectors (definitions, fields, values)

Mirror of Section 3.4.

```sql
-- UP
CREATE TABLE injector_definitions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    workspace_id    UUID REFERENCES workspaces(id),
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT injector_def_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (name, workspace_id)
);

CREATE INDEX idx_injector_defs_workspace ON injector_definitions (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_injector_defs_global ON injector_definitions (name) WHERE workspace_id IS NULL AND deleted_at IS NULL;

CREATE TABLE injector_fields (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    injector_definition_id  UUID NOT NULL REFERENCES injector_definitions(id) ON DELETE CASCADE,
    field_name              VARCHAR(100) NOT NULL,
    field_type              injector_field_type NOT NULL,
    description             TEXT,
    position                INT NOT NULL DEFAULT 0,

    CONSTRAINT injector_field_unique
        UNIQUE (injector_definition_id, field_name)
);

CREATE TABLE injector_values (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    injector_definition_id  UUID NOT NULL REFERENCES injector_definitions(id),
    field_name              VARCHAR(100) NOT NULL,
    workspace_id            UUID REFERENCES workspaces(id),
    value                   JSONB NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT injector_value_unique
        UNIQUE NULLS NOT DISTINCT (injector_definition_id, field_name, workspace_id)
);

CREATE INDEX idx_injector_values_lookup
    ON injector_values (injector_definition_id, workspace_id);

-- DOWN
DROP TABLE IF EXISTS injector_values;
DROP TABLE IF EXISTS injector_fields;
DROP TABLE IF EXISTS injector_definitions;
```

#### 000005 — Adapters

Mirror of Section 3.5 + `rate_limit_per_second` (P1 architectural change).

```sql
-- UP
CREATE TABLE adapters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    workspace_id    UUID REFERENCES workspaces(id),
    adapter_type    adapter_type NOT NULL,
    config_encrypted BYTEA NOT NULL,
    is_default      BOOLEAN NOT NULL DEFAULT false,
    rate_limit_per_second INT NOT NULL DEFAULT 14,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT adapters_one_default_per_scope
        EXCLUDE USING btree (workspace_id WITH =) WHERE (is_default = true AND deleted_at IS NULL)
);

CREATE UNIQUE INDEX idx_adapters_global_default
    ON adapters ((true)) WHERE workspace_id IS NULL AND is_default = true AND deleted_at IS NULL;

CREATE INDEX idx_adapters_workspace ON adapters (workspace_id) WHERE deleted_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS adapters;
```

#### 000006 — Domains

Mirror of Section 3.6.

```sql
-- UP
CREATE TABLE domains (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            UUID REFERENCES workspaces(id),
    domain_name             VARCHAR(255) NOT NULL,
    status                  domain_status NOT NULL DEFAULT 'pending',
    dkim_selector           VARCHAR(63) NOT NULL,
    dkim_private_key_encrypted BYTEA NOT NULL,
    dkim_public_key         TEXT NOT NULL,
    dns_records             JSONB NOT NULL DEFAULT '[]',
    verified_at             TIMESTAMPTZ,
    last_check_at           TIMESTAMPTZ,
    next_check_at           TIMESTAMPTZ,
    last_error              TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ,

    CONSTRAINT domains_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (domain_name, workspace_id)
);

CREATE INDEX idx_domains_workspace ON domains (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_domains_pending ON domains (next_check_at) WHERE status != 'verified' AND deleted_at IS NULL;
CREATE INDEX idx_domains_recheck ON domains (next_check_at) WHERE deleted_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS domains;
```

#### 000007 — Template Types y Templates

Mirror of Sections 3.7 + 3.8 (templates). `adapter_id` is a P1 architectural change.

```sql
-- UP
CREATE TABLE template_types (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(100) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    workspace_id    UUID REFERENCES workspaces(id),
    adapter_id      UUID REFERENCES adapters(id),
    variable_schema JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT template_types_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (slug, workspace_id)
);

CREATE INDEX idx_template_types_workspace ON template_types (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_template_types_global ON template_types (slug) WHERE workspace_id IS NULL AND deleted_at IS NULL;
CREATE INDEX idx_template_types_adapter ON template_types (adapter_id);

CREATE TABLE templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_type_id    UUID NOT NULL REFERENCES template_types(id),
    workspace_id        UUID REFERENCES workspaces(id),
    is_disabled         BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,

    CONSTRAINT templates_unique_type_per_scope
        UNIQUE NULLS NOT DISTINCT (template_type_id, workspace_id)
);

CREATE INDEX idx_templates_workspace ON templates (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_templates_type ON templates (template_type_id) WHERE deleted_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS template_types;
```

#### 000008 — Template Versions y Locales

Mirror of Section 3.8 (versions and locales).

```sql
-- UP
CREATE TABLE template_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID NOT NULL REFERENCES templates(id),
    version_number  INT NOT NULL,
    status          version_status NOT NULL DEFAULT 'draft',
    subject         TEXT NOT NULL DEFAULT '',
    preview_text    TEXT NOT NULL DEFAULT '',
    from_name       TEXT NOT NULL DEFAULT '',
    from_email      VARCHAR(255) NOT NULL DEFAULT '',
    reply_to        VARCHAR(255),
    body_mjml       TEXT NOT NULL DEFAULT '',
    default_locale  VARCHAR(10) NOT NULL DEFAULT 'en',
    editor_data     JSONB,
    created_by      UUID REFERENCES members(id),
    published_at    TIMESTAMPTZ,
    archived_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tv_version_unique UNIQUE (template_id, version_number),
    CONSTRAINT tv_one_published
        EXCLUDE USING btree (template_id WITH =) WHERE (status = 'published')
);

CREATE INDEX idx_tv_template_status ON template_versions (template_id, status);
CREATE INDEX idx_tv_published ON template_versions (template_id) WHERE status = 'published';

CREATE TABLE template_version_locales (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_version_id     UUID NOT NULL REFERENCES template_versions(id) ON DELETE CASCADE,
    locale                  VARCHAR(10) NOT NULL,
    subject                 TEXT,
    preview_text            TEXT,
    from_name               TEXT,
    body_mjml               TEXT,
    editor_data             JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tvl_unique UNIQUE (template_version_id, locale)
);

-- DOWN
DROP TABLE IF EXISTS template_version_locales;
DROP TABLE IF EXISTS template_versions;
```

> **Note:** `template_versions` has an FK to `members(id)` via `created_by`. This migration must run after 000009 (members). golang-migrate handles dependencies by numeric order, so in production it can be reordered if necessary, or `created_by` can be made without an FK and the constraint added in a later migration.

#### 000009 — Members y Roles

Mirror of Section 3.9.

```sql
-- UP
CREATE TABLE members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL,
    display_name    VARCHAR(255),
    oidc_subject    VARCHAR(255),
    oidc_issuer     VARCHAR(512),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT members_email_unique UNIQUE (email)
);

CREATE INDEX idx_members_email ON members (email);
CREATE INDEX idx_members_oidc ON members (oidc_issuer, oidc_subject) WHERE oidc_subject IS NOT NULL;

CREATE TABLE member_roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id   UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    role        member_role NOT NULL,
    scope_type  scope_type NOT NULL,
    tenant_id   UUID REFERENCES tenants(id),
    workspace_id UUID REFERENCES workspaces(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID REFERENCES members(id),

    CONSTRAINT mr_scope_check CHECK (
        (scope_type = 'global' AND tenant_id IS NULL AND workspace_id IS NULL AND role = 'superadmin')
        OR (scope_type = 'tenant' AND tenant_id IS NOT NULL AND workspace_id IS NULL AND role = 'tenant_admin')
        OR (scope_type = 'workspace' AND tenant_id IS NOT NULL AND workspace_id IS NOT NULL
            AND role IN ('workspace_admin', 'workspace_editor', 'workspace_viewer'))
    ),

    CONSTRAINT mr_unique_role
        UNIQUE NULLS NOT DISTINCT (member_id, role, scope_type, tenant_id, workspace_id)
);

CREATE INDEX idx_member_roles_member ON member_roles (member_id);
CREATE INDEX idx_member_roles_tenant ON member_roles (tenant_id) WHERE scope_type = 'tenant';
CREATE INDEX idx_member_roles_workspace ON member_roles (workspace_id) WHERE scope_type = 'workspace';

-- DOWN
DROP TABLE IF EXISTS member_roles;
DROP TABLE IF EXISTS members;
```

#### 000010 — API Keys

Mirror of Section 3.10.

```sql
-- UP
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    key_prefix      VARCHAR(20) NOT NULL DEFAULT 'senda_live',
    key_hash        VARCHAR(128) NOT NULL,
    key_hint        VARCHAR(8) NOT NULL,
    name            VARCHAR(255),
    created_by      UUID NOT NULL REFERENCES members(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,

    CONSTRAINT api_keys_hash_unique UNIQUE (key_hash)
);

CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_keys_workspace ON api_keys (workspace_id) WHERE revoked_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS api_keys;
```

#### 000011 — Emails y Events

Mirror of Section 3.11 + partitioning from Section 4.

```sql
-- UP
CREATE TABLE emails (
    id                      UUID NOT NULL DEFAULT gen_random_uuid(),
    tracking_id             VARCHAR(32) NOT NULL,
    external_id             VARCHAR(255),
    workspace_id            UUID NOT NULL,
    tenant_id               UUID NOT NULL,
    template_id             UUID,
    template_version_id     UUID,
    template_type_slug      VARCHAR(100) NOT NULL,
    template_ref            VARCHAR(255) NOT NULL,
    recipient_email         VARCHAR(255) NOT NULL,
    cc                      JSONB,
    bcc                     JSONB,
    from_email              VARCHAR(255) NOT NULL,
    from_name               VARCHAR(255),
    reply_to                VARCHAR(255),
    subject_rendered        TEXT NOT NULL,
    body_mjml               TEXT,
    locale                  VARCHAR(10),
    status                  email_status NOT NULL DEFAULT 'queued',
    adapter_id              UUID,
    provider_message_id     VARCHAR(512),
    variables_snapshot      JSONB,
    injectors_snapshot      JSONB,
    retry_count             INT NOT NULL DEFAULT 0,
    max_retries             INT NOT NULL DEFAULT 3,
    next_retry_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Initial partitions (3 months; pg_cron creates the following ones)
CREATE TABLE emails_2026_01 PARTITION OF emails
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE emails_2026_02 PARTITION OF emails
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE emails_2026_03 PARTITION OF emails
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE UNIQUE INDEX idx_emails_tracking_id ON emails (tracking_id, created_at);
CREATE INDEX idx_emails_external_id ON emails (external_id, created_at) WHERE external_id IS NOT NULL;
CREATE INDEX idx_emails_workspace_created ON emails (workspace_id, created_at DESC);
CREATE INDEX idx_emails_recipient ON emails (recipient_email, created_at DESC);
CREATE INDEX idx_emails_from ON emails (from_email, created_at DESC);
CREATE INDEX idx_emails_tenant_created ON emails (tenant_id, created_at DESC);
CREATE INDEX idx_emails_retry ON emails (next_retry_at, created_at) WHERE status = 'queued';
CREATE INDEX idx_emails_workspace_cursor ON emails (workspace_id, id, created_at);

-- Email lifecycle events (append-only)
CREATE TABLE email_events (
    id          UUID NOT NULL DEFAULT gen_random_uuid(),
    email_id    UUID NOT NULL,
    event_type  email_status NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at),
    CONSTRAINT email_events_unique UNIQUE (email_id, event_type, occurred_at, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE email_events_2026_01 PARTITION OF email_events
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE email_events_2026_02 PARTITION OF email_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE email_events_2026_03 PARTITION OF email_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE INDEX idx_email_events_email ON email_events (email_id, occurred_at);

-- DOWN
DROP TABLE IF EXISTS email_events CASCADE;
DROP TABLE IF EXISTS emails CASCADE;
```

#### 000012 — Suppression Lists

Mirror of Section 3.12.

```sql
-- UP
CREATE TABLE suppression_global (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    source_email_id UUID,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at      TIMESTAMPTZ,
    removed_by      UUID,
    removal_reason  TEXT,

    CONSTRAINT suppression_global_email_unique UNIQUE (email)
);

CREATE INDEX idx_suppression_global_email ON suppression_global (email) WHERE removed_at IS NULL;

CREATE TABLE suppression_workspace (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    email           VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    source_email_id UUID,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at      TIMESTAMPTZ,
    removed_by      UUID,
    removal_reason  TEXT,

    CONSTRAINT suppression_ws_unique UNIQUE (workspace_id, email)
);

CREATE INDEX idx_suppression_ws_lookup ON suppression_workspace (workspace_id, email) WHERE removed_at IS NULL;

-- DOWN
DROP TABLE IF EXISTS suppression_workspace;
DROP TABLE IF EXISTS suppression_global;
```

> **Note:** The FKs to `emails(id)` and `members(id)` in suppression are defined without an explicit constraint because `emails` is partitioned (the PK is composite). Validation happens at the application level.

#### 000013 — Webhooks

Mirror of Section 3.13.

```sql
-- UP
CREATE TABLE webhooks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspaces(id),
    url                 VARCHAR(2048) NOT NULL,
    secret              VARCHAR(128) NOT NULL,
    events              JSONB NOT NULL DEFAULT '["sent","delivered","bounced","complained","failed"]',
    is_active           BOOLEAN NOT NULL DEFAULT true,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_at     TIMESTAMPTZ,
    disabled_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhooks_workspace ON webhooks (workspace_id) WHERE is_active = true;

-- DOWN
DROP TABLE IF EXISTS webhooks;
```

#### 000014 — Audit Logs

Mirror of Section 3.14 + partitioning.

```sql
-- UP
CREATE TABLE audit_logs (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    member_id       UUID NOT NULL,
    member_email    VARCHAR(255) NOT NULL,
    action          audit_action NOT NULL,
    resource_type   VARCHAR(50) NOT NULL,
    resource_id     UUID NOT NULL,
    scope_type      scope_type NOT NULL,
    tenant_id       UUID,
    workspace_id    UUID,
    changes         JSONB,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_logs_2026_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE INDEX idx_audit_scope ON audit_logs (scope_type, tenant_id, workspace_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_logs (resource_type, resource_id, created_at DESC);
CREATE INDEX idx_audit_member ON audit_logs (member_id, created_at DESC);
CREATE INDEX idx_audit_created ON audit_logs (created_at DESC);

-- DOWN
DROP TABLE IF EXISTS audit_logs CASCADE;
```

#### 000015 — Global Config with Seed Data

Mirror of Section 3.15.

```sql
-- UP
CREATE TABLE global_config (
    key             VARCHAR(100) PRIMARY KEY,
    value           JSONB NOT NULL,
    description     TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      UUID
);

INSERT INTO global_config (key, value, description) VALUES
    ('oidc.discovery_url', '"https://accounts.google.com/.well-known/openid-configuration"', 'OIDC provider discovery URL'),
    ('oidc.client_id', '""', 'OIDC client ID'),
    ('oidc.client_secret_encrypted', '""', 'OIDC client secret (encrypted)'),
    ('email.default_retry_count', '3', 'Default max retries for failed sends'),
    ('email.retry_backoff_base_seconds', '60', 'Base seconds for exponential backoff'),
    ('email.log_retention_days', '365', 'Days to retain email logs'),
    ('bounce.alert_threshold_percent', '5', 'Bounce rate % threshold for alerts (24h window)'),
    ('complaint.alert_threshold_percent', '0.1', 'Complaint rate % threshold for alerts'),
    ('domain.recheck_interval_hours', '24', 'Hours between domain re-verification checks'),
    ('onboarding.completed', 'false', 'Whether initial onboarding has been completed');

-- DOWN
DROP TABLE IF EXISTS global_config;
```

#### 000016 — UNLOGGED Tables (Cache + Token Buckets)

Tables without WAL for performance. They are truncated automatically if PG crashes (acceptable for cache and rate limiting).

```sql
-- UP
CREATE UNLOGGED TABLE cache (
    key         VARCHAR(512) PRIMARY KEY,
    value       JSONB NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cache_expires ON cache(expires_at);

CREATE UNLOGGED TABLE token_buckets (
    adapter_id  UUID PRIMARY KEY REFERENCES adapters(id),
    tokens      FLOAT NOT NULL,
    max_tokens  INT NOT NULL,
    refill_rate FLOAT NOT NULL,
    last_refill TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- DOWN
DROP TABLE IF EXISTS token_buckets;
DROP TABLE IF EXISTS cache;
```

#### 000017 — PL/pgSQL Functions

```sql
-- UP

-- Resolution chain (Section 3.16): returns the workspace → _system → global chain
CREATE OR REPLACE FUNCTION get_resolution_chain(p_workspace_id UUID)
RETURNS TABLE(workspace_id UUID, priority INT) AS $$
    SELECT w.id, 1 AS priority
    FROM workspaces w WHERE w.id = p_workspace_id AND w.deleted_at IS NULL
    UNION ALL
    SELECT sys.id, 2 AS priority
    FROM workspaces w
    JOIN workspaces sys ON sys.tenant_id = w.tenant_id AND sys.is_system = true AND sys.deleted_at IS NULL
    WHERE w.id = p_workspace_id AND w.deleted_at IS NULL AND w.is_system = false
    UNION ALL
    SELECT NULL::UUID, 3 AS priority
$$ LANGUAGE sql STABLE;

-- Token bucket: atomic take-one-token (custom PL/pgSQL, no external dependencies)
CREATE OR REPLACE FUNCTION take_send_token(p_adapter_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    v_row token_buckets%ROWTYPE;
    v_now TIMESTAMPTZ := now();
    v_elapsed FLOAT;
    v_new_tokens FLOAT;
BEGIN
    SELECT * INTO v_row
    FROM token_buckets
    WHERE adapter_id = p_adapter_id
    FOR UPDATE;

    IF NOT FOUND THEN
        INSERT INTO token_buckets (adapter_id, tokens, max_tokens, refill_rate)
        SELECT id, rate_limit_per_second, rate_limit_per_second, rate_limit_per_second
        FROM adapters WHERE id = p_adapter_id
        ON CONFLICT (adapter_id) DO NOTHING;

        SELECT * INTO v_row
        FROM token_buckets
        WHERE adapter_id = p_adapter_id
        FOR UPDATE;
    END IF;

    v_elapsed := EXTRACT(EPOCH FROM (v_now - v_row.last_refill));
    v_new_tokens := LEAST(v_row.max_tokens, v_row.tokens + (v_elapsed * v_row.refill_rate));

    IF v_new_tokens < 1 THEN
        UPDATE token_buckets
        SET tokens = v_new_tokens, last_refill = v_now
        WHERE adapter_id = p_adapter_id;
        RETURN FALSE;
    END IF;

    UPDATE token_buckets
    SET tokens = v_new_tokens - 1, last_refill = v_now
    WHERE adapter_id = p_adapter_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- DOWN
DROP FUNCTION IF EXISTS take_send_token(UUID);
DROP FUNCTION IF EXISTS get_resolution_chain(UUID);
```

#### 000018 — Performance Indices (Section 5)

```sql
-- UP
CREATE INDEX idx_workspace_resolution
    ON workspaces (tenant_id, code)
    INCLUDE (id, is_system)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_template_resolution
    ON templates (template_type_id, workspace_id)
    INCLUDE (id, is_disabled)
    WHERE deleted_at IS NULL;

-- DOWN
DROP INDEX IF EXISTS idx_template_resolution;
DROP INDEX IF EXISTS idx_workspace_resolution;
```

#### 000019 — pg_cron Scheduled Jobs

```sql
-- UP

-- Cache cleanup: purge expired entries every 60 seconds
SELECT cron.schedule('cache-cleanup', '*/1 * * * *',
    $$DELETE FROM cache WHERE expires_at < now()$$
);

-- Partition maintenance: create next month's partitions (runs 1st of each month)
SELECT cron.schedule('create-partitions', '0 0 1 * *',
    $$
    DO $body$
    DECLARE
        v_start DATE := date_trunc('month', now() + interval '1 month');
        v_end DATE := v_start + interval '1 month';
        v_suffix TEXT := to_char(v_start, 'YYYY_MM');
    BEGIN
        EXECUTE format('CREATE TABLE IF NOT EXISTS emails_%s PARTITION OF emails FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
        EXECUTE format('CREATE TABLE IF NOT EXISTS email_events_%s PARTITION OF email_events FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
        EXECUTE format('CREATE TABLE IF NOT EXISTS audit_logs_%s PARTITION OF audit_logs FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
    END
    $body$;
    $$
);

-- DOWN
SELECT cron.unschedule('create-partitions');
SELECT cron.unschedule('cache-cleanup');
```

### Migration Summary

| # | Name | Source | Content |
|---|--------|--------|-----------|
| 001 | extensions | New | pgcrypto, pg_cron |
| 002 | enums | S3.1 | 10 enumerated types |
| 003 | tenants_workspaces | S3.2, S3.3 | With CHECKs, EXCLUDE, partial indexes |
| 004 | injectors | S3.4 | definitions, fields (typed), values (per field) |
| 005 | adapters | S3.5 + P1 | With default EXCLUDE, + rate_limit_per_second |
| 006 | domains | S3.6 | With domain_status, DKIM, dns_records, verification |
| 007 | template_types_templates | S3.7, S3.8 | With adapter_id FK (P1), UNIQUE NULLS NOT DISTINCT |
| 008 | versions_locales | S3.8 | With full content, EXCLUDE one-published |
| 009 | members_roles | S3.9 | With OIDC, scope_type, CHECK constraints |
| 010 | api_keys | S3.10 | With key_prefix, key_hint, revoked_at |
| 011 | emails_events | S3.11, S4 | Partitioned, tracking_id, full columns |
| 012 | suppressions | S3.12 | With source_email_id, removed_by, removal_reason |
| 013 | webhooks | S3.13 | With failure tracking, disabled_at |
| 014 | audit_logs | S3.14, S4 | With audit_action enum, scope_type, partitioned |
| 015 | global_config | S3.15 | With description, updated_by, complete seed data |
| 016 | unlogged_tables | New | cache (UNLOGGED), token_buckets (UNLOGGED) |
| 017 | plpgsql_functions | S3.16 + P1 | get_resolution_chain(TABLE), take_send_token() |
| 018 | performance_indices | S5 | workspace_resolution, template_resolution (INCLUDE) |
| 019 | cron_jobs | New | cache-cleanup (1min), create-partitions (monthly) |

> **Total: 19 migrations.** Each one is idempotent and reversible with its `down.sql`. The "Source" column indicates the spec section it originates from.

> **Ordering note:** Migration 000008 (template_versions) has an FK to `members(id)` via `created_by`, but members is created in 000009. In implementation, one can: (a) reorder 009 before 008, (b) create the FK as `DEFERRABLE`, or (c) add the FK constraint in a later migration (000020). This is decided during implementation.

---

---

# PART II: Application Architecture

---

## 8. General Architecture

### 8.1. Pattern: Hexagonal (Ports & Adapters)

Senda follows a hexagonal architecture with three clear layers:

```
┌─────────────────────────────────────────────────────────────────┐
│                        DRIVING ADAPTERS                         │
│  (HTTP handlers, CLI commands, background workers)              │
│                                                                 │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │ Echo v5 HTTP │  │ River Worker │  │ Cron (domain check)│     │
│  │ Handlers     │  │ (job queue)  │  │                    │     │
│  └──────┬───────┘  └──────┬───────┘  └─────────┬──────────┘     │
│         │                 │                    │                │
├─────────▼─────────────────▼────────────────────▼────────────────┤
│                       DOMAIN / CORE                             │
│  (Business logic, services, domain models, resolution engine)   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Services                              │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │    │
│  │  │ SendService   │  │ TemplateServ │  │ InjectorServ │  │    │
│  │  │ MemberService │  │ TenantServ   │  │ DomainServ   │  │    │
│  │  │ AuditService  │  │ TrackingServ │  │ SuppressionS │  │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  │    │
│  │                                                         │    │
│  │  ┌─────────────────────────────────────────────┐        │    │
│  │  │         Resolution Engine                    │        │    │
│  │  │  ChainResolver · InjectorMerger              │        │    │
│  │  │  TemplateResolver · AdapterResolver          │        │    │
│  │  │  DomainResolver                              │        │    │
│  │  └─────────────────────────────────────────────┘        │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│                     PORTS (interfaces)                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │ EmailPort │  │ StorePort│  │ QueuePort│  │ CachePort│       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                      DRIVEN ADAPTERS                            │
│  (Implementations of ports — infrastructure)                    │
│                                                                 │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────┐  ┌──────────┐ │
│  │ SESAdapter   │  │ PostgresStore│  │ River   │  │ PGCache  │ │
│  │ GmailAdapter │  │              │  │ Queue   │  │(UNLOGGED)│ │
│  └─────────────┘  └──────────────┘  └─────────┘  └──────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Dependency rule:** Arrows point inward. The core never imports adapter packages. Adapters implement interfaces defined in the core.

### 8.2. Dependency Injection

No DI frameworks. Manual composition in `main.go` via constructor injection:

```go
func main() {
    // Config
    cfg := config.Load()

    // Infrastructure (driven adapters)
    db := postgres.Connect(cfg.Database)
    cache := pgcache.New(pool) // PG UNLOGGED table cache
    queue := river.New(db)

    // Repositories (implement store ports)
    tenantRepo := postgres.NewTenantRepo(db)
    workspaceRepo := postgres.NewWorkspaceRepo(db)
    injectorRepo := postgres.NewInjectorRepo(db)
    templateRepo := postgres.NewTemplateRepo(db)
    memberRepo := postgres.NewMemberRepo(db)
    emailRepo := postgres.NewEmailRepo(db)
    // ... etc

    // Resolution engine
    chainResolver := resolution.NewChainResolver(workspaceRepo)
    injectorMerger := resolution.NewInjectorMerger(injectorRepo, chainResolver)
    templateResolver := resolution.NewTemplateResolver(templateRepo, chainResolver)
    adapterResolver := resolution.NewAdapterResolver(adapterRepo, chainResolver)
    domainResolver := resolution.NewDomainResolver(domainRepo, chainResolver)

    // Email adapters (implement email port)
    sesAdapter := ses.NewAdapter(cfg.SES)
    // gmailAdapter := gmail.NewAdapter(cfg.Gmail) // Phase 4

    adapterRegistry := email.NewAdapterRegistry()
    adapterRegistry.Register("ses", sesAdapter)

    // Services (domain/core)
    sendService := service.NewSendService(
        templateResolver, injectorMerger, adapterResolver,
        domainResolver, emailRepo, queue, adapterRegistry,
    )
    tenantService := service.NewTenantService(tenantRepo, workspaceRepo)
    templateService := service.NewTemplateService(templateRepo, templateTypeRepo, chainResolver)
    memberService := service.NewMemberService(memberRepo, cfg.OIDC)
    auditService := service.NewAuditService(auditRepo)
    // ... etc

    // HTTP server (driving adapter)
    server := http.NewServer(cfg.Server, sendService, tenantService, ...)
    server.Start()
}
```

---

## 9. Folder Structure

```
senda/
├── cmd/
│   └── senda/
│       └── main.go                    # Entry point, DI composition
│
├── internal/
│   ├── domain/                        # Domain models (entities, value objects)
│   │   ├── tenant.go                  # Tenant, Workspace entities
│   │   ├── injector.go                # InjectorDefinition, InjectorField, InjectorValue
│   │   ├── adapter.go                 # AdapterConfig entity
│   │   ├── template.go               # TemplateType, Template, TemplateVersion
│   │   ├── email.go                   # Email, EmailEvent, EmailStatus
│   │   ├── member.go                  # Member, MemberRole, Role
│   │   ├── apikey.go                  # APIKey
│   │   ├── domain_record.go          # Domain (verified domain, not DDD domain)
│   │   ├── suppression.go            # SuppressionEntry
│   │   ├── webhook.go                # Webhook
│   │   ├── audit.go                  # AuditLog
│   │   └── errors.go                 # Domain errors (ErrNotFound, ErrConflict, etc.)
│   │
│   ├── port/                          # Port interfaces (contracts)
│   │   ├── email_sender.go           # EmailSender port
│   │   ├── store.go                  # Repository interfaces (TenantStore, etc.)
│   │   ├── queue.go                  # JobQueue port
│   │   ├── cache.go                  # Cache port
│   │   ├── template_compiler.go      # TemplateCompiler port
│   │   └── crypto.go                 # Encryption port
│   │
│   ├── service/                       # Application services (use cases)
│   │   ├── send.go                   # SendService — orchestrates email sending
│   │   ├── tenant.go                 # TenantService — CRUD tenants/workspaces
│   │   ├── injector.go               # InjectorService — CRUD + validation
│   │   ├── template.go              # TemplateService — CRUD + versioning
│   │   ├── template_type.go         # TemplateTypeService
│   │   ├── member.go                # MemberService — OIDC + roles
│   │   ├── apikey.go                # APIKeyService
│   │   ├── domain.go                # DomainService — verification flow
│   │   ├── suppression.go           # SuppressionService
│   │   ├── tracking.go              # TrackingService — lifecycle + queries
│   │   ├── audit.go                 # AuditService — append-only logging
│   │   └── webhook.go               # WebhookService
│   │
│   ├── resolution/                    # Resolution engine
│   │   ├── chain.go                  # ChainResolver — workspace → _system → global
│   │   ├── injector_merger.go        # Field-by-field merge
│   │   ├── template_resolver.go     # Template + version resolution
│   │   ├── adapter_resolver.go      # Default adapter in chain
│   │   └── domain_resolver.go       # Domain verification check
│   │
│   ├── adapter/                       # Driven adapters (infrastructure)
│   │   ├── postgres/                 # PostgreSQL implementations
│   │   │   ├── tenant_repo.go
│   │   │   ├── workspace_repo.go
│   │   │   ├── injector_repo.go
│   │   │   ├── template_repo.go
│   │   │   ├── email_repo.go
│   │   │   ├── member_repo.go
│   │   │   ├── apikey_repo.go
│   │   │   ├── domain_repo.go
│   │   │   ├── suppression_repo.go
│   │   │   ├── audit_repo.go
│   │   │   ├── webhook_repo.go
│   │   │   ├── global_config_repo.go
│   │   │   └── db.go                # Connection, pool, migrations
│   │   │
│   │   ├── ses/                      # SES email sender
│   │   │   └── adapter.go
│   │   │
│   │   ├── gmail/                    # Gmail email sender (Phase 4)
│   │   │   └── adapter.go
│   │   │
│   │   ├── river/                    # River job queue
│   │   │   ├── client.go
│   │   │   ├── send_worker.go       # Email send job
│   │   │   ├── verify_worker.go     # Domain verification job
│   │   │   └── webhook_worker.go    # Webhook delivery job
│   │   │
│   │   ├── pgcache/                  # PG UNLOGGED cache implementation
│   │   │   └── client.go
│   │   │
│   │   ├── mjml/                     # MJML compiler (gomjml wrapper)
│   │   │   └── compiler.go
│   │   │
│   │   ├── dkim/                     # DKIM signer (go-msgauth wrapper)
│   │   │   └── signer.go
│   │   │
│   │   └── crypto/                   # AES-256-GCM encryption
│   │       └── aes.go
│   │
│   └── http/                          # Driving adapter: HTTP API
│       ├── server.go                 # Echo setup, middleware, graceful shutdown
│       ├── middleware/
│       │   ├── auth.go               # OIDC token validation + membership check
│       │   ├── apikey.go             # API Key authentication
│       │   ├── rbac.go               # Role-based access control
│       │   ├── scope.go              # Extract tenant/workspace from URL
│       │   ├── ratelimit.go          # Per-workspace rate limiting
│       │   ├── requestid.go          # X-Request-ID
│       │   ├── logger.go             # Structured logging
│       │   └── recovery.go          # Panic recovery
│       │
│       ├── handler/
│       │   ├── send.go               # POST /api/v1/send
│       │   ├── tenant.go            # CRUD /api/v1/tenants
│       │   ├── workspace.go         # CRUD /api/v1/tenants/:code/workspaces
│       │   ├── injector.go          # CRUD injectors
│       │   ├── template_type.go     # CRUD template types
│       │   ├── template.go          # CRUD templates + versions
│       │   ├── member.go            # CRUD members + roles
│       │   ├── apikey.go            # CRUD API keys
│       │   ├── domain.go            # Domain verification
│       │   ├── email.go             # Query emails + lifecycle
│       │   ├── suppression.go       # View/manage suppression lists
│       │   ├── audit.go             # Query audit logs
│       │   ├── webhook.go           # CRUD webhooks
│       │   ├── config.go            # Global config
│       │   ├── health.go            # Health check
│       │   └── onboarding.go        # First-run onboarding
│       │
│       ├── request/                   # Request DTOs + validation
│       │   ├── send.go
│       │   ├── tenant.go
│       │   ├── template.go
│       │   └── ...
│       │
│       └── response/                  # Response DTOs
│           ├── send.go
│           ├── email.go
│           └── ...
│
├── migrations/                        # SQL migrations (golang-migrate)
│   ├── 001_enums.up.sql
│   ├── 001_enums.down.sql
│   ├── 002_tenants_workspaces.up.sql
│   └── ...
│
├── config/                            # Configuration
│   ├── config.go                     # Struct + env vars + defaults
│   └── config.example.yaml
│
├── pkg/                               # Public packages (reusable by others)
│   ├── slug/                         # Slug validation
│   │   └── slug.go
│   ├── tracking/                     # Tracking ID generation
│   │   └── id.go
│   └── apperr/                       # Application error types
│       └── errors.go
│
├── docker/
│   ├── Dockerfile
│   ├── Dockerfile.dev
│   └── docker-compose.yml
│
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## 10. Port Interfaces (Contratos)

### 10.1. EmailSender Port

```go
// port/email_sender.go
package port

import "context"

// EmailSender is the port that email provider adapters must implement.
// Each adapter (SES, Gmail, etc.) implements this interface.
type EmailSender interface {
    // Send delivers a single email. Returns the provider's message ID.
    Send(ctx context.Context, msg *OutgoingEmail) (providerMessageID string, err error)

    // Name returns the adapter identifier (e.g., "ses", "gmail").
    Name() string

    // HealthCheck verifies the adapter can reach the provider.
    HealthCheck(ctx context.Context) error
}

// OutgoingEmail contains everything needed to send an email.
type OutgoingEmail struct {
    From        EmailAddress
    To          EmailAddress
    CC          []EmailAddress
    BCC         []EmailAddress
    ReplyTo     *EmailAddress
    Subject     string          // Already rendered (variables resolved)
    BodyHTML    string          // Compiled from MJML (final HTML)
    BodyText    string          // Plain text fallback
    Headers     map[string]string
    DKIMConfig  *DKIMConfig     // For signing
    TrackingID  string
}

type EmailAddress struct {
    Name    string // Display name ("Acme Support")
    Address string // Email ("support@acme.com")
}

type DKIMConfig struct {
    Selector   string
    Domain     string
    PrivateKey []byte // Decrypted
}
```

### 10.2. Store Ports (Repositories)

```go
// port/store.go
package port

import (
    "context"
    "github.com/google/uuid"
    "senda/internal/domain"
)

// TenantStore manages tenant persistence.
type TenantStore interface {
    Create(ctx context.Context, tenant *domain.Tenant) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
    GetByCode(ctx context.Context, code string) (*domain.Tenant, error)
    List(ctx context.Context, opts ListOptions) ([]*domain.Tenant, string, error)
    Update(ctx context.Context, tenant *domain.Tenant) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
    Purge(ctx context.Context, id uuid.UUID) error
}

// WorkspaceStore manages workspace persistence.
type WorkspaceStore interface {
    Create(ctx context.Context, ws *domain.Workspace) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
    GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error)
    GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error)
    ListByTenant(ctx context.Context, tenantID uuid.UUID, opts ListOptions) ([]*domain.Workspace, string, error)
    Update(ctx context.Context, ws *domain.Workspace) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}

// InjectorStore manages injector persistence.
type InjectorStore interface {
    // Definitions (schema)
    CreateDefinition(ctx context.Context, def *domain.InjectorDefinition) error
    GetDefinitionByID(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error)
    FindDefinitionByName(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error)
    ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)

    // Fields (immutable schema)
    CreateField(ctx context.Context, field *domain.InjectorField) error
    GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)

    // Values (overrideable)
    SetValue(ctx context.Context, val *domain.InjectorValue) error
    GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

// TemplateStore manages template persistence.
type TemplateStore interface {
    // Types
    CreateType(ctx context.Context, tt *domain.TemplateType) error
    GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
    FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error)

    // Templates
    CreateTemplate(ctx context.Context, tpl *domain.Template) error
    GetByTypeAndScope(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error)
    ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error)

    // Versions
    CreateVersion(ctx context.Context, ver *domain.TemplateVersion) error
    GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
    Publish(ctx context.Context, versionID uuid.UUID) error  // archives previous published
    ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error)

    // Locales
    SetLocale(ctx context.Context, locale *domain.TemplateVersionLocale) error
    GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error)
}

// EmailStore manages email persistence and queries.
type EmailStore interface {
    Create(ctx context.Context, email *domain.Email) error
    GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error
    UpdateRetry(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error

    AddEvent(ctx context.Context, event *domain.EmailEvent) error
    GetEvents(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error)

    // Queries (scoped)
    QueryByExternalID(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
    QueryByRecipient(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error)
    QueryByWorkspace(ctx context.Context, wsID uuid.UUID, filters EmailFilters, cursor string, limit int) ([]*domain.Email, string, error)

    // Cross-tenant (superadmin only)
    QueryByExternalIDGlobal(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
}

// MemberStore manages member persistence.
type MemberStore interface {
    Create(ctx context.Context, member *domain.Member) error
    GetByEmail(ctx context.Context, email string) (*domain.Member, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Member, error)
    CountAll(ctx context.Context) (int64, error) // For onboarding check

    AddRole(ctx context.Context, role *domain.MemberRole) error
    RemoveRole(ctx context.Context, roleID uuid.UUID) error
    GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error)
    GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error)
}

// SuppressionStore manages suppression lists.
type SuppressionStore interface {
    // Global
    AddGlobal(ctx context.Context, entry *domain.SuppressionGlobal) error
    IsGloballySuppressed(ctx context.Context, email string) (bool, error)
    RemoveGlobal(ctx context.Context, email string, removedBy uuid.UUID, reason string) error

    // Workspace
    AddWorkspace(ctx context.Context, entry *domain.SuppressionWorkspace) error
    IsWorkspaceSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, error)

    // Combined check (optimized)
    IsSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error) // returns (suppressed, reason, err)
}

// AdapterStore manages email adapter persistence.
type AdapterStore interface {
    Create(ctx context.Context, adapter *domain.Adapter) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
    Update(ctx context.Context, adapter *domain.Adapter) error
    SoftDelete(ctx context.Context, id uuid.UUID) error

    // ListInChain returns all adapters visible in the resolution chain.
    ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Adapter, error)

    // ListByWorkspace returns adapters owned by a specific workspace (or global if nil).
    ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts ListOptions) (*PageResult[domain.Adapter], error)
}

// DomainStore manages domain persistence and verification state.
type DomainStore interface {
    Create(ctx context.Context, d *domain.Domain) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error)
    Update(ctx context.Context, d *domain.Domain) error
    SoftDelete(ctx context.Context, id uuid.UUID) error

    // ListInChain returns all domains visible in the resolution chain.
    ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error)

    // ListByWorkspace returns domains owned by a specific workspace (or global if nil).
    ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts ListOptions) (*PageResult[domain.Domain], error)

    // GetPendingVerifications returns domains needing DNS re-check.
    GetPendingVerifications(ctx context.Context, limit int) ([]*domain.Domain, error)
}

// WebhookStore manages webhook endpoint persistence.
type WebhookStore interface {
    Create(ctx context.Context, wh *domain.Webhook) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
    Update(ctx context.Context, wh *domain.Webhook) error
    Delete(ctx context.Context, id uuid.UUID) error // hard delete — webhooks have no hierarchy

    // ListByWorkspace returns webhooks for a workspace.
    ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts ListOptions) (*PageResult[domain.Webhook], error)

    // GetActiveByWorkspace returns all enabled webhooks for event dispatch.
    GetActiveByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error)
}

// APIKeyStore manages API key persistence.
type APIKeyStore interface {
    Create(ctx context.Context, key *domain.APIKey) error
    GetByHash(ctx context.Context, hash string) (*domain.APIKey, error)
    Revoke(ctx context.Context, id uuid.UUID) error
    TouchLastUsed(ctx context.Context, id uuid.UUID) error

    // ListByWorkspace returns API keys for a workspace (hash excluded from response).
    ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts ListOptions) (*PageResult[domain.APIKey], error)
}

// AuditLogStore manages append-only audit log persistence.
type AuditLogStore interface {
    // Append writes a single audit entry. No update/delete allowed.
    Append(ctx context.Context, entry *domain.AuditLog) error

    // Query returns audit logs matching the filter with cursor pagination.
    Query(ctx context.Context, filter AuditFilter, opts ListOptions) (*PageResult[domain.AuditLog], error)
}

type AuditFilter struct {
    TenantID    *uuid.UUID
    WorkspaceID *uuid.UUID
    ActorID     *uuid.UUID
    Action      *string
    EntityType  *string
    Since       *time.Time
    Until       *time.Time
}

// GlobalConfigStore manages the singleton global configuration row.
type GlobalConfigStore interface {
    Get(ctx context.Context) (*domain.GlobalConfig, error)
    Upsert(ctx context.Context, cfg *domain.GlobalConfig) error
}

// ─── Pagination Types ──────────────────────────────────────

// ListOptions for paginated list queries.
type ListOptions struct {
    Cursor string // opaque cursor (base64-encoded id+timestamp)
    Limit  int    // max items to return (default 25, max 100)
}

// PageResult wraps a paginated response.
type PageResult[T any] struct {
    Items      []*T   `json:"items"`
    NextCursor string `json:"next_cursor,omitempty"` // empty = no more pages
    HasMore    bool   `json:"has_more"`
    Total      *int64 `json:"total,omitempty"`        // only if explicitly requested
}
```

### 10.3. Queue Port

```go
// port/queue.go
package port

import "context"

// JobQueue manages background job processing.
type JobQueue interface {
    // EnqueueSend enqueues an email send job.
    EnqueueSend(ctx context.Context, job *SendJob) error

    // EnqueueDomainCheck enqueues a domain verification check.
    EnqueueDomainCheck(ctx context.Context, domainID uuid.UUID) error

    // EnqueueWebhook enqueues a webhook delivery.
    EnqueueWebhook(ctx context.Context, job *WebhookJob) error
}

type SendJob struct {
    EmailID    uuid.UUID
    TrackingID string
    AdapterID  uuid.UUID
    Priority   int // 1 = highest
}

type WebhookJob struct {
    WebhookID  uuid.UUID
    EventType  string
    Payload    []byte
    RetryCount int
}
```

### 10.4. TemplateCompiler Port

```go
// port/template_compiler.go
package port

import "context"

// TemplateCompiler compiles MJML templates into HTML.
type TemplateCompiler interface {
    // Compile takes MJML source with resolved variables and returns HTML.
    Compile(ctx context.Context, mjml string) (html string, err error)
}

// VariableRenderer resolves variables in text (subject, preview, body).
type VariableRenderer interface {
    // Render replaces {{ injector.X.Y }} and {{ event.Z }} with actual values.
    Render(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error)
}
```

### 10.5. Cache Port

```go
// port/cache.go
package port

import (
    "context"
    "time"
)

// Cache provides key-value caching via a PG UNLOGGED table.
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    DeletePattern(ctx context.Context, pattern string) error // for global invalidation
}
```

---

## 11. Domain Models (Entidades)

### 11.1. Core Entities

```go
// domain/tenant.go
package domain

type Tenant struct {
    ID        uuid.UUID
    Code      string
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt *time.Time
}

type Workspace struct {
    ID                   uuid.UUID
    TenantID             uuid.UUID
    Code                 string
    Name                 string
    IsSystem             bool
    OpenTrackingEnabled  bool
    DefaultLocale        *string
    CreatedAt            time.Time
    UpdatedAt            time.Time
    DeletedAt            *time.Time
}
```

### 11.2. Injectors

```go
// domain/injector.go
package domain

type InjectorFieldType string

const (
    FieldTypeText    InjectorFieldType = "text"
    FieldTypeNumber  InjectorFieldType = "number"
    FieldTypeBoolean InjectorFieldType = "boolean"
    FieldTypeURL     InjectorFieldType = "url"
    FieldTypeImage   InjectorFieldType = "image_url"
    FieldTypeDate    InjectorFieldType = "date"
    FieldTypeJSON    InjectorFieldType = "json"
)

type InjectorDefinition struct {
    ID          uuid.UUID
    WorkspaceID *uuid.UUID // nil = global
    Name        string
    Description *string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
}

type InjectorField struct {
    ID           uuid.UUID
    DefinitionID uuid.UUID
    FieldName    string
    FieldType    InjectorFieldType
    IsRequired   bool
    DefaultValue *string
    SortOrder    int
}

type InjectorValue struct {
    ID          uuid.UUID
    FieldID     uuid.UUID
    WorkspaceID *uuid.UUID // scope where value is set
    Value       string     // stored as text, cast by field type
    UpdatedAt   time.Time
}
```

### 11.3. Templates

```go
// domain/template.go
package domain

type VersionStatus string

const (
    VersionStatusDraft     VersionStatus = "draft"
    VersionStatusPublished VersionStatus = "published"
    VersionStatusArchived  VersionStatus = "archived"
)

type TemplateType struct {
    ID             uuid.UUID
    WorkspaceID    *uuid.UUID // nil = global
    Slug           string
    Name           string
    Description    *string
    AdapterID      *uuid.UUID // adapter assigned by admin; nil = not configured yet
    VariableSchema map[string]any // JSON Schema for event variables
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      *time.Time
}

type Template struct {
    ID              uuid.UUID
    WorkspaceID     *uuid.UUID // nil = global
    TemplateTypeID  uuid.UUID
    Slug            string
    Name            string
    IsDisabled      bool // kill switch
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       *time.Time
}

type TemplateVersion struct {
    ID            uuid.UUID
    TemplateID    uuid.UUID
    VersionNumber int
    Status        VersionStatus
    Subject       string
    PreviewText   *string
    FromEmail     string
    FromName      string
    ReplyTo       *string
    BodyMJML      string // MJML source
    DefaultLocale string
    PublishedAt   *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type TemplateVersionLocale struct {
    ID        uuid.UUID
    VersionID uuid.UUID
    Locale    string // e.g., "es", "pt-BR"
    Subject   *string
    PreviewText *string
    FromName  *string
    BodyMJML  *string // nil = use default body
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 11.4. Adapters

```go
// domain/adapter.go
package domain

type AdapterType string

const (
    AdapterTypeSES  AdapterType = "ses"
    AdapterTypeSMTP AdapterType = "smtp"
)

type Adapter struct {
    ID                uuid.UUID
    WorkspaceID       *uuid.UUID // nil = global
    Name              string
    AdapterType       AdapterType
    ConfigEncrypted   []byte  // AES-256-GCM encrypted JSON
    IsDefault         bool
    CreatedAt         time.Time
    UpdatedAt         time.Time
    DeletedAt         *time.Time
}
```

### 11.5. Domains

```go
// domain/domain_entity.go
package domain

type DomainStatus string

const (
    DomainStatusPending  DomainStatus = "pending"
    DomainStatusVerified DomainStatus = "verified"
    DomainStatusError    DomainStatus = "error"
)

type Domain struct {
    ID            uuid.UUID
    WorkspaceID   *uuid.UUID // nil = global
    DomainName    string     // e.g., "example.com"
    DKIMSelector  string     // e.g., "senda"
    DKIMPublicKey string
    DKIMPrivateKeyEncrypted []byte // AES-256-GCM
    Status        DomainStatus
    VerifiedAt    *time.Time
    LastCheckAt   *time.Time
    NextCheckAt   *time.Time
    LastError     *string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    DeletedAt     *time.Time
}
```

### 11.6. Members & Auth

```go
// domain/member.go
package domain

type Role string

const (
    RoleSuperadmin     Role = "superadmin"
    RoleTenantAdmin    Role = "tenant_admin"
    RoleWorkspaceAdmin Role = "workspace_admin"
    RoleWorkspaceEditor Role = "workspace_editor"
    RoleWorkspaceViewer Role = "workspace_viewer"
)

// Level returns the role's numeric level for comparison.
// Higher = more permissions.
func (r Role) Level() int {
    switch r {
    case RoleSuperadmin:     return 100
    case RoleTenantAdmin:    return 80
    case RoleWorkspaceAdmin: return 60
    case RoleWorkspaceEditor: return 40
    case RoleWorkspaceViewer: return 20
    default: return 0
    }
}

type Member struct {
    ID        uuid.UUID
    Email     string
    Name      *string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type MemberRole struct {
    ID          uuid.UUID
    MemberID    uuid.UUID
    Role        Role
    ScopeType   ScopeType
    TenantID    *uuid.UUID
    WorkspaceID *uuid.UUID
    CreatedAt   time.Time
}

type ScopeType string

const (
    ScopeGlobal    ScopeType = "global"
    ScopeTenant    ScopeType = "tenant"
    ScopeWorkspace ScopeType = "workspace"
)

type APIKey struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    Name        string
    KeyHash     string    // SHA-256 of the raw key
    KeyPrefix   string    // first 8 chars for identification
    LastUsedAt  *time.Time
    RevokedAt   *time.Time
    CreatedAt   time.Time
}
```

### 11.7. Webhooks

```go
// domain/webhook.go
package domain

type Webhook struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    URL         string
    Secret      string // HMAC signing secret
    Events      []string // ["email.sent", "email.delivered", "email.bounced", ...]
    IsActive    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// SubscribesTo checks if this webhook listens for the given event type.
func (w *Webhook) SubscribesTo(eventType string) bool {
    for _, e := range w.Events {
        if e == eventType || e == "*" {
            return true
        }
    }
    return false
}
```

### 11.8. Suppression

```go
// domain/suppression.go
package domain

type BounceType string

const (
    BounceHard BounceType = "hard"
    BounceSoft BounceType = "soft"
)

type SuppressionReason string

const (
    SuppressionBounce    SuppressionReason = "bounce"
    SuppressionComplaint SuppressionReason = "complaint"
    SuppressionManual    SuppressionReason = "manual"
)

type SuppressionGlobal struct {
    ID        uuid.UUID
    Email     string
    Reason    SuppressionReason
    Source    string // "ses_webhook", "manual", etc.
    CreatedAt time.Time
}

type SuppressionWorkspace struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    Email       string
    Reason      SuppressionReason
    Source      string
    CreatedAt   time.Time
}
```

### 11.9. Audit Log

```go
// domain/audit.go
package domain

type AuditAction string

const (
    AuditCreate    AuditAction = "create"
    AuditUpdate    AuditAction = "update"
    AuditDelete    AuditAction = "delete"
    AuditPublish   AuditAction = "publish"
    AuditDisable   AuditAction = "disable"
    AuditEnable    AuditAction = "enable"
    AuditRevoke    AuditAction = "revoke"
    AuditPurge     AuditAction = "purge"
    AuditLogin     AuditAction = "login"
)

type AuditLog struct {
    ID          uuid.UUID
    ActorID     uuid.UUID
    ActorEmail  string
    Action      AuditAction
    EntityType  string     // "tenant", "workspace", "template", etc.
    EntityID    uuid.UUID
    TenantID    *uuid.UUID
    WorkspaceID *uuid.UUID
    Detail      map[string]any // arbitrary context
    IPAddress   string
    CreatedAt   time.Time
}
```

### 11.10. Global Config

```go
// domain/global_config.go
package domain

type GlobalConfig struct {
    ID                       uuid.UUID
    DefaultRateLimitPerSecond int
    MaxRecipientsPerRequest   int
    RetentionDays            int
    MaintenanceMode          bool
    UpdatedAt                time.Time
}
```

### 11.11. Addressing

```go
// domain/addressing.go
package domain

// TemplateRef represents the deterministic addressing: tenantCode:workspaceCode:templateType
type TemplateRef struct {
    TenantCode    string
    WorkspaceCode string
    TemplateType  string
}

// ParseRef parses "latam:acme:welcome" into a TemplateRef.
func ParseRef(ref string) (*TemplateRef, error) {
    parts := strings.SplitN(ref, ":", 3)
    if len(parts) != 3 {
        return nil, ErrInvalidRef
    }
    return &TemplateRef{
        TenantCode:    parts[0],
        WorkspaceCode: parts[1],
        TemplateType:  parts[2],
    }, nil
}
```

### 11.3. Email Entity

```go
// domain/email.go
package domain

type EmailStatus string

const (
    StatusQueued     EmailStatus = "queued"
    StatusProcessing EmailStatus = "processing"
    StatusSent       EmailStatus = "sent"
    StatusDelivered  EmailStatus = "delivered"
    StatusOpened     EmailStatus = "opened"
    StatusBounced    EmailStatus = "bounced"
    StatusComplained EmailStatus = "complained"
    StatusFailed     EmailStatus = "failed"
    StatusSuppressed EmailStatus = "suppressed"
)

type Email struct {
    ID                uuid.UUID
    TrackingID        string
    ExternalID        *string
    WorkspaceID       uuid.UUID
    TenantID          uuid.UUID
    TemplateID        uuid.UUID
    TemplateVersionID uuid.UUID
    TemplateTypeSlug  string
    TemplateRef       string     // original "latam:acme:welcome"
    RecipientEmail    string
    CC                []string
    BCC               []string
    FromEmail         string
    FromName          string
    ReplyTo           *string
    SubjectRendered   string
    Locale            *string
    Status            EmailStatus
    AdapterID         uuid.UUID
    ProviderMsgID     *string
    VariablesSnapshot map[string]any
    InjectorsSnapshot map[string]map[string]any
    BodyMJML          string  // MJML source snapshot (rendered with variables before compile)
    RetryCount        int
    MaxRetries        int
    NextRetryAt       *time.Time
    CreatedAt         time.Time
    UpdatedAt         time.Time
}

type EmailEvent struct {
    ID         uuid.UUID
    EmailID    uuid.UUID
    EventType  EmailStatus
    OccurredAt time.Time
    Metadata   map[string]any
}
```

---

## 12. Resolution Engine

The resolution engine is Senda's central component. It resolves resources through the hierarchical chain.

### 12.1. ChainResolver

```go
// resolution/chain.go
package resolution

// ChainResolver builds the resolution chain for a workspace.
type ChainResolver struct {
    workspaceStore port.WorkspaceStore
    cache          port.Cache
}

// ResolutionChain represents the ordered list of scopes to check.
// Index 0 = highest priority (workspace), last = lowest (global/nil).
type ResolutionChain struct {
    WorkspaceID       uuid.UUID       // target workspace
    SystemWorkspaceID uuid.UUID       // _system of tenant
    TenantID          uuid.UUID       // for denormalization
    Scopes            []uuid.NullUUID // [workspace_id, system_ws_id, NULL(global)]
}

func (r *ChainResolver) Resolve(ctx context.Context, workspaceID uuid.UUID) (*ResolutionChain, error) {
    // Check cache first (5min TTL)
    cacheKey := fmt.Sprintf("chain:%s", workspaceID)
    if cached, err := r.cache.Get(ctx, cacheKey); err == nil {
        return deserialize(cached), nil
    }

    // Build chain
    ws, err := r.workspaceStore.GetByID(ctx, workspaceID)
    if err != nil { return nil, err }

    chain := &ResolutionChain{
        WorkspaceID: workspaceID,
        TenantID:    ws.TenantID,
    }

    if ws.IsSystem {
        // _system workspace: chain is [_system, global]
        chain.SystemWorkspaceID = workspaceID
        chain.Scopes = []uuid.NullUUID{
            {UUID: workspaceID, Valid: true},
            {Valid: false}, // global (NULL)
        }
    } else {
        // Regular workspace: chain is [workspace, _system, global]
        sysWs, err := r.workspaceStore.GetSystemWorkspace(ctx, ws.TenantID)
        if err != nil { return nil, err }

        chain.SystemWorkspaceID = sysWs.ID
        chain.Scopes = []uuid.NullUUID{
            {UUID: workspaceID, Valid: true},
            {UUID: sysWs.ID, Valid: true},
            {Valid: false}, // global (NULL)
        }
    }

    // Cache for 5 minutes
    r.cache.Set(ctx, cacheKey, serialize(chain), 5*time.Minute)
    return chain, nil
}
```

### 12.2. InjectorMerger

```go
// resolution/injector_merger.go
package resolution

// InjectorMerger resolves and merges injector values field-by-field.
type InjectorMerger struct {
    store         port.InjectorStore
    chainResolver *ChainResolver
}

// MergedInjectors is the fully resolved map: injectorName -> fieldName -> value
type MergedInjectors map[string]map[string]any

func (m *InjectorMerger) Resolve(ctx context.Context, workspaceID uuid.UUID) (MergedInjectors, error) {
    chain, err := m.chainResolver.Resolve(ctx, workspaceID)
    if err != nil { return nil, err }

    // 1. Get all injector definitions visible in the chain
    defs, err := m.store.ListDefinitionsInChain(ctx, chain.Scopes)
    if err != nil { return nil, err }

    // 2. Deduplicate by name (first in chain wins for definition ownership)
    seen := map[string]bool{}
    uniqueDefs := make([]*domain.InjectorDefinition, 0)
    for _, scope := range chain.Scopes {
        for _, def := range defs {
            if def.WorkspaceID == scope && !seen[def.Name] {
                uniqueDefs = append(uniqueDefs, def)
                seen[def.Name] = true
            }
        }
    }

    // 3. For each definition, merge field values
    result := MergedInjectors{}
    for _, def := range uniqueDefs {
        fields, _ := m.store.GetFieldsByDefinition(ctx, def.ID)
        values, _ := m.store.GetValues(ctx, def.ID, chain.Scopes)

        merged := map[string]any{}
        for _, field := range fields {
            // Find value: workspace → _system → global (first wins)
            for _, scope := range chain.Scopes {
                for _, val := range values {
                    if val.FieldName == field.FieldName && val.WorkspaceID == scope {
                        merged[field.FieldName] = val.Value
                        goto nextField
                    }
                }
            }
            nextField:
        }
        result[def.Name] = merged
    }

    return result, nil
}
```

### 12.3. TemplateResolver

```go
// resolution/template_resolver.go
package resolution

type TemplateResolver struct {
    store         port.TemplateStore
    chainResolver *ChainResolver
}

// ResolvedTemplate contains everything needed to render an email.
type ResolvedTemplate struct {
    Template       *domain.Template
    Version        *domain.TemplateVersion
    Locale         *domain.TemplateVersionLocale // nil if no locale match
    TemplateType   *domain.TemplateType
}

func (r *TemplateResolver) Resolve(ctx context.Context, workspaceID uuid.UUID, ref *domain.TemplateRef, locale *string) (*ResolvedTemplate, error) {
    // 1. Build resolution chain
    chain, err := r.chainResolver.Resolve(ctx, workspaceID)

    // 2. Find template type in chain
    templateType, err := r.store.GetTypeBySlug(ctx, ref.TemplateType, chain.Scopes)
    if err != nil {
        return nil, domain.ErrTemplateTypeNotFound
    }

    // 3. Find template for this type in chain (workspace → _system → global)
    template, err := r.store.ResolveTemplate(ctx, templateType.ID, chain.Scopes)
    if err != nil {
        return nil, domain.ErrTemplateNotFound
    }

    // 4. Check kill switch
    if template.IsDisabled {
        return nil, domain.ErrTemplateDisabled
    }

    // 5. Get published version
    version, err := r.store.GetPublishedVersion(ctx, template.ID)
    if err != nil {
        return nil, domain.ErrNoPublishedVersion
    }

    // 6. Resolve locale (optional)
    var localeContent *domain.TemplateVersionLocale
    if locale != nil && *locale != version.DefaultLocale {
        localeContent, _ = r.store.GetLocale(ctx, version.ID, *locale)
        // If not found, fallback to default (localeContent stays nil)
    }

    return &ResolvedTemplate{
        Template:     template,
        Version:      version,
        Locale:       localeContent,
        TemplateType: templateType,
    }, nil
}
```

### 12.4. AdapterResolver

```go
// resolution/adapter_resolver.go
package resolution

// AdapterResolver resolves the email adapter based on the template type's assigned adapter.
// The admin assigns an adapter to each template type via the dashboard.
// This replaces chain-based resolution: the mapping is explicit, not inherited.
type AdapterResolver struct {
    adapterStore port.AdapterStore
    cache        port.Cache
}

type ResolvedAdapter struct {
    Adapter *domain.Adapter
}

// ResolveForTemplateType gets the adapter assigned to the given template type.
// Returns ErrNoAdapterConfigured (422) if no adapter is assigned.
func (r *AdapterResolver) ResolveForTemplateType(ctx context.Context, templateType *domain.TemplateType) (*ResolvedAdapter, error) {
    if templateType.AdapterID == nil {
        return nil, domain.ErrNoAdapterConfigured // 422
    }

    // Check cache
    cacheKey := fmt.Sprintf("adapter:%s", *templateType.AdapterID)
    if cached, err := r.cache.Get(ctx, cacheKey); err == nil {
        return deserializeAdapter(cached), nil
    }

    adapter, err := r.adapterStore.GetByID(ctx, *templateType.AdapterID)
    if err != nil {
        return nil, fmt.Errorf("adapter %s assigned to template type %s not found: %w",
            *templateType.AdapterID, templateType.Slug, err)
    }

    if adapter.DeletedAt != nil {
        return nil, fmt.Errorf("%w: assigned adapter %s was deleted", domain.ErrNoAdapterConfigured, adapter.Name)
    }

    result := &ResolvedAdapter{Adapter: adapter}
    r.cache.Set(ctx, cacheKey, serializeAdapter(result), 10*time.Minute)
    return result, nil
}
```

### 12.5. DomainResolver

```go
// resolution/domain_resolver.go
package resolution

// DomainResolver validates that the from_email address belongs to a verified domain
// visible in the workspace's resolution chain.
// Inheritance: domain verified at global applies to all workspaces.
type DomainResolver struct {
    store         port.DomainStore
    chainResolver *ChainResolver
    cache         port.Cache
}

// ValidateFromAddress checks that the domain part of fromEmail has a verified domain
// in the resolution chain. Returns nil if valid, error if not.
func (r *DomainResolver) ValidateFromAddress(ctx context.Context, workspaceID uuid.UUID, fromEmail string) error {
    emailDomain := extractDomain(fromEmail) // "user@example.com" → "example.com"

    // Check cache
    cacheKey := fmt.Sprintf("domain_valid:%s:%s", workspaceID, emailDomain)
    if cached, err := r.cache.Get(ctx, cacheKey); err == nil && string(cached) == "1" {
        return nil
    }

    chain, err := r.chainResolver.Resolve(ctx, workspaceID)
    if err != nil {
        return err
    }

    // Get all verified domains in chain
    domains, err := r.store.ListInChain(ctx, chain.Scopes)
    if err != nil {
        return err
    }

    for _, d := range domains {
        if d.DomainName == emailDomain && d.Status == domain.DomainStatusVerified && d.DeletedAt == nil {
            // Cache positive result for 10min (domain verification changes are rare)
            r.cache.Set(ctx, cacheKey, []byte("1"), 10*time.Minute)
            return nil
        }
    }

    return fmt.Errorf("%w: domain %s not verified in scope chain", domain.ErrDomainNotVerified, emailDomain)
}

func extractDomain(email string) string {
    parts := strings.SplitN(email, "@", 2)
    if len(parts) != 2 {
        return ""
    }
    return strings.ToLower(parts[1])
}
```

### 12.6. Cache Invalidation Strategy

The resolution engine caches data to avoid repetitive queries. Invalidation is based on **event-driven invalidation** from the management services.

```go
// resolution/cache_invalidator.go
package resolution

// CacheInvalidator centralizes cache invalidation for the resolution engine.
// Called by management services when resources change.
type CacheInvalidator struct {
    cache port.Cache
}

// Key patterns:
//   chain:<workspace_id>       — ResolutionChain (ChainResolver)
//   adapter:<adapter_id>       — Adapter by ID (AdapterResolver)
//   domain_valid:<ws_id>:<dom> — Domain validation (DomainResolver)

// InvalidateWorkspace clears all cached data for a workspace and its tenant.
// Called when: workspace updated, workspace deleted, _system workspace modified.
func (c *CacheInvalidator) InvalidateWorkspace(ctx context.Context, workspaceID uuid.UUID) {
    c.cache.Delete(ctx, fmt.Sprintf("chain:%s", workspaceID))
    // domain_valid keys expire naturally (10min TTL)
}

// InvalidateAdapter clears cached adapter data.
// Called when: adapter updated, adapter deleted.
func (c *CacheInvalidator) InvalidateAdapter(ctx context.Context, adapterID uuid.UUID) {
    c.cache.Delete(ctx, fmt.Sprintf("adapter:%s", adapterID))
}

// InvalidateTenantWorkspaces clears caches for ALL workspaces in a tenant.
// Called when: _system workspace config changes, tenant-level resource changes.
// Implementation: store workspace IDs in cache table with tenant prefix for bulk invalidation.
func (c *CacheInvalidator) InvalidateTenantWorkspaces(ctx context.Context, tenantID uuid.UUID) {
    // Option A: iterate known workspace IDs (small N, acceptable)
    // Option B: use cache.DeletePattern (DELETE WHERE key LIKE prefix%)
    // Chosen: Option A — tenants have bounded workspace count
    workspaceIDs, _ := c.getWorkspaceIDs(ctx, tenantID)
    for _, wsID := range workspaceIDs {
        c.InvalidateWorkspace(ctx, wsID)
    }
}

// InvalidateGlobal clears caches affected by global resource changes.
// Called when: global adapter/domain/injector/template changes.
// This is the most expensive operation — clears all cached chains.
func (c *CacheInvalidator) InvalidateGlobal(ctx context.Context) {
    // Global changes are rare. Use cache.DeletePattern("chain:*")
    // and delete matching keys. Acceptable cost for infrequent operation.
    c.cache.DeletePattern(ctx, "chain:*")
    c.cache.DeletePattern(ctx, "adapter:*")
}
```

**Invalidation rules by operation:**

| Operation | Invalidated keys |
|-----------|-----------------|
| Modify/delete adapter | `adapter:<adapter_id>` |
| Create/delete workspace | `chain:<workspace_id>` |
| Modify domain | `domain_valid:<ws_id>:<domain>` (natural TTL) |
| Modify `_system` workspace | All tenant `chain:*` keys |

**Note:** `DeletePattern` is defined in the `Cache` port (section 10.5). The in-memory implementation uses `Clear()` because global invalidation is rare.

---

## 13. SendService — Main Flow

```go
// service/send.go
package service

type SendService struct {
    templateResolver *resolution.TemplateResolver
    injectorMerger   *resolution.InjectorMerger
    adapterResolver  *resolution.AdapterResolver
    domainResolver   *resolution.DomainResolver
    emailStore       port.EmailStore
    suppression      port.SuppressionStore
    queue            port.JobQueue
    compiler         port.TemplateCompiler
    renderer         port.VariableRenderer
    adapterRegistry  *AdapterRegistry
}

type SendRequest struct {
    Ref        string            `json:"ref" validate:"required"`
    To         []string          `json:"to" validate:"required,min=1,max=50,dive,email"`
    CC         []string          `json:"cc,omitempty" validate:"omitempty,dive,email"`
    BCC        []string          `json:"bcc,omitempty" validate:"omitempty,dive,email"`
    Variables  map[string]any    `json:"variables"`
    ExternalID *string           `json:"external_id,omitempty"`
    Locale     *string           `json:"locale,omitempty"`
}

type SendResponse struct {
    Status           string           `json:"status"`
    TrackingIDs      []TrackingEntry  `json:"tracking_ids"`
    ExternalID       *string          `json:"external_id,omitempty"`
    TemplateResolved string           `json:"template_resolved"`
    TemplateVersion  int              `json:"template_version"`
}

type TrackingEntry struct {
    To         string `json:"to"`
    TrackingID string `json:"tracking_id"`
}

func (s *SendService) Send(ctx context.Context, workspaceID uuid.UUID, req *SendRequest) (*SendResponse, error) {
    // 1. Parse addressing
    ref, err := domain.ParseRef(req.Ref)
    if err != nil {
        return nil, apperr.BadRequest("invalid ref format: %s", req.Ref)
    }

    // 2. Resolve template (includes kill switch check)
    resolved, err := s.templateResolver.Resolve(ctx, workspaceID, ref, req.Locale)
    if err != nil {
        return nil, mapResolutionError(err)
    }

    // 3. Validate event variables against template type schema
    if err := validateVariables(resolved.TemplateType.VariableSchema, req.Variables); err != nil {
        return nil, apperr.UnprocessableEntity("variable validation failed: %v", err)
    }

    // 4. Resolve injectors (field-by-field merge)
    injectors, err := s.injectorMerger.Resolve(ctx, workspaceID)
    if err != nil {
        return nil, err
    }

    // 5. Resolve adapter (from template type assignment, not chain)
    adapter, err := s.adapterResolver.ResolveForTemplateType(ctx, resolved.TemplateType)
    if err != nil {
        return nil, apperr.UnprocessableEntity("no adapter assigned to template type '%s'", ref.TemplateType)
    }

    // 6. Resolve domain (validate from_email)
    fromEmail := resolveFromEmail(resolved, injectors)
    if err := s.domainResolver.ValidateFromAddress(ctx, workspaceID, fromEmail); err != nil {
        return nil, apperr.UnprocessableEntity("from address domain not verified: %s", fromEmail)
    }

    // 7. Render subject and preview text
    subject := getLocalizedField(resolved, "subject")
    previewText := getLocalizedField(resolved, "preview_text")
    fromName := getLocalizedField(resolved, "from_name")

    renderedSubject, _ := s.renderer.Render(subject, injectors, req.Variables)
    renderedPreview, _ := s.renderer.Render(previewText, injectors, req.Variables)
    renderedFromName, _ := s.renderer.Render(fromName, injectors, req.Variables)

    // 8. Create email records (one per recipient)
    response := &SendResponse{
        Status:           "accepted",
        TemplateResolved: req.Ref,
        TemplateVersion:  resolved.Version.VersionNumber,
        ExternalID:       req.ExternalID,
    }

    for _, recipient := range req.To {
        // 8a. Check suppression
        suppressed, reason, _ := s.suppression.IsSuppressed(ctx, workspaceID, recipient)

        trackingID := tracking.GenerateID() // "trk_" + random

        email := &domain.Email{
            ID:                uuid.New(),
            TrackingID:        trackingID,
            ExternalID:        req.ExternalID,
            WorkspaceID:       workspaceID,
            TenantID:          /*from chain*/,
            TemplateID:        resolved.Template.ID,
            TemplateVersionID: resolved.Version.ID,
            TemplateTypeSlug:  ref.TemplateType,
            TemplateRef:       req.Ref,
            RecipientEmail:    recipient,
            CC:                req.CC,
            BCC:               req.BCC,
            FromEmail:         fromEmail,
            FromName:          renderedFromName,
            SubjectRendered:   renderedSubject,
            Locale:            req.Locale,
            AdapterID:         adapter.ID,
            VariablesSnapshot: req.Variables,
            InjectorsSnapshot: injectors,
            BodyMJML:          getLocalizedBody(resolved),
        }

        if suppressed {
            email.Status = domain.StatusSuppressed
            s.emailStore.Create(ctx, email)
            s.emailStore.AddEvent(ctx, &domain.EmailEvent{
                EmailID:   email.ID,
                EventType: domain.StatusSuppressed,
                Metadata:  map[string]any{"reason": reason},
            })
        } else {
            email.Status = domain.StatusQueued
            s.emailStore.Create(ctx, email)
            s.queue.EnqueueSend(ctx, &port.SendJob{
                EmailID:    email.ID,
                TrackingID: trackingID,
                AdapterID:  adapter.ID,
            })
        }

        response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
            To:         recipient,
            TrackingID: trackingID,
        })
    }

    return response, nil
}
```

---

## 14. Middleware Chain

### 14.1. Request Flow

```
Request
  │
  ▼
┌─────────────┐
│ Recovery     │  Panic → 500
├─────────────┤
│ RequestID    │  X-Request-ID header
├─────────────┤
│ Logger       │  Structured logging (slog)
├─────────────┤
│ Metrics      │  Prometheus request duration + count
├─────────────┤
│ Auth         │  API Key OR OIDC token → identity
├─────────────┤
│ RBAC         │  Role → permissions check
├─────────────┤
│ Scope        │  Extract tenant/workspace from URL
├─────────────┤
│ Handler      │  Business logic
└─────────────┘
```

### 14.2. Auth Middleware (Dual Mode)

```go
// http/middleware/auth.go
package middleware

// AuthMiddleware supports two authentication modes:
// 1. API Key (Bearer senda_live_*) → workspace-scoped
// 2. OIDC JWT (Bearer eyJ*) → member with roles
func AuthMiddleware(apiKeyStore port.APIKeyStore, memberStore port.MemberStore, oidcVerifier OIDCVerifier) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            token := extractBearerToken(c)
            if token == "" {
                return echo.NewHTTPError(401, "missing authorization")
            }

            if strings.HasPrefix(token, "senda_live_") {
                // API Key auth
                hash := sha256Hex(token)
                apiKey, err := apiKeyStore.GetByHash(c.Request().Context(), hash)
                if err != nil || apiKey.RevokedAt != nil {
                    return echo.NewHTTPError(401, "invalid or revoked API key")
                }
                // Update last_used_at async
                go apiKeyStore.TouchLastUsed(context.Background(), apiKey.ID)

                c.Set("auth_type", "apikey")
                c.Set("workspace_id", apiKey.WorkspaceID)
                return next(c)
            }

            // OIDC JWT auth
            claims, err := oidcVerifier.Verify(c.Request().Context(), token)
            if err != nil {
                return echo.NewHTTPError(401, "invalid OIDC token")
            }

            member, err := memberStore.GetByEmail(c.Request().Context(), claims.Email)
            if err != nil {
                return echo.NewHTTPError(403, "access denied: email not registered as member")
            }

            roles, _ := memberStore.GetRoles(c.Request().Context(), member.ID)

            c.Set("auth_type", "oidc")
            c.Set("member", member)
            c.Set("roles", roles)
            return next(c)
        }
    }
}
```

### 14.3. RBAC Middleware

```go
// http/middleware/rbac.go
package middleware

// RequireRole checks that the authenticated member has the required role
// for the target scope (extracted from URL params).
func RequireRole(minRole domain.Role) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            authType := c.Get("auth_type").(string)
            if authType == "apikey" {
                // API Keys bypass RBAC — they're already workspace-scoped
                // and can only do data-plane operations
                return next(c)
            }

            roles := c.Get("roles").([]*domain.MemberRole)
            targetScope := extractTargetScope(c) // from URL params

            if !hasPermission(roles, minRole, targetScope) {
                return echo.NewHTTPError(403, "insufficient permissions")
            }

            return next(c)
        }
    }
}

func hasPermission(roles []*domain.MemberRole, minRole domain.Role, target TargetScope) bool {
    for _, r := range roles {
        // Superadmin can do anything
        if r.Role == domain.RoleSuperadmin {
            return true
        }
        // Tenant admin can do anything in their tenant
        if r.Role == domain.RoleTenantAdmin && r.TenantID != nil && *r.TenantID == target.TenantID {
            if minRole.Level() <= domain.RoleTenantAdmin.Level() {
                return true
            }
        }
        // Workspace roles check workspace match
        if r.WorkspaceID != nil && *r.WorkspaceID == target.WorkspaceID {
            if r.Role.Level() >= minRole.Level() {
                return true
            }
        }
    }
    return false
}
```

---

## 15. API Contract

### 15.1. Pagination (Cursor-based)

All LIST routes use cursor-based pagination with the following contract:

**Request query params:**
```
?cursor=<opaque_string>&limit=25
```

- `cursor`: opaque, base64-encoded `timestamp:uuid`. Omit for the first page.
- `limit`: 1-100, default 25.

**Response envelope:**
```json
{
  "items": [...],
  "next_cursor": "eyJ0IjoiMjAyNi0wMS0wMVQwMDowMDowMFoiLCJpZCI6IjAxOTQ...",
  "has_more": true
}
```

**Cursor implementation:**
```go
// http/pagination.go
package httputil

type CursorData struct {
    Timestamp time.Time `json:"t"`
    ID        uuid.UUID `json:"id"`
}

func EncodeCursor(t time.Time, id uuid.UUID) string {
    data, _ := json.Marshal(CursorData{Timestamp: t, ID: id})
    return base64.URLEncoding.EncodeToString(data)
}

func DecodeCursor(cursor string) (*CursorData, error) {
    data, err := base64.URLEncoding.DecodeString(cursor)
    if err != nil {
        return nil, domain.ErrInvalidCursor
    }
    var c CursorData
    if err := json.Unmarshal(data, &c); err != nil {
        return nil, domain.ErrInvalidCursor
    }
    return &c, nil
}

// SQL pattern for cursor pagination:
// WHERE (created_at, id) < ($cursor_timestamp, $cursor_id)
// ORDER BY created_at DESC, id DESC
// LIMIT $limit + 1  -- fetch one extra to detect has_more
```

### 15.2. Error Response Contract

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human-readable description",
    "details": [
      {"field": "to", "message": "must contain at least 1 email"}
    ],
    "request_id": "req_abc123"
  }
}
```

**Error codes HTTP:**

| HTTP | Code | Uso |
|------|------|-----|
| 400 | BAD_REQUEST | Malformed JSON, invalid params |
| 401 | UNAUTHORIZED | Missing or invalid token/API key |
| 403 | FORBIDDEN | Permisos insuficientes |
| 404 | NOT_FOUND | Recurso no existe |
| 409 | CONFLICT | Duplicate code, version already published |
| 422 | UNPROCESSABLE | No adapter, unverified domain, invalid variables |
| 429 | RATE_LIMITED | Rate limit excedido |
| 500 | INTERNAL_ERROR | Error interno |

### 15.3. Routes

```go
// http/server.go — route registration

func (s *Server) registerRoutes() {
    e := s.echo

    // Health (no auth)
    e.GET("/health", s.healthHandler.Health)

    // ─── Data Plane API (API Key OR OIDC) ─────────────────────
    api := e.Group("/api/v1")
    api.Use(middleware.AuthMiddleware(s.apiKeyStore, s.memberStore, s.oidcVerifier))

    // Send
    api.POST("/send", s.sendHandler.Send)

    // Query (scoped by API Key workspace or RBAC)
    api.GET("/emails/:tracking_id", s.emailHandler.GetByTrackingID)
    api.GET("/emails", s.emailHandler.Query)           // ?external_id=X, ?recipient=Y, ?status=X
    api.GET("/emails/export", s.emailHandler.Export)    // CSV export

    // ─── Management API (OIDC only) ──────────────────────────
    mgmt := e.Group("/api/v1/manage")
    mgmt.Use(middleware.OIDCOnly())  // reject API Keys

    // Onboarding
    mgmt.POST("/onboarding", s.onboardingHandler.Setup)
    mgmt.GET("/onboarding/status", s.onboardingHandler.Status)

    // Tenants (superadmin)
    mgmt.POST("/tenants", s.tenantHandler.Create, middleware.RequireRole(domain.RoleSuperadmin))
    mgmt.GET("/tenants", s.tenantHandler.List, middleware.RequireRole(domain.RoleSuperadmin))
    mgmt.GET("/tenants/:code", s.tenantHandler.Get, middleware.RequireRole(domain.RoleTenantAdmin))
    mgmt.PUT("/tenants/:code", s.tenantHandler.Update, middleware.RequireRole(domain.RoleSuperadmin))
    mgmt.DELETE("/tenants/:code", s.tenantHandler.SoftDelete, middleware.RequireRole(domain.RoleSuperadmin))

    // Workspaces (tenant-admin+)
    tenantWs := mgmt.Group("/tenants/:tenantCode/workspaces")
    tenantWs.POST("", s.workspaceHandler.Create, middleware.RequireRole(domain.RoleTenantAdmin))
    tenantWs.GET("", s.workspaceHandler.List, middleware.RequireRole(domain.RoleTenantAdmin))
    tenantWs.GET("/:wsCode", s.workspaceHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer))
    tenantWs.PUT("/:wsCode", s.workspaceHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    tenantWs.DELETE("/:wsCode", s.workspaceHandler.SoftDelete, middleware.RequireRole(domain.RoleTenantAdmin))

    // Scoped resources (within a workspace context)
    ws := mgmt.Group("/tenants/:tenantCode/workspaces/:wsCode")

    // Injectors
    ws.GET("/injectors", s.injectorHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/injectors", s.injectorHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.GET("/injectors/:name", s.injectorHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.PUT("/injectors/:name", s.injectorHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.PUT("/injectors/:name/values", s.injectorHandler.SetValues, middleware.RequireRole(domain.RoleWorkspaceEditor))
    ws.DELETE("/injectors/:name", s.injectorHandler.SoftDelete, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Template Types
    ws.GET("/template-types", s.templateTypeHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/template-types", s.templateTypeHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.GET("/template-types/:slug", s.templateTypeHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.PUT("/template-types/:slug", s.templateTypeHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.DELETE("/template-types/:slug", s.templateTypeHandler.SoftDelete, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Templates
    ws.GET("/templates", s.templateHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/templates", s.templateHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.GET("/templates/:slug", s.templateHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.PUT("/templates/:slug", s.templateHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.PUT("/templates/:slug/disable", s.templateHandler.Disable, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.PUT("/templates/:slug/enable", s.templateHandler.Enable, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Template Versions
    ws.GET("/templates/:slug/versions", s.templateHandler.ListVersions, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/templates/:slug/versions", s.templateHandler.CreateDraft, middleware.RequireRole(domain.RoleWorkspaceEditor))
    ws.GET("/templates/:slug/versions/:versionNum", s.templateHandler.GetVersion, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.PUT("/templates/:slug/versions/:versionNum", s.templateHandler.UpdateDraft, middleware.RequireRole(domain.RoleWorkspaceEditor))
    ws.POST("/templates/:slug/versions/:versionNum/publish", s.templateHandler.Publish, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.POST("/templates/:slug/versions/:versionNum/archive", s.templateHandler.Archive, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Template Version Locales
    ws.GET("/templates/:slug/versions/:versionNum/locales", s.templateHandler.ListLocales, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/templates/:slug/versions/:versionNum/locales", s.templateHandler.CreateLocale, middleware.RequireRole(domain.RoleWorkspaceEditor))
    ws.PUT("/templates/:slug/versions/:versionNum/locales/:locale", s.templateHandler.UpdateLocale, middleware.RequireRole(domain.RoleWorkspaceEditor))
    ws.DELETE("/templates/:slug/versions/:versionNum/locales/:locale", s.templateHandler.DeleteLocale, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Adapters
    ws.GET("/adapters", s.adapterHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/adapters", s.adapterHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.GET("/adapters/:id", s.adapterHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.PUT("/adapters/:id", s.adapterHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.DELETE("/adapters/:id", s.adapterHandler.SoftDelete, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Domains
    ws.GET("/domains", s.domainHandler.List, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/domains", s.domainHandler.Register, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.GET("/domains/:id", s.domainHandler.Get, middleware.RequireRole(domain.RoleWorkspaceViewer))
    ws.POST("/domains/:id/verify", s.domainHandler.VerifyNow, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.DELETE("/domains/:id", s.domainHandler.SoftDelete, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // API Keys
    ws.GET("/api-keys", s.apikeyHandler.List, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.POST("/api-keys", s.apikeyHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.DELETE("/api-keys/:id", s.apikeyHandler.Revoke, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Members (workspace scope)
    ws.GET("/members", s.memberHandler.ListWorkspace, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.POST("/members", s.memberHandler.Invite, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.PUT("/members/:id/roles", s.memberHandler.UpdateRoles, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.DELETE("/members/:id/roles/:roleId", s.memberHandler.RemoveRole, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Webhooks
    ws.GET("/webhooks", s.webhookHandler.List, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.POST("/webhooks", s.webhookHandler.Create, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.GET("/webhooks/:id", s.webhookHandler.Get, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.PUT("/webhooks/:id", s.webhookHandler.Update, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.DELETE("/webhooks/:id", s.webhookHandler.Delete, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.POST("/webhooks/:id/test", s.webhookHandler.Test, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Suppression
    ws.GET("/suppression", s.suppressionHandler.List, middleware.RequireRole(domain.RoleWorkspaceAdmin))
    ws.POST("/suppression", s.suppressionHandler.Add, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Audit (scoped)
    ws.GET("/audit-log", s.auditHandler.Query, middleware.RequireRole(domain.RoleWorkspaceAdmin))

    // Test email (dashboard only)
    ws.POST("/test-send", s.sendHandler.TestSend, middleware.RequireRole(domain.RoleWorkspaceEditor))

    // ─── Global Management (superadmin only) ─────────────────
    global := mgmt.Group("/global")
    global.Use(middleware.RequireRole(domain.RoleSuperadmin))

    // Global config
    global.GET("/config", s.configHandler.Get)
    global.PUT("/config", s.configHandler.Update)

    // Global injectors
    global.GET("/injectors", s.injectorHandler.ListGlobal)
    global.POST("/injectors", s.injectorHandler.CreateGlobal)
    global.GET("/injectors/:name", s.injectorHandler.GetGlobal)
    global.PUT("/injectors/:name", s.injectorHandler.UpdateGlobal)
    global.DELETE("/injectors/:name", s.injectorHandler.SoftDeleteGlobal)

    // Global template types
    global.GET("/template-types", s.templateTypeHandler.ListGlobal)
    global.POST("/template-types", s.templateTypeHandler.CreateGlobal)
    global.GET("/template-types/:slug", s.templateTypeHandler.GetGlobal)
    global.PUT("/template-types/:slug", s.templateTypeHandler.UpdateGlobal)
    global.DELETE("/template-types/:slug", s.templateTypeHandler.SoftDeleteGlobal)

    // Global templates
    global.GET("/templates", s.templateHandler.ListGlobal)
    global.POST("/templates", s.templateHandler.CreateGlobal)
    global.GET("/templates/:slug", s.templateHandler.GetGlobal)

    // Global adapters
    global.GET("/adapters", s.adapterHandler.ListGlobal)
    global.POST("/adapters", s.adapterHandler.CreateGlobal)
    global.GET("/adapters/:id", s.adapterHandler.GetGlobal)
    global.PUT("/adapters/:id", s.adapterHandler.UpdateGlobal)
    global.DELETE("/adapters/:id", s.adapterHandler.SoftDeleteGlobal)

    // Global domains
    global.GET("/domains", s.domainHandler.ListGlobal)
    global.POST("/domains", s.domainHandler.RegisterGlobal)
    global.GET("/domains/:id", s.domainHandler.GetGlobal)
    global.DELETE("/domains/:id", s.domainHandler.SoftDeleteGlobal)

    // Global members
    global.GET("/members", s.memberHandler.ListAll)
    global.POST("/members", s.memberHandler.InviteGlobal)

    // Global suppression
    global.GET("/suppression", s.suppressionHandler.ListGlobal)
    global.POST("/suppression", s.suppressionHandler.AddGlobal)
    global.DELETE("/suppression/:email", s.suppressionHandler.RemoveGlobal)

    // Global audit
    global.GET("/audit-log", s.auditHandler.QueryGlobal)

    // Tenant-level members (tenant-admin)
    mgmt.GET("/tenants/:code/members", s.memberHandler.ListTenant, middleware.RequireRole(domain.RoleTenantAdmin))
    mgmt.POST("/tenants/:code/members", s.memberHandler.InviteTenant, middleware.RequireRole(domain.RoleTenantAdmin))
}
```

---

## 16. Background Workers (River)

### 16.1. Send Worker

```go
// adapter/river/send_worker.go
package river

type SendWorker struct {
    emailStore     port.EmailStore
    adapterReg     *service.AdapterRegistry
    compiler       port.TemplateCompiler
    renderer       port.VariableRenderer
    dkimSigner     *dkim.Signer
    suppression    port.SuppressionStore
    webhookService *service.WebhookService
}

type SendJobArgs struct {
    EmailID    uuid.UUID `json:"email_id"`
    TrackingID string    `json:"tracking_id"`
    AdapterID  uuid.UUID `json:"adapter_id"`
}

func (w *SendWorker) Work(ctx context.Context, job *river.Job[SendJobArgs]) error {
    email, err := w.emailStore.GetByTrackingID(ctx, job.Args.TrackingID)
    if err != nil { return err }

    // 1. Mark as processing
    w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusProcessing)
    w.emailStore.AddEvent(ctx, &domain.EmailEvent{
        EmailID: email.ID, EventType: domain.StatusProcessing,
    })

    // 2. Compile MJML → HTML
    // Body MJML is stored in the email record (snapshotted at queue time)
    bodyRendered, _ := w.renderer.Render(email.BodyMJML, email.InjectorsSnapshot, email.VariablesSnapshot)
    bodyHTML, err := w.compiler.Compile(ctx, bodyRendered)
    if err != nil {
        w.markFailed(ctx, email, err)
        return nil // don't retry compile errors
    }

    // 3. Sign with DKIM
    signedMsg, err := w.dkimSigner.Sign(bodyHTML, email.FromEmail)
    if err != nil {
        w.markFailed(ctx, email, err)
        return nil
    }

    // 4. Send via adapter
    adapter := w.adapterReg.Get(job.Args.AdapterID)
    providerMsgID, err := adapter.Send(ctx, &port.OutgoingEmail{
        From:     port.EmailAddress{Name: email.FromName, Address: email.FromEmail},
        To:       port.EmailAddress{Address: email.RecipientEmail},
        CC:       toAddresses(email.CC),
        BCC:      toAddresses(email.BCC),
        Subject:  email.SubjectRendered,
        BodyHTML: signedMsg,
    })

    if err != nil {
        // 5. Handle failure / retry
        if isTransient(err) && email.RetryCount < email.MaxRetries {
            nextRetry := time.Now().Add(backoff(email.RetryCount))
            w.emailStore.UpdateRetry(ctx, email.ID, email.RetryCount+1, &nextRetry)
            return err // River will re-enqueue
        }
        w.markFailed(ctx, email, err)
        return nil
    }

    // 6. Success
    w.emailStore.UpdateStatus(ctx, email.ID, domain.StatusSent)
    w.emailStore.AddEvent(ctx, &domain.EmailEvent{
        EmailID:   email.ID,
        EventType: domain.StatusSent,
        Metadata:  map[string]any{"provider_message_id": providerMsgID},
    })

    return nil
}

func backoff(retryCount int) time.Duration {
    base := 60 * time.Second
    return base * time.Duration(math.Pow(2, float64(retryCount)))
}
```

### 16.2. Domain Verification Worker

```go
// adapter/river/verify_worker.go
package river

type VerifyWorker struct {
    domainStore port.DomainStore
    dnsChecker  *dkim.DNSChecker
}

func (w *VerifyWorker) Work(ctx context.Context, job *river.Job[VerifyJobArgs]) error {
    domain, _ := w.domainStore.GetByID(ctx, job.Args.DomainID)

    // Check DNS records
    valid, details := w.dnsChecker.VerifyAll(ctx, domain.DomainName, domain.DKIMSelector, domain.DKIMPublicKey)

    if valid {
        domain.Status = "verified"
        domain.VerifiedAt = timePtr(time.Now())
        domain.LastError = nil
    } else {
        domain.Status = "error"
        domain.LastError = &details
    }

    domain.LastCheckAt = timePtr(time.Now())
    domain.NextCheckAt = timePtr(time.Now().Add(24 * time.Hour))

    return w.domainStore.Update(ctx, domain)
}
```

### 16.3. Webhook Worker

```go
// adapter/river/webhook_worker.go
package river

type WebhookWorker struct {
    webhookStore port.WebhookStore
    httpClient   *http.Client
    signer       *hmac.Signer // HMAC-SHA256 signing with webhook secret
}

type WebhookJobArgs struct {
    WebhookID  uuid.UUID `json:"webhook_id"`
    EventType  string    `json:"event_type"`  // "email.sent", "email.delivered", etc.
    Payload    []byte    `json:"payload"`      // JSON event payload
    RetryCount int       `json:"retry_count"`
}

func (w *WebhookWorker) Work(ctx context.Context, job *river.Job[WebhookJobArgs]) error {
    webhook, err := w.webhookStore.GetByID(ctx, job.Args.WebhookID)
    if err != nil || !webhook.IsActive {
        return nil // webhook deleted or disabled, discard
    }

    // 1. Sign payload with webhook secret
    timestamp := time.Now().Unix()
    signature := w.signer.Sign(job.Args.Payload, webhook.Secret, timestamp)

    // 2. Build request
    req, _ := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(job.Args.Payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Senda-Event", job.Args.EventType)
    req.Header.Set("X-Senda-Signature", signature)
    req.Header.Set("X-Senda-Timestamp", fmt.Sprintf("%d", timestamp))
    req.Header.Set("X-Senda-Delivery", job.ID.String())

    // 3. Send with timeout
    resp, err := w.httpClient.Do(req)
    if err != nil {
        return w.handleRetry(job, fmt.Errorf("connection error: %w", err))
    }
    defer resp.Body.Close()

    // 4. Evaluate response
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return nil // success
    }

    if resp.StatusCode >= 500 || resp.StatusCode == 429 {
        // Transient error — retry
        return w.handleRetry(job, fmt.Errorf("webhook returned %d", resp.StatusCode))
    }

    // 4xx (except 429) — permanent failure, don't retry
    return nil
}

func (w *WebhookWorker) handleRetry(job *river.Job[WebhookJobArgs], err error) error {
    maxRetries := 5
    if job.Args.RetryCount >= maxRetries {
        // Log permanent failure, don't retry
        slog.Warn("webhook delivery permanently failed",
            "webhook_id", job.Args.WebhookID,
            "event", job.Args.EventType,
            "retries", job.Args.RetryCount,
        )
        return nil
    }
    // Return error so River re-enqueues with exponential backoff
    return err
}
```

### 16.4. Webhook Event Dispatch

```go
// service/webhook_service.go
package service

// WebhookService handles dispatching events to registered webhooks.
type WebhookService struct {
    webhookStore port.WebhookStore
    queue        port.JobQueue
}

// Dispatch sends an event to all active webhooks in the workspace.
// Called by SendWorker, event processing, etc.
func (s *WebhookService) Dispatch(ctx context.Context, workspaceID uuid.UUID, eventType string, payload any) error {
    webhooks, err := s.webhookStore.GetActiveByWorkspace(ctx, workspaceID)
    if err != nil || len(webhooks) == 0 {
        return nil // no webhooks configured — not an error
    }

    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("marshal webhook payload: %w", err)
    }

    for _, wh := range webhooks {
        // Check if webhook subscribes to this event type
        if !wh.SubscribesTo(eventType) {
            continue
        }
        s.queue.EnqueueWebhook(ctx, &port.WebhookJob{
            WebhookID: wh.ID,
            EventType: eventType,
            Payload:   payloadBytes,
        })
    }
    return nil
}
```

---

## 17. Configuration

```go
// config/config.go
package config

type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    OIDC     OIDCConfig     `yaml:"oidc"`
    Crypto   CryptoConfig   `yaml:"crypto"`
    Log      LogConfig      `yaml:"log"`
}
// Note: the cache uses the same PG connection (UNLOGGED table).
// No necesita config separado.

type ServerConfig struct {
    Host            string        `yaml:"host" env:"SENDA_HOST" default:"0.0.0.0"`
    Port            int           `yaml:"port" env:"SENDA_PORT" default:"8080"`
    ReadTimeout     time.Duration `yaml:"read_timeout" default:"30s"`
    WriteTimeout    time.Duration `yaml:"write_timeout" default:"30s"`
    ShutdownTimeout time.Duration `yaml:"shutdown_timeout" default:"15s"`
}

type DatabaseConfig struct {
    URL             string `yaml:"url" env:"SENDA_DATABASE_URL" required:"true"`
    MaxOpenConns    int    `yaml:"max_open_conns" default:"25"`
    MaxIdleConns    int    `yaml:"max_idle_conns" default:"10"`
    ConnMaxLifetime string `yaml:"conn_max_lifetime" default:"5m"`
    MigrateOnStart  bool   `yaml:"migrate_on_start" default:"true"`
}

type OIDCConfig struct {
    DiscoveryURL string `yaml:"discovery_url" env:"SENDA_OIDC_DISCOVERY_URL" required:"true"`
    ClientID     string `yaml:"client_id" env:"SENDA_OIDC_CLIENT_ID" required:"true"`
    ClientSecret string `yaml:"client_secret" env:"SENDA_OIDC_CLIENT_SECRET" required:"true"`
}

// Note: SES/SMTP credentials do not belong in the global config.
// They are stored encrypted per adapter in the DB (table adapters.config_encrypted).

type CryptoConfig struct {
    // Master key for AES-256-GCM encryption of adapter credentials and DKIM keys
    MasterKey string `yaml:"master_key" env:"SENDA_MASTER_KEY" required:"true"`
}

type LogConfig struct {
    Level  string `yaml:"level" default:"info"`
    Format string `yaml:"format" default:"json"` // json or text
}
```

---

## 18. Docker Compose (Desarrollo)

> **Note:** The stack is Go + PostgreSQL only. The cache uses a PG UNLOGGED table, the queue uses River (PG-native), and provider rate limiting uses a PL/pgSQL token bucket. PostgreSQL is the only external dependency required.

```yaml
# docker/docker-compose.yml
version: "3.9"

services:
  senda:
    build:
      context: ..
      dockerfile: docker/Dockerfile.dev
    ports:
      - "8080:8080"
    environment:
      SENDA_DATABASE_URL: postgres://senda:senda@postgres:5432/senda?sslmode=disable
      SENDA_OIDC_DISCOVERY_URL: ${OIDC_DISCOVERY_URL}
      SENDA_OIDC_CLIENT_ID: ${OIDC_CLIENT_ID}
      SENDA_OIDC_CLIENT_SECRET: ${OIDC_CLIENT_SECRET}
      SENDA_MASTER_KEY: ${MASTER_KEY:-dev-master-key-change-in-production}
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ../:/app  # Hot reload in dev

  postgres:
    image: postgres:16-alpine
    command: >
      postgres
        -c shared_preload_libraries=pg_cron
        -c cron.database_name=senda
    environment:
      POSTGRES_USER: senda
      POSTGRES_PASSWORD: senda
      POSTGRES_DB: senda
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U senda"]
      interval: 5s
      timeout: 3s
      retries: 10

  # Optional: Caddy for HTTPS in local dev
  caddy:
    image: caddy:2-alpine
    ports:
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
    profiles:
      - https

volumes:
  pgdata:
```

---

## 19. Provider Event Ingestion (Channel-Agnostic)

The system receives provider events (bounces, deliveries, complaints) through configurable channels. Reception is channel-agnostic; processing is unified.

### 19.1. Arquitectura

```
Provider (SES, SMTP, etc.)
    │
    ▼
┌──────────────────────┐
│  EventChannel (Port)  │  ← channel-agnostic interface
│  - Webhook HTTP       │  ← Phase 1: SES/SNS webhook
│  - SQS Poller         │  ← Phase 2: alternative channel
│  - IMAP Bounce Parser │  ← Phase 2: SMTP bounce emails
└──────────┬───────────┘
           │ Normalized ProviderEvent
           ▼
┌──────────────────────┐
│  EventProcessor       │  ← unified processing
│  - Update email status│
│  - Add email_event    │
│  - Suppression check  │
│  - Dispatch webhooks  │
└──────────────────────┘
```

### 19.2. Port Interface

```go
// port/event_channel.go
package port

// ProviderEvent is the normalized event from any provider channel.
type ProviderEvent struct {
    Type             EventType         // delivered, bounced, complained, opened
    ProviderMessageID string           // maps to emails.provider_message_id
    Timestamp        time.Time
    RawPayload       json.RawMessage   // original payload for debugging
    BounceDetail     *BounceDetail     // only for bounced events
    ComplaintDetail  *ComplaintDetail   // only for complained events
}

type EventType string

const (
    EventDelivered  EventType = "delivered"
    EventBounced    EventType = "bounced"
    EventComplained EventType = "complained"
    EventOpened     EventType = "opened"
)

type BounceDetail struct {
    BounceType    string // "hard", "soft"
    DiagnosticCode string
    Recipients    []string
}

type ComplaintDetail struct {
    ComplaintType string // "abuse"
    FeedbackID    string
    Recipients    []string
}
```

### 19.3. SES Webhook Handler (Phase 1)

```go
// http/handler/ses_webhook.go
package handler

// SESWebhookHandler handles SNS notifications from SES.
// SES → SNS Topic → HTTP POST to /api/v1/webhooks/ses/inbound
type SESWebhookHandler struct {
    processor *service.EventProcessor
    snsVerifier *sns.SignatureVerifier // verify SNS message authenticity
}

func (h *SESWebhookHandler) Handle(c echo.Context) error {
    // 1. Verify SNS signature (prevents spoofed events)
    body, _ := io.ReadAll(c.Request().Body)
    if err := h.snsVerifier.Verify(body); err != nil {
        return echo.NewHTTPError(403, "invalid SNS signature")
    }

    // 2. Parse SNS envelope
    var snsMsg sns.Message
    json.Unmarshal(body, &snsMsg)

    // 3. Handle SNS subscription confirmation (first-time setup)
    if snsMsg.Type == "SubscriptionConfirmation" {
        return h.confirmSubscription(c, snsMsg)
    }

    // 4. Parse SES event from SNS message
    events, err := parseSESNotification(snsMsg.Message)
    if err != nil {
        slog.Warn("failed to parse SES notification", "error", err)
        return c.NoContent(200) // ack to prevent retries
    }

    // 5. Process each event
    for _, event := range events {
        if err := h.processor.Process(c.Request().Context(), event); err != nil {
            slog.Error("failed to process provider event",
                "type", event.Type,
                "provider_msg_id", event.ProviderMessageID,
                "error", err,
            )
        }
    }

    return c.NoContent(200)
}
```

### 19.4. EventProcessor

```go
// service/event_processor.go
package service

type EventProcessor struct {
    emailStore      port.EmailStore
    suppressionStore port.SuppressionStore
    webhookService  *WebhookService
}

func (p *EventProcessor) Process(ctx context.Context, event *port.ProviderEvent) error {
    // 1. Find email by provider message ID
    email, err := p.emailStore.GetByProviderMessageID(ctx, event.ProviderMessageID)
    if err != nil {
        return fmt.Errorf("email not found for provider_msg_id %s: %w", event.ProviderMessageID, err)
    }

    // 2. Map to internal status
    var newStatus domain.EmailStatus
    switch event.Type {
    case port.EventDelivered:
        newStatus = domain.StatusDelivered
    case port.EventBounced:
        newStatus = domain.StatusBounced
    case port.EventComplained:
        newStatus = domain.StatusComplained
    case port.EventOpened:
        newStatus = domain.StatusOpened
    }

    // 3. Update email status + add event
    p.emailStore.UpdateStatus(ctx, email.ID, newStatus)
    p.emailStore.AddEvent(ctx, &domain.EmailEvent{
        ID:         uuid.New(),
        EmailID:    email.ID,
        EventType:  newStatus,
        OccurredAt: event.Timestamp,
        Metadata:   mapEventMetadata(event),
    })

    // 4. Handle suppression side effects
    switch event.Type {
    case port.EventBounced:
        if event.BounceDetail != nil && event.BounceDetail.BounceType == "hard" {
            // Hard bounce → global suppression
            p.suppressionStore.AddGlobal(ctx, &domain.SuppressionGlobal{
                Email:  email.RecipientEmail,
                Reason: domain.SuppressionBounce,
                Source: "provider_webhook",
            })
        }
    case port.EventComplained:
        // Complaint → workspace suppression
        p.suppressionStore.AddWorkspace(ctx, &domain.SuppressionWorkspace{
            WorkspaceID: email.WorkspaceID,
            Email:       email.RecipientEmail,
            Reason:      domain.SuppressionComplaint,
            Source:      "provider_webhook",
        })
    }

    // 5. Dispatch to workspace webhooks
    p.webhookService.Dispatch(ctx, email.WorkspaceID, "email."+string(event.Type), map[string]any{
        "tracking_id":   email.TrackingID,
        "external_id":   email.ExternalID,
        "recipient":     email.RecipientEmail,
        "status":        newStatus,
        "occurred_at":   event.Timestamp,
    })

    return nil
}
```

### 19.5. Route Registration

```go
// In registerRoutes():
// Provider event ingestion (no auth — uses signature verification)
e.POST("/api/v1/webhooks/ses/inbound", s.sesWebhookHandler.Handle)
// Future: e.POST("/api/v1/webhooks/sendgrid/inbound", s.sendgridHandler.Handle)
```

---

## 20. Onboarding Flow

### 20.1. Bootstrap: Primer Superadmin

When Senda starts for the first time, the DB is empty (there are no members). The first user to log in with OIDC automatically becomes a superadmin.

```go
// service/onboarding.go
package service

type OnboardingService struct {
    memberStore  port.MemberStore
    tenantStore  port.TenantStore
    auditStore   port.AuditLogStore
}

type OnboardingRequest struct {
    TenantCode string `json:"tenant_code" validate:"required,min=2,max=50"`
    TenantName string `json:"tenant_name" validate:"required"`
}

// Setup handles the onboarding wizard (POST /api/v1/manage/onboarding).
// Only works when the system has zero members.
func (s *OnboardingService) Setup(ctx context.Context, oidcClaims *OIDCClaims, req *OnboardingRequest) error {
    // 1. Guard: only allowed if no members exist
    count, _ := s.memberStore.CountAll(ctx)
    if count > 0 {
        return apperr.Forbidden("onboarding already completed — system has members")
    }

    // 2. Create the first member (from OIDC token)
    member := &domain.Member{
        ID:    uuid.New(),
        Email: oidcClaims.Email,
        Name:  &oidcClaims.Name,
    }
    s.memberStore.Create(ctx, member)

    // 3. Assign superadmin role (global scope)
    s.memberStore.AddRole(ctx, &domain.MemberRole{
        ID:        uuid.New(),
        MemberID:  member.ID,
        Role:      domain.RoleSuperadmin,
        ScopeType: domain.ScopeGlobal,
    })

    // 4. Create first tenant
    tenant := &domain.Tenant{
        ID:   uuid.New(),
        Code: req.TenantCode,
        Name: req.TenantName,
    }
    s.tenantStore.Create(ctx, tenant)

    // 5. Auto-create _system workspace for the tenant
    s.tenantStore.CreateSystemWorkspace(ctx, tenant.ID)

    // 6. Assign tenant_admin role
    s.memberStore.AddRole(ctx, &domain.MemberRole{
        ID:        uuid.New(),
        MemberID:  member.ID,
        Role:      domain.RoleTenantAdmin,
        ScopeType: domain.ScopeTenant,
        TenantID:  &tenant.ID,
    })

    // 7. Audit
    s.auditStore.Append(ctx, &domain.AuditLog{
        ActorID:    member.ID,
        ActorEmail: member.Email,
        Action:     domain.AuditCreate,
        EntityType: "system",
        EntityID:   member.ID,
        Detail:     map[string]any{"event": "onboarding_completed", "tenant": req.TenantCode},
    })

    return nil
}

// Status returns whether onboarding is needed (GET /api/v1/manage/onboarding/status).
// This endpoint requires NO auth — it must work before any member exists.
func (s *OnboardingService) Status(ctx context.Context) (*OnboardingStatus, error) {
    count, _ := s.memberStore.CountAll(ctx)
    return &OnboardingStatus{
        NeedsOnboarding: count == 0,
    }, nil
}

type OnboardingStatus struct {
    NeedsOnboarding bool `json:"needs_onboarding"`
}
```

**Note:** The `GET /onboarding/status` endpoint is public (no auth). `POST /onboarding` requires a valid OIDC token but does NOT require the user to already be a member — it is the exception to the standard auth middleware.

---

## 21. Observability

### 21.1. Structured Logging (slog)

`slog` is the standard for the entire system. JSON in production, text in development.

```go
// cmd/main.go — logger setup
func setupLogger(cfg config.LogConfig) {
    var handler slog.Handler
    if cfg.Format == "json" {
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.Level)})
    } else {
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.Level)})
    }
    slog.SetDefault(slog.New(handler))
}
```

**Logging conventions:**

```go
// Cada log entry incluye contexto estructurado
slog.Info("email sent",
    "tracking_id", email.TrackingID,
    "workspace_id", email.WorkspaceID,
    "adapter", adapter.Name,
    "recipient", email.RecipientEmail,
    "duration_ms", elapsed.Milliseconds(),
)

slog.Error("adapter send failed",
    "tracking_id", email.TrackingID,
    "adapter", adapter.Name,
    "error", err,
    "retry_count", email.RetryCount,
)
```

### 21.2. Metrics Endpoint (Prometheus)

```go
// http/handler/metrics.go
package handler

import "github.com/prometheus/client_golang/prometheus/promhttp"

// In registerRoutes():
e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
```

**Exposed metrics:**

```go
// internal/metrics/metrics.go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    // Email send metrics
    EmailsSent = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "senda_emails_sent_total",
            Help: "Total emails sent by status",
        },
        []string{"status", "adapter", "tenant", "workspace"},
    )

    EmailSendDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "senda_email_send_duration_seconds",
            Help:    "Time to process email send",
            Buckets: prometheus.DefBuckets,
        },
        []string{"adapter"},
    )

    // HTTP request metrics
    HTTPRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "senda_http_request_duration_seconds",
            Help:    "HTTP request latency",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
        },
        []string{"method", "path", "status"},
    )

    HTTPRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "senda_http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    // Queue metrics
    QueueDepth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "senda_queue_depth",
            Help: "Number of pending jobs in queue",
        },
        []string{"queue"},
    )

    // Provider metrics
    ProviderErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "senda_provider_errors_total",
            Help: "Errors from email providers",
        },
        []string{"adapter", "error_type"},
    )

    // Bounce/complaint rates
    BounceRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "senda_bounce_rate",
            Help: "Bounce rate in last 24h window",
        },
        []string{"tenant", "workspace", "bounce_type"},
    )
)

func Register() {
    prometheus.MustRegister(
        EmailsSent, EmailSendDuration,
        HTTPRequestDuration, HTTPRequestsTotal,
        QueueDepth, ProviderErrors, BounceRate,
    )
}
```

### 21.3. Health Check

```go
// http/handler/health.go
package handler

type HealthHandler struct {
    db    *sql.DB
    queue *river.Client[pgx.Tx]
}

func (h *HealthHandler) Health(c echo.Context) error {
    checks := map[string]string{}

    // Database
    if err := h.db.PingContext(c.Request().Context()); err != nil {
        checks["database"] = "unhealthy: " + err.Error()
    } else {
        checks["database"] = "ok"
    }

    // Queue (River uses PG, so if DB is ok, queue is ok)
    checks["queue"] = "ok"

    // Overall
    status := 200
    for _, v := range checks {
        if v != "ok" {
            status = 503
            break
        }
    }

    return c.JSON(status, map[string]any{
        "status": map[int]string{200: "healthy", 503: "unhealthy"}[status],
        "checks": checks,
    })
}
```

---

## 22. DKIM Signing

### 22.1. Key Generation (al registrar dominio)

```go
// internal/dkim/keygen.go
package dkim

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
)

// GenerateKeyPair creates a 2048-bit RSA key pair for DKIM signing.
func GenerateKeyPair() (privateKeyPEM []byte, publicKeyBase64 string, err error) {
    privKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, "", err
    }

    // Private key → PEM
    privPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(privKey),
    })

    // Public key → base64 (for DNS TXT record)
    pubASN1, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
    pubBase64 := base64.StdEncoding.EncodeToString(pubASN1)

    return privPEM, pubBase64, nil
}
```

### 22.2. Signing in SendWorker

```go
// internal/dkim/signer.go
package dkim

import (
    "github.com/emersion/go-msgauth/dkim"
)

type Signer struct {
    domainStore port.DomainStore
    crypto      *crypto.Service // AES-256-GCM decrypt
}

// Sign adds DKIM signature headers to the email message.
func (s *Signer) Sign(ctx context.Context, rawMsg []byte, fromEmail string) ([]byte, error) {
    emailDomain := extractDomain(fromEmail)

    // 1. Get domain with decrypted private key
    dom, err := s.domainStore.GetVerifiedByDomain(ctx, emailDomain)
    if err != nil {
        return nil, fmt.Errorf("no verified domain for %s: %w", emailDomain, err)
    }

    // 2. Decrypt private key
    privKeyPEM, err := s.crypto.Decrypt(dom.DKIMPrivateKeyEncrypted)
    if err != nil {
        return nil, fmt.Errorf("decrypt DKIM key: %w", err)
    }

    // 3. Parse PEM → RSA key
    block, _ := pem.Decode(privKeyPEM)
    privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
    if err != nil {
        return nil, err
    }

    // 4. Sign with go-msgauth
    opts := &dkim.SignOptions{
        Domain:   emailDomain,
        Selector: dom.DKIMSelector,
        Signer:   privKey,
        Hash:     crypto.SHA256,
        HeaderKeys: []string{
            "From", "To", "Subject", "Date",
            "MIME-Version", "Content-Type",
        },
    }

    var signed bytes.Buffer
    err = dkim.Sign(&signed, bytes.NewReader(rawMsg), opts)
    return signed.Bytes(), err
}
```

---

## 23. Distributed Cache in PostgreSQL (No Redis)

### 23.1. Architectural Decision

Redis was removed from the stack. The cache is implemented with a **UNLOGGED table** in PostgreSQL.

**Rationale:**
- **Distributed:** unlike in-memory, the PG cache is shared if Senda runs multiple instances.
- **Sin infra adicional:** PG ya es dependencia obligatoria. No se agrega nada.
- **Crash-safe for cache:** if PG crashes, the UNLOGGED table is automatically emptied — expected behavior for a cache.
- **Sufficient performance:** UNLOGGED tables reach ~485K TPS writes. For chain and adapter resolution (tens of req/s in the worst case) this is more than enough.
- **Future:** if more cache throughput is needed, an alternative adapter (e.g. Valkey) can be implemented without changing the ports.

**Referencia:** [Martin Heinz — PostgreSQL as a Cache](https://martinheinz.dev/blog/105), [Tiger Data — Just Use Postgres (2026)](https://www.tigerdata.com/blog/its-2026-just-use-postgres), [Replace Redis with PostgreSQL UNLOGGED Tables](https://medium.com/@tihomir.manushev/replace-redis-with-postgresql-6c11f4666f23)

### 23.2. Schema

```sql
-- Cache table: UNLOGGED = no WAL overhead → 2-3x faster writes.
-- If PG crashes, the table is automatically emptied (acceptable for a cache).
CREATE UNLOGGED TABLE cache (
    key         VARCHAR(512) PRIMARY KEY,
    value       JSONB NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cache_expires ON cache (expires_at);

-- Cleanup of expired entries:
-- Option A (pg_cron installed): every minute
--   SELECT cron.schedule('cache-cleanup', '* * * * *',
--       $$DELETE FROM cache WHERE expires_at < now()$$);
--
-- Option B (without pg_cron): a goroutine with time.Ticker in the app.
```

### 23.3. Cache Port Implementation

```go
// adapter/cache/postgres.go
package cache

// PGCache implements port.Cache using a PostgreSQL UNLOGGED table.
// Thread-safe, distributed (shared across instances), crash-tolerant.
type PGCache struct {
    db *sql.DB
}

func NewPGCache(db *sql.DB) *PGCache {
    return &PGCache{db: db}
}

func (c *PGCache) Get(ctx context.Context, key string) ([]byte, error) {
    var value json.RawMessage
    err := c.db.QueryRowContext(ctx, `
        SELECT value FROM cache
        WHERE key = $1 AND expires_at > now()
    `, key).Scan(&value)

    if err == sql.ErrNoRows {
        return nil, ErrCacheMiss
    }
    return value, err
}

func (c *PGCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
    expiresAt := time.Now().Add(ttl)
    _, err := c.db.ExecContext(ctx, `
        INSERT INTO cache (key, value, expires_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (key)
        DO UPDATE SET value = EXCLUDED.value,
                      expires_at = EXCLUDED.expires_at,
                      created_at = now()
    `, key, value, expiresAt)
    return err
}

func (c *PGCache) Delete(ctx context.Context, key string) error {
    _, err := c.db.ExecContext(ctx, `DELETE FROM cache WHERE key = $1`, key)
    return err
}

func (c *PGCache) DeletePattern(ctx context.Context, pattern string) error {
    // Converts glob "chain:*" → SQL LIKE "chain:%"
    likePattern := strings.ReplaceAll(pattern, "*", "%")
    _, err := c.db.ExecContext(ctx, `DELETE FROM cache WHERE key LIKE $1`, likePattern)
    return err
}

// StartCleanup starts periodic cleanup of expired entries.
// Use if pg_cron is not available.
func (c *PGCache) StartCleanup(ctx context.Context, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                c.db.ExecContext(ctx, `DELETE FROM cache WHERE expires_at < now()`)
            }
        }
    }()
}
```

---

## 24. Provider Rate Limiting (Token Bucket in PG)

### 24.1. Problem

Providers (SES, SMTP relay) have send limits (e.g. SES = 14 emails/sec in sandbox, scalable). If Senda exceeds those limits, the provider blocks the account.

### 24.2. Approach: Token Bucket in PL/pgSQL

The **Token Bucket** algorithm is implemented as a PL/pgSQL function. A single `SELECT take_send_token(adapter_id)` call returns `true/false`. It is atomic, thread-safe, and does not need additional infrastructure.

**Reference:** [Neon — Rate Limiting in Postgres](https://neon.com/guides/rate-limiting). The implementation is custom PL/pgSQL (it does not depend on external libraries).

### 24.3. Schema

```sql
-- Rate limit configuration per adapter
-- The admin configures the limit when creating/editing the adapter.
-- Read from `adapters.rate_limit_per_second` (new field).
ALTER TABLE adapters ADD COLUMN rate_limit_per_second INT NOT NULL DEFAULT 14;

-- Bucket table: one row per adapter, tracking available tokens.
CREATE UNLOGGED TABLE token_buckets (
    adapter_id      UUID PRIMARY KEY REFERENCES adapters(id),
    tokens          FLOAT NOT NULL,           -- tokens actuales
    max_tokens      INT NOT NULL,             -- bucket capacity
    refill_rate     FLOAT NOT NULL,           -- tokens per second
    last_refill     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 24.4. PL/pgSQL Function: Token Bucket

```sql
-- take_send_token: attempts to consume one token for the given adapter.
-- Returns TRUE if sending is allowed, FALSE if the caller must wait.
-- Atomic and thread-safe (implicit row-level lock in UPDATE).
CREATE OR REPLACE FUNCTION take_send_token(p_adapter_id UUID)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    v_tokens FLOAT;
    v_max_tokens INT;
    v_refill_rate FLOAT;
    v_last_refill TIMESTAMPTZ;
    v_now TIMESTAMPTZ := now();
    v_elapsed FLOAT;
    v_new_tokens FLOAT;
BEGIN
    -- Lock the row and read current state
    SELECT tokens, max_tokens, refill_rate, last_refill
    INTO v_tokens, v_max_tokens, v_refill_rate, v_last_refill
    FROM token_buckets
    WHERE adapter_id = p_adapter_id
    FOR UPDATE;

    -- If no bucket exists, create one (first use)
    IF NOT FOUND THEN
        -- Read rate from adapter config
        SELECT rate_limit_per_second INTO v_max_tokens
        FROM adapters WHERE id = p_adapter_id;

        IF NOT FOUND THEN
            RETURN FALSE;
        END IF;

        INSERT INTO token_buckets (adapter_id, tokens, max_tokens, refill_rate, last_refill)
        VALUES (p_adapter_id, v_max_tokens - 1, v_max_tokens, v_max_tokens, v_now);
        RETURN TRUE;
    END IF;

    -- Calculate refilled tokens since last check
    v_elapsed := EXTRACT(EPOCH FROM (v_now - v_last_refill));
    v_new_tokens := LEAST(v_max_tokens, v_tokens + (v_elapsed * v_refill_rate));

    -- Try to take one token
    IF v_new_tokens >= 1.0 THEN
        UPDATE token_buckets
        SET tokens = v_new_tokens - 1.0,
            last_refill = v_now
        WHERE adapter_id = p_adapter_id;
        RETURN TRUE;
    ELSE
        -- No tokens available, update refill state but don't consume
        UPDATE token_buckets
        SET tokens = v_new_tokens,
            last_refill = v_now
        WHERE adapter_id = p_adapter_id;
        RETURN FALSE;
    END IF;
END;
$$;
```

### 24.5. Go Integration

```go
// internal/ratelimit/provider_limiter.go
package ratelimit

// ProviderRateLimiter uses PG token bucket function for provider throttling.
type ProviderRateLimiter struct {
    db *sql.DB
}

// TryAcquire attempts to consume one send token for the adapter.
// Returns true if allowed, false if throttled. Single PG call, atomic.
func (l *ProviderRateLimiter) TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error) {
    var allowed bool
    err := l.db.QueryRowContext(ctx,
        `SELECT take_send_token($1)`, adapterID,
    ).Scan(&allowed)
    return allowed, err
}

// SyncBucket updates the bucket config when adapter rate limit changes.
func (l *ProviderRateLimiter) SyncBucket(ctx context.Context, adapterID uuid.UUID, maxPerSecond int) error {
    _, err := l.db.ExecContext(ctx, `
        INSERT INTO token_buckets (adapter_id, tokens, max_tokens, refill_rate, last_refill)
        VALUES ($1, $2, $2, $2, now())
        ON CONFLICT (adapter_id)
        DO UPDATE SET max_tokens = $2, refill_rate = $2
    `, adapterID, maxPerSecond)
    return err
}
```

### 24.6. SendWorker Integration

```go
// In SendWorker.Work(), before calling the adapter:
allowed, err := w.rateLimiter.TryAcquire(ctx, job.Args.AdapterID)
if !allowed {
    // Re-enqueue with a short delay — River JobSnooze puts the job on hold
    return river.JobSnooze(1 * time.Second)
}
```

**Note about API rate limiting:** Removed from Phase 1. The product is self-hosted, and the admin controls access. It can be added as an optional feature if there is demand.

---

*Next step: Build the Technical Stories (HTs) based on the PRD v5 + Tech Spec.*
