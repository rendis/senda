-- Sweep emails stuck in 'processing' for more than 30 minutes.
-- Runs every 5 minutes via pg_cron.
SELECT cron.schedule(
    'stuck-processing-sweep',
    '*/5 * * * *',
    $$
    WITH stuck AS (
        UPDATE emails
        SET status = 'failed', updated_at = now()
        WHERE status = 'processing'
          AND updated_at < now() - interval '30 minutes'
        RETURNING id, created_at
    )
    INSERT INTO email_events (id, email_id, event_type, occurred_at, metadata, created_at)
    SELECT
        gen_random_uuid(),
        s.id,
        'failed',
        now(),
        '{"reason": "stuck_processing_timeout"}'::jsonb,
        s.created_at
    FROM stuck s;
    $$
);
