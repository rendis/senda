ALTER TABLE member_roles
    DROP CONSTRAINT IF EXISTS mr_unique_scope_assignment;

ALTER TABLE member_roles
    ADD CONSTRAINT mr_unique_role
        UNIQUE NULLS NOT DISTINCT (member_id, role, scope_type, tenant_id, workspace_id);
