ALTER TABLE workspaces
    ADD COLUMN allow_workspace_local_templates BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN allow_workspace_inherited_template_forks BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN allow_workspace_local_injectors BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE templates
    ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN origin_template_id UUID REFERENCES templates(id);

CREATE INDEX idx_templates_origin_template_id
    ON templates (origin_template_id)
    WHERE origin_template_id IS NOT NULL AND deleted_at IS NULL;
