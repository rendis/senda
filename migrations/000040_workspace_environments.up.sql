ALTER TABLE workspaces
    ADD COLUMN logical_workspace_id UUID,
    ADD COLUMN environment VARCHAR(10);

UPDATE workspaces
SET logical_workspace_id = id
WHERE logical_workspace_id IS NULL;

UPDATE workspaces
SET environment = 'prod'
WHERE environment IS NULL;

ALTER TABLE workspaces
    DROP CONSTRAINT workspaces_code_per_tenant,
    DROP CONSTRAINT workspaces_one_system_per_tenant;

DROP INDEX IF EXISTS idx_workspaces_code;
DROP INDEX IF EXISTS idx_workspaces_active_code_lookup;

INSERT INTO workspaces (
    id,
    logical_workspace_id,
    tenant_id,
    code,
    name,
    environment,
    is_system,
    is_active,
    open_tracking_enabled,
    default_locale,
    allow_workspace_local_templates,
    allow_workspace_inherited_template_forks,
    allow_workspace_local_injectors,
    created_at,
    updated_at,
    deleted_at
)
SELECT
    gen_random_uuid(),
    logical_workspace_id,
    tenant_id,
    code,
    name,
    'test',
    is_system,
    is_active,
    open_tracking_enabled,
    default_locale,
    allow_workspace_local_templates,
    allow_workspace_inherited_template_forks,
    allow_workspace_local_injectors,
    created_at,
    updated_at,
    deleted_at
FROM workspaces
WHERE environment = 'prod';

ALTER TABLE workspaces
    ALTER COLUMN logical_workspace_id SET NOT NULL,
    ALTER COLUMN environment SET NOT NULL;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_environment_valid CHECK (environment IN ('prod', 'test')),
    ADD CONSTRAINT workspaces_code_per_tenant_environment UNIQUE (tenant_id, code, environment),
    ADD CONSTRAINT workspaces_one_system_per_tenant_environment
        EXCLUDE USING btree (tenant_id WITH =, environment WITH =)
        WHERE (is_system = true AND deleted_at IS NULL);

CREATE INDEX idx_workspaces_code
    ON workspaces (tenant_id, code, environment)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_workspaces_active_code_lookup
    ON workspaces (tenant_id, code, environment)
    WHERE deleted_at IS NULL AND is_active = true;

CREATE INDEX idx_workspaces_logical_workspace
    ON workspaces (logical_workspace_id, environment)
    WHERE deleted_at IS NULL;

ALTER TABLE api_keys
    ALTER COLUMN key_prefix SET DEFAULT 'senda_prod';
