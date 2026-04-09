ALTER TABLE injector_fields
    DROP COLUMN IF EXISTS allow_overwrite,
    DROP COLUMN IF EXISTS default_value;
