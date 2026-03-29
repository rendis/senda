-- Reschedule partition creation to the 25th of each month (gives 5-day buffer)
-- and pre-create partitions for the next 6 months.

-- 1. Unschedule old cron (runs on the 1st).
SELECT cron.unschedule('create-partitions');

-- 2. Reschedule to the 25th of each month at midnight.
SELECT cron.schedule('create-partitions', '0 0 25 * *',
    $$
    DO $body$
    DECLARE
        v_start DATE := date_trunc('month', now() + interval '1 month');
        v_end DATE := v_start + interval '1 month';
        v_suffix TEXT := to_char(v_start, 'YYYY_MM');
    BEGIN
        EXECUTE format('CREATE TABLE IF NOT EXISTS emails_%s PARTITION OF emails FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
        EXECUTE format('CREATE TABLE IF NOT EXISTS email_events_%s PARTITION OF email_events FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
        EXECUTE format('CREATE TABLE IF NOT EXISTS audit_logs_%s PARTITION OF audit_logs FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
    END
    $body$;
    $$
);

-- 3. Pre-create partitions for the next 6 months from now.
DO $body$
DECLARE
    i INT;
    v_start DATE;
    v_end DATE;
    v_suffix TEXT;
BEGIN
    FOR i IN 1..6 LOOP
        v_start := date_trunc('month', now() + (i || ' months')::interval);
        v_end := v_start + interval '1 month';
        v_suffix := to_char(v_start, 'YYYY_MM');
        EXECUTE format('CREATE TABLE IF NOT EXISTS emails_%s PARTITION OF emails FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
        EXECUTE format('CREATE TABLE IF NOT EXISTS email_events_%s PARTITION OF email_events FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
        EXECUTE format('CREATE TABLE IF NOT EXISTS audit_logs_%s PARTITION OF audit_logs FOR VALUES FROM (%L) TO (%L)', v_suffix, v_start, v_end);
    END LOOP;
END
$body$;
