CREATE TABLE emails (
    id                      UUID NOT NULL DEFAULT gen_random_uuid(),
    tracking_id             VARCHAR(32) NOT NULL,
    external_id             VARCHAR(255),
    workspace_id            UUID NOT NULL,
    tenant_id               UUID NOT NULL,
    template_id             UUID,
    template_version_id     UUID,
    template_type_slug      VARCHAR(100) NOT NULL,
    template_ref            VARCHAR(255) NOT NULL,
    recipient_email         VARCHAR(255) NOT NULL,
    cc                      JSONB,
    bcc                     JSONB,
    from_email              VARCHAR(255) NOT NULL,
    from_name               VARCHAR(255),
    reply_to                VARCHAR(255),
    subject_rendered        TEXT NOT NULL,
    body_mjml               TEXT,
    locale                  VARCHAR(10),
    status                  email_status NOT NULL DEFAULT 'queued',
    adapter_id              UUID,
    provider_message_id     VARCHAR(512),
    variables_snapshot      JSONB,
    injectors_snapshot      JSONB,
    retry_count             INT NOT NULL DEFAULT 0,
    max_retries             INT NOT NULL DEFAULT 3,
    next_retry_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Initial partitions (3 months, pg_cron creates the rest)
CREATE TABLE emails_2026_01 PARTITION OF emails
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE emails_2026_02 PARTITION OF emails
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE emails_2026_03 PARTITION OF emails
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE UNIQUE INDEX idx_emails_tracking_id ON emails (tracking_id, created_at);
CREATE INDEX idx_emails_external_id ON emails (external_id, created_at) WHERE external_id IS NOT NULL;
CREATE INDEX idx_emails_workspace_created ON emails (workspace_id, created_at DESC);
CREATE INDEX idx_emails_recipient ON emails (recipient_email, created_at DESC);
CREATE INDEX idx_emails_from ON emails (from_email, created_at DESC);
CREATE INDEX idx_emails_tenant_created ON emails (tenant_id, created_at DESC);
CREATE INDEX idx_emails_retry ON emails (next_retry_at, created_at) WHERE status = 'queued';
CREATE INDEX idx_emails_workspace_cursor ON emails (workspace_id, id, created_at);

-- Email lifecycle events (append-only)
CREATE TABLE email_events (
    id          UUID NOT NULL DEFAULT gen_random_uuid(),
    email_id    UUID NOT NULL,
    event_type  email_status NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at),
    CONSTRAINT email_events_unique UNIQUE (email_id, event_type, occurred_at, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE email_events_2026_01 PARTITION OF email_events
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE email_events_2026_02 PARTITION OF email_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE email_events_2026_03 PARTITION OF email_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE INDEX idx_email_events_email ON email_events (email_id, occurred_at);
