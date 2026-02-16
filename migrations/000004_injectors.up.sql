CREATE TABLE injector_definitions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    workspace_id    UUID REFERENCES workspaces(id),
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT injector_def_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (name, workspace_id)
);

CREATE INDEX idx_injector_defs_workspace ON injector_definitions (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_injector_defs_global ON injector_definitions (name) WHERE workspace_id IS NULL AND deleted_at IS NULL;

CREATE TABLE injector_fields (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    injector_definition_id  UUID NOT NULL REFERENCES injector_definitions(id) ON DELETE CASCADE,
    field_name              VARCHAR(100) NOT NULL,
    field_type              injector_field_type NOT NULL,
    description             TEXT,
    position                INT NOT NULL DEFAULT 0,

    CONSTRAINT injector_field_unique
        UNIQUE (injector_definition_id, field_name)
);

CREATE TABLE injector_values (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    injector_definition_id  UUID NOT NULL REFERENCES injector_definitions(id),
    field_name              VARCHAR(100) NOT NULL,
    workspace_id            UUID REFERENCES workspaces(id),
    value                   JSONB NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT injector_value_unique
        UNIQUE NULLS NOT DISTINCT (injector_definition_id, field_name, workspace_id)
);

CREATE INDEX idx_injector_values_lookup
    ON injector_values (injector_definition_id, workspace_id);
