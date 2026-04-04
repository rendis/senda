CREATE TABLE adapter_workspace_grants (
    adapter_id     UUID NOT NULL REFERENCES adapters(id) ON DELETE CASCADE,
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (adapter_id, workspace_id)
);

CREATE INDEX idx_awg_workspace ON adapter_workspace_grants (workspace_id, adapter_id);

CREATE TABLE adapter_identity_workspace_grants (
    adapter_identity_id   UUID NOT NULL REFERENCES adapter_identities(id) ON DELETE CASCADE,
    workspace_id          UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (adapter_identity_id, workspace_id)
);

CREATE INDEX idx_aiwg_workspace ON adapter_identity_workspace_grants (workspace_id, adapter_identity_id);

ALTER TABLE emails
    ADD COLUMN sender_identity_id UUID;

CREATE INDEX idx_emails_sender_identity
    ON emails (sender_identity_id, created_at DESC)
    WHERE sender_identity_id IS NOT NULL;
