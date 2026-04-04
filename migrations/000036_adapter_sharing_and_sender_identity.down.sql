DROP INDEX IF EXISTS idx_emails_sender_identity;

ALTER TABLE emails
    DROP COLUMN IF EXISTS sender_identity_id;

DROP INDEX IF EXISTS idx_aiwg_workspace;
DROP TABLE IF EXISTS adapter_identity_workspace_grants;

DROP INDEX IF EXISTS idx_awg_workspace;
DROP TABLE IF EXISTS adapter_workspace_grants;
