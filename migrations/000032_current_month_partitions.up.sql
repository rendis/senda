-- Ensure a fresh database always has partitions for the current month.
-- Earlier migrations seeded fixed 2026_Q1 partitions and later pre-created only future months,
-- which breaks inserts when bootstrapping a brand-new environment in a later month.

DO $body$
DECLARE
    v_start DATE := date_trunc('month', now());
    v_end DATE := v_start + interval '1 month';
    v_suffix TEXT := to_char(v_start, 'YYYY_MM');
BEGIN
    EXECUTE format('CREATE TABLE IF NOT EXISTS emails_%s PARTITION OF emails FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
    EXECUTE format('CREATE TABLE IF NOT EXISTS email_events_%s PARTITION OF email_events FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
    EXECUTE format('CREATE TABLE IF NOT EXISTS audit_logs_%s PARTITION OF audit_logs FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
END
$body$;
