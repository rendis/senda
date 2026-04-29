-- 1. Ensure pgcrypto for gen_random_bytes (idempotent).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 2. Add 'unsubscribe' value to suppression_reason enum.
ALTER TYPE suppression_reason ADD VALUE IF NOT EXISTS 'unsubscribe';

-- 3. Per-workspace HMAC signing key for unsubscribe tokens.
ALTER TABLE workspaces
    ADD COLUMN unsubscribe_signing_key BYTEA;

UPDATE workspaces
    SET unsubscribe_signing_key = gen_random_bytes(32)
    WHERE unsubscribe_signing_key IS NULL;

ALTER TABLE workspaces
    ALTER COLUMN unsubscribe_signing_key SET NOT NULL;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_unsubscribe_signing_key_len
        CHECK (length(unsubscribe_signing_key) = 32);

-- 4. Mark template_types as bulk (subject to unsubscribe headers + system vars).
ALTER TABLE template_types
    ADD COLUMN is_bulk BOOLEAN NOT NULL DEFAULT false;

-- 5. Per-(workspace, template_type, email) subscription state.

-- Source of a subscription state change.
CREATE TYPE subscription_source AS ENUM ('recipient_optout', 'recipient_optin', 'admin');

CREATE TABLE template_type_subscriptions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspaces(id),
    template_type_id  UUID NOT NULL REFERENCES template_types(id),
    email             VARCHAR(255) NOT NULL,
    subscribed        BOOLEAN NOT NULL,
    source            subscription_source NOT NULL,
    source_email_id   UUID,
    actor_id          UUID,
    notes             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tts_unique UNIQUE (workspace_id, template_type_id, email)
);

-- Partial index for the hot path: send pipeline only cares about explicit opt-outs.
CREATE INDEX idx_tts_optout
    ON template_type_subscriptions (workspace_id, template_type_id, email)
    WHERE subscribed = false;

-- Index for preference center lookups (all rows for a recipient in a workspace).
CREATE INDEX idx_tts_recipient_lookup
    ON template_type_subscriptions (workspace_id, email, template_type_id);
