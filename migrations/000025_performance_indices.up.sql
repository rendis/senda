-- Performance indices for hot query paths.
-- The emails table is partitioned by created_at; PG 16 supports
-- CREATE INDEX CONCURRENTLY on partitioned tables, and the index
-- propagates to all partitions automatically.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_emails_provider_msg_id
    ON emails (provider_message_id, created_at)
    WHERE provider_message_id IS NOT NULL;

-- idx_template_versions_published already exists (migration 000009),
-- so we skip it here.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_emails_dashboard
    ON emails (workspace_id, created_at DESC, status);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_injector_fields_def_pos
    ON injector_fields (injector_definition_id, position);
