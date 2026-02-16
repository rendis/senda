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
