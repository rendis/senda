CREATE INDEX idx_workspace_resolution
    ON workspaces (tenant_id, code)
    INCLUDE (id, is_system)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_template_resolution
    ON templates (template_type_id, workspace_id)
    INCLUDE (id, is_disabled)
    WHERE deleted_at IS NULL;
