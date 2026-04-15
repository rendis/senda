CREATE OR REPLACE FUNCTION take_send_token_burst(p_adapter_id UUID, p_requested INT)
RETURNS INT AS $$
DECLARE
    v_row token_buckets%ROWTYPE;
    v_now TIMESTAMPTZ := now();
    v_elapsed FLOAT;
    v_new_tokens FLOAT;
    v_requested INT := GREATEST(COALESCE(p_requested, 0), 0);
    v_reserved INT;
BEGIN
    IF v_requested = 0 THEN
        RETURN 0;
    END IF;

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
    v_reserved := LEAST(FLOOR(v_new_tokens)::INT, v_requested);

    UPDATE token_buckets
    SET tokens = v_new_tokens - v_reserved, last_refill = v_now
    WHERE adapter_id = p_adapter_id;

    RETURN v_reserved;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION take_send_token(p_adapter_id UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN take_send_token_burst(p_adapter_id, 1) > 0;
END;
$$ LANGUAGE plpgsql;
