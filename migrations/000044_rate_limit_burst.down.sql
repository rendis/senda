DROP FUNCTION IF EXISTS take_send_token_burst(UUID, INT);

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
