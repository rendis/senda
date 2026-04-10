ALTER TABLE template_types
    DROP CONSTRAINT IF EXISTS template_types_test_recipient_mode_check,
    DROP COLUMN IF EXISTS test_recipient_addresses,
    DROP COLUMN IF EXISTS test_recipient_mode;

ALTER TABLE workspaces
    DROP CONSTRAINT IF EXISTS workspaces_test_recipient_mode_check,
    DROP COLUMN IF EXISTS test_recipient_addresses,
    DROP COLUMN IF EXISTS test_recipient_mode;
