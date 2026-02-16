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
