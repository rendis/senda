-- Performance indices for hot query paths.
-- The emails table is partitioned by created_at; PG 16 propagates
-- indices to all partitions automatically.
-- Note: CONCURRENTLY removed — golang-migrate wraps migrations in
-- transactions and PG forbids CONCURRENTLY inside a transaction.

CREATE INDEX IF NOT EXISTS idx_emails_provider_msg_id
    ON emails (provider_message_id, created_at)
    WHERE provider_message_id IS NOT NULL;

-- idx_template_versions_published already exists (migration 000009),
-- so we skip it here.

CREATE INDEX IF NOT EXISTS idx_emails_dashboard
    ON emails (workspace_id, created_at DESC, status);

CREATE INDEX IF NOT EXISTS idx_injector_fields_def_pos
    ON injector_fields (injector_definition_id, position);
