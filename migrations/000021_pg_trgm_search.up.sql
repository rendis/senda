-- Enable trigram extension for fuzzy text search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- GIN indexes for fast ILIKE search on tenants
CREATE INDEX idx_tenants_name_trgm ON tenants USING GIN (name gin_trgm_ops);
CREATE INDEX idx_tenants_code_trgm ON tenants USING GIN (code gin_trgm_ops);

-- GIN indexes for fast ILIKE search on workspaces
CREATE INDEX idx_workspaces_name_trgm ON workspaces USING GIN (name gin_trgm_ops);
CREATE INDEX idx_workspaces_code_trgm ON workspaces USING GIN (code gin_trgm_ops);
