DROP INDEX IF EXISTS idx_templates_origin_template_id;

ALTER TABLE templates
    DROP COLUMN IF EXISTS origin_template_id,
    DROP COLUMN IF EXISTS is_fork;

ALTER TABLE workspaces
    DROP COLUMN IF EXISTS allow_workspace_local_injectors,
    DROP COLUMN IF EXISTS allow_workspace_inherited_template_forks,
    DROP COLUMN IF EXISTS allow_workspace_local_templates;
