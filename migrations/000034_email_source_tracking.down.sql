DROP INDEX IF EXISTS idx_emails_source_actor_member_created;
DROP INDEX IF EXISTS idx_emails_source_type_created;

ALTER TABLE emails
    DROP COLUMN IF EXISTS source_actor_email,
    DROP COLUMN IF EXISTS source_actor_member_id,
    DROP COLUMN IF EXISTS source_type;
