CREATE TYPE identity_type AS ENUM ('email', 'domain');
CREATE TYPE identity_status AS ENUM ('verified', 'pending', 'failed');

CREATE TABLE adapter_identities (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id        UUID NOT NULL REFERENCES adapters(id) ON DELETE CASCADE,
    identity          VARCHAR(255) NOT NULL,
    identity_type     identity_type NOT NULL,
    status            identity_status NOT NULL DEFAULT 'pending',
    sending_enabled   BOOLEAN NOT NULL DEFAULT false,
    is_default        BOOLEAN NOT NULL DEFAULT false,
    display_name      VARCHAR(255),
    source            VARCHAR(20) NOT NULL DEFAULT 'manual',
    last_synced_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT ai_unique UNIQUE (adapter_id, identity),
    CONSTRAINT ai_one_default
        EXCLUDE USING btree (adapter_id WITH =) WHERE (is_default = true)
);

CREATE INDEX idx_ai_adapter ON adapter_identities (adapter_id);
CREATE INDEX idx_ai_adapter_verified ON adapter_identities (adapter_id)
    WHERE status = 'verified' AND sending_enabled = true;
