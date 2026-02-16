CREATE TABLE webhooks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspaces(id),
    url                 VARCHAR(2048) NOT NULL,
    secret              VARCHAR(128) NOT NULL,
    events              JSONB NOT NULL DEFAULT '["sent","delivered","bounced","complained","failed"]',
    is_active           BOOLEAN NOT NULL DEFAULT true,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_at     TIMESTAMPTZ,
    disabled_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhooks_workspace ON webhooks (workspace_id) WHERE is_active = true;
