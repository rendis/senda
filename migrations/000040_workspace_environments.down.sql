ALTER TABLE api_keys
    ALTER COLUMN key_prefix SET DEFAULT 'senda_live';

DROP INDEX IF EXISTS idx_workspaces_logical_workspace;
DROP INDEX IF EXISTS idx_workspaces_active_code_lookup;
DROP INDEX IF EXISTS idx_workspaces_code;

ALTER TABLE workspaces
    DROP CONSTRAINT IF EXISTS workspaces_one_system_per_tenant_environment,
    DROP CONSTRAINT IF EXISTS workspaces_code_per_tenant_environment,
    DROP CONSTRAINT IF EXISTS workspaces_environment_valid;

DELETE FROM workspaces
WHERE environment = 'test';

ALTER TABLE workspaces
    DROP COLUMN IF EXISTS environment,
    DROP COLUMN IF EXISTS logical_workspace_id;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_code_per_tenant UNIQUE (tenant_id, code),
    ADD CONSTRAINT workspaces_one_system_per_tenant
        EXCLUDE USING btree (tenant_id WITH =)
        WHERE (is_system = true AND deleted_at IS NULL);

CREATE INDEX idx_workspaces_code
    ON workspaces (tenant_id, code)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_workspaces_active_code_lookup
    ON workspaces (tenant_id, code)
    WHERE deleted_at IS NULL AND is_active = true;
