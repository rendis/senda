CREATE TABLE email_payloads (
    email_id             UUID PRIMARY KEY,
    email_created_at     TIMESTAMPTZ NOT NULL,
    body_mjml            TEXT,
    variables_snapshot   JSONB,
    injectors_snapshot   JSONB,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_email_payloads_email
        FOREIGN KEY (email_id, email_created_at)
        REFERENCES emails (id, created_at)
        ON DELETE CASCADE
);

INSERT INTO email_payloads (
    email_id,
    email_created_at,
    body_mjml,
    variables_snapshot,
    injectors_snapshot
)
SELECT
    id,
    created_at,
    body_mjml,
    variables_snapshot,
    injectors_snapshot
FROM emails
WHERE body_mjml IS NOT NULL
   OR variables_snapshot IS NOT NULL
   OR injectors_snapshot IS NOT NULL;

ALTER TABLE emails
    DROP COLUMN IF EXISTS body_mjml,
    DROP COLUMN IF EXISTS variables_snapshot,
    DROP COLUMN IF EXISTS injectors_snapshot;
