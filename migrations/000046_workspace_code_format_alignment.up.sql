ALTER TABLE workspaces
    DROP CONSTRAINT IF EXISTS workspaces_code_format;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_code_format CHECK (
        code = '_system' OR (
            code ~ '^[a-z](?:[a-z0-9_-]*[a-z0-9])?$'
            AND length(code) BETWEEN 2 AND 50
            AND code NOT IN ('global', 'admin', 'api', 'system')
        )
    );
