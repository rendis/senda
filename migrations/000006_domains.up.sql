CREATE TABLE domains (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            UUID REFERENCES workspaces(id),
    domain_name             VARCHAR(255) NOT NULL,
    status                  domain_status NOT NULL DEFAULT 'pending',
    dkim_selector           VARCHAR(63) NOT NULL,
    dkim_private_key_encrypted BYTEA NOT NULL,
    dkim_public_key         TEXT NOT NULL,
    dns_records             JSONB NOT NULL DEFAULT '[]',
    verified_at             TIMESTAMPTZ,
    last_check_at           TIMESTAMPTZ,
    next_check_at           TIMESTAMPTZ,
    last_error              TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ,

    CONSTRAINT domains_unique_per_scope
        UNIQUE NULLS NOT DISTINCT (domain_name, workspace_id)
);

CREATE INDEX idx_domains_workspace ON domains (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_domains_pending ON domains (next_check_at) WHERE status != 'verified' AND deleted_at IS NULL;
CREATE INDEX idx_domains_recheck ON domains (next_check_at) WHERE deleted_at IS NULL;
