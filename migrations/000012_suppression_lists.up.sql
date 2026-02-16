CREATE TABLE suppression_global (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    source_email_id UUID,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at      TIMESTAMPTZ,
    removed_by      UUID,
    removal_reason  TEXT,

    CONSTRAINT suppression_global_email_unique UNIQUE (email)
);

CREATE INDEX idx_suppression_global_email ON suppression_global (email) WHERE removed_at IS NULL;

CREATE TABLE suppression_workspace (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    email           VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    source_email_id UUID,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at      TIMESTAMPTZ,
    removed_by      UUID,
    removal_reason  TEXT,

    CONSTRAINT suppression_ws_unique UNIQUE (workspace_id, email)
);

CREATE INDEX idx_suppression_ws_lookup ON suppression_workspace (workspace_id, email) WHERE removed_at IS NULL;
