ALTER TABLE injector_fields
    ADD COLUMN default_value JSONB,
    ADD COLUMN allow_overwrite BOOLEAN NOT NULL DEFAULT true;

UPDATE injector_fields AS field
SET default_value = value.value
FROM injector_values AS value
JOIN injector_definitions AS definition
  ON definition.id = value.injector_definition_id
WHERE definition.id = field.injector_definition_id
  AND value.injector_definition_id = field.injector_definition_id
  AND value.field_name = field.field_name
  AND value.workspace_id IS NOT DISTINCT FROM definition.workspace_id;
