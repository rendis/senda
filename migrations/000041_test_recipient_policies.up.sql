ALTER TABLE workspaces
    ADD COLUMN test_recipient_mode VARCHAR(16) NOT NULL DEFAULT 'replace',
    ADD COLUMN test_recipient_addresses TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    ADD CONSTRAINT workspaces_test_recipient_mode_check
        CHECK (test_recipient_mode IN ('replace', 'append'));

ALTER TABLE template_types
    ADD COLUMN test_recipient_mode VARCHAR(16),
    ADD COLUMN test_recipient_addresses TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    ADD CONSTRAINT template_types_test_recipient_mode_check
        CHECK (test_recipient_mode IS NULL OR test_recipient_mode IN ('replace', 'append'));
