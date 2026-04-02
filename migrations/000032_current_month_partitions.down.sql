DO $body$
DECLARE
    v_suffix TEXT := to_char(date_trunc('month', now()), 'YYYY_MM');
BEGIN
    EXECUTE format('DROP TABLE IF EXISTS emails_%s', v_suffix);
    EXECUTE format('DROP TABLE IF EXISTS email_events_%s', v_suffix);
    EXECUTE format('DROP TABLE IF EXISTS audit_logs_%s', v_suffix);
END
$body$;
