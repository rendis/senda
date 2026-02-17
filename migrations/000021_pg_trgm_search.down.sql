DROP INDEX IF EXISTS idx_workspaces_code_trgm;
DROP INDEX IF EXISTS idx_workspaces_name_trgm;
DROP INDEX IF EXISTS idx_tenants_code_trgm;
DROP INDEX IF EXISTS idx_tenants_name_trgm;

DROP EXTENSION IF EXISTS pg_trgm;
