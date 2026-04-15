ALTER TABLE emails
    ADD COLUMN body_mjml TEXT,
    ADD COLUMN variables_snapshot JSONB,
    ADD COLUMN injectors_snapshot JSONB;

UPDATE emails e
SET
    body_mjml = p.body_mjml,
    variables_snapshot = p.variables_snapshot,
    injectors_snapshot = p.injectors_snapshot
FROM email_payloads p
WHERE e.id = p.email_id
  AND e.created_at = p.email_created_at;

DROP TABLE IF EXISTS email_payloads;
