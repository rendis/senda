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
