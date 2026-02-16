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
