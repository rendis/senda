CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id),
    key_prefix      VARCHAR(20) NOT NULL DEFAULT 'senda_live',
    key_hash        VARCHAR(128) NOT NULL,
    key_hint        VARCHAR(8) NOT NULL,
    name            VARCHAR(255),
    created_by      UUID NOT NULL REFERENCES members(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,

    CONSTRAINT api_keys_hash_unique UNIQUE (key_hash)
);

CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_keys_workspace ON api_keys (workspace_id) WHERE revoked_at IS NULL;
