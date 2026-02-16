-- Cache cleanup: purge expired entries every 60 seconds
SELECT cron.schedule('cache-cleanup', '*/1 * * * *',
    $$DELETE FROM cache WHERE expires_at < now()$$
);

-- Partition maintenance: create next month's partitions (runs 1st of each month)
SELECT cron.schedule('create-partitions', '0 0 1 * *',
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
