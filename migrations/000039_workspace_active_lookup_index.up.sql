CREATE INDEX idx_workspaces_active_code_lookup
    ON workspaces (tenant_id, code)
    WHERE deleted_at IS NULL AND is_active = true;
