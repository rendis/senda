CREATE TABLE adapters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    workspace_id    UUID REFERENCES workspaces(id),
    adapter_type    adapter_type NOT NULL,
    config_encrypted BYTEA NOT NULL,
    is_default      BOOLEAN NOT NULL DEFAULT false,
    rate_limit_per_second INT NOT NULL DEFAULT 14,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT adapters_one_default_per_scope
        EXCLUDE USING btree (workspace_id WITH =) WHERE (is_default = true AND deleted_at IS NULL)
);

CREATE UNIQUE INDEX idx_adapters_global_default
    ON adapters ((true)) WHERE workspace_id IS NULL AND is_default = true AND deleted_at IS NULL;

CREATE INDEX idx_adapters_workspace ON adapters (workspace_id) WHERE deleted_at IS NULL;
