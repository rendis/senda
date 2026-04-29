-- Drop subscription table and its indexes (cascade drops indexes).
DROP TABLE IF EXISTS template_type_subscription;

-- Drop is_bulk column.
ALTER TABLE template_types DROP COLUMN IF EXISTS is_bulk;

-- Drop signing key column and constraint.
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_unsubscribe_signing_key_len;
ALTER TABLE workspaces DROP COLUMN IF EXISTS unsubscribe_signing_key;

-- Note: enum values cannot be removed in Postgres without recreating the type.
-- The 'unsubscribe' value is left in place; it is harmless if unused.
