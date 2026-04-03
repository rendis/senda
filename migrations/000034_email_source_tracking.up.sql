ALTER TABLE emails
    ADD COLUMN source_type VARCHAR(64) NOT NULL DEFAULT 'data_plane_api_key',
    ADD COLUMN source_actor_member_id UUID,
    ADD COLUMN source_actor_email VARCHAR(255);

CREATE INDEX idx_emails_source_type_created ON emails (source_type, created_at DESC);
CREATE INDEX idx_emails_source_actor_member_created ON emails (source_actor_member_id, created_at DESC)
    WHERE source_actor_member_id IS NOT NULL;
