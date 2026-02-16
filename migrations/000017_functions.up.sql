-- Resolution chain (Section 3.16): returns the chain workspace -> _system -> global
CREATE OR REPLACE FUNCTION get_resolution_chain(p_workspace_id UUID)
RETURNS TABLE(workspace_id UUID, priority INT) AS $$
    SELECT w.id, 1 AS priority
    FROM workspaces w WHERE w.id = p_workspace_id AND w.deleted_at IS NULL
    UNION ALL
    SELECT sys.id, 2 AS priority
    FROM workspaces w
    JOIN workspaces sys ON sys.tenant_id = w.tenant_id AND sys.is_system = true AND sys.deleted_at IS NULL
    WHERE w.id = p_workspace_id AND w.deleted_at IS NULL AND w.is_system = false
    UNION ALL
    SELECT NULL::UUID, 3 AS priority
$$ LANGUAGE sql STABLE;

-- Token bucket: atomic take-one-token
CREATE OR REPLACE FUNCTION take_send_token(p_adapter_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    v_row token_buckets%ROWTYPE;
    v_now TIMESTAMPTZ := now();
    v_elapsed FLOAT;
    v_new_tokens FLOAT;
BEGIN
    SELECT * INTO v_row
    FROM token_buckets
    WHERE adapter_id = p_adapter_id
    FOR UPDATE;

    IF NOT FOUND THEN
        INSERT INTO token_buckets (adapter_id, tokens, max_tokens, refill_rate)
        SELECT id, rate_limit_per_second, rate_limit_per_second, rate_limit_per_second
        FROM adapters WHERE id = p_adapter_id
        ON CONFLICT (adapter_id) DO NOTHING;

        SELECT * INTO v_row
        FROM token_buckets
        WHERE adapter_id = p_adapter_id
        FOR UPDATE;
    END IF;

    v_elapsed := EXTRACT(EPOCH FROM (v_now - v_row.last_refill));
    v_new_tokens := LEAST(v_row.max_tokens, v_row.tokens + (v_elapsed * v_row.refill_rate));

    IF v_new_tokens < 1 THEN
        UPDATE token_buckets
        SET tokens = v_new_tokens, last_refill = v_now
        WHERE adapter_id = p_adapter_id;
        RETURN FALSE;
    END IF;

    UPDATE token_buckets
    SET tokens = v_new_tokens - 1, last_refill = v_now
    WHERE adapter_id = p_adapter_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
