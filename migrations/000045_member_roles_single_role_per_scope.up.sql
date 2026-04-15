WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY member_id,
                            scope_type,
                            COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid),
                            COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'::uuid)
               ORDER BY CASE role
                            WHEN 'superadmin' THEN 100
                            WHEN 'tenant_admin' THEN 80
                            WHEN 'workspace_admin' THEN 60
                            WHEN 'workspace_editor' THEN 40
                            WHEN 'workspace_viewer' THEN 20
                            ELSE 0
                        END DESC,
                        created_at DESC,
                        id DESC
           ) AS rn
    FROM member_roles
)
DELETE FROM member_roles mr
USING ranked r
WHERE mr.id = r.id
  AND r.rn > 1;

ALTER TABLE member_roles
    DROP CONSTRAINT IF EXISTS mr_unique_role;

ALTER TABLE member_roles
    ADD CONSTRAINT mr_unique_scope_assignment
        UNIQUE NULLS NOT DISTINCT (member_id, scope_type, tenant_id, workspace_id);
