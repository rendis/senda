CREATE TYPE provisioning_step_status AS ENUM ('pending', 'completed', 'failed');

CREATE TABLE adapter_provisioning_steps (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_id    UUID NOT NULL REFERENCES adapters(id) ON DELETE CASCADE,
    step_name     VARCHAR(50) NOT NULL,
    step_order    SMALLINT NOT NULL,
    status        provisioning_step_status NOT NULL DEFAULT 'pending',
    resource_name VARCHAR(255),
    resource_arn  VARCHAR(512),
    error_message TEXT,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT aps_unique_step UNIQUE (adapter_id, step_name)
);

CREATE INDEX idx_aps_adapter ON adapter_provisioning_steps (adapter_id);
