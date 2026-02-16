CREATE TABLE global_config (
    key             VARCHAR(100) PRIMARY KEY,
    value           JSONB NOT NULL,
    description     TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      UUID
);

INSERT INTO global_config (key, value, description) VALUES
    ('oidc.discovery_url', '"https://accounts.google.com/.well-known/openid-configuration"', 'OIDC provider discovery URL'),
    ('oidc.client_id', '""', 'OIDC client ID'),
    ('oidc.client_secret_encrypted', '""', 'OIDC client secret (encrypted)'),
    ('email.default_retry_count', '3', 'Default max retries for failed sends'),
    ('email.retry_backoff_base_seconds', '60', 'Base seconds for exponential backoff'),
    ('email.log_retention_days', '365', 'Days to retain email logs'),
    ('bounce.alert_threshold_percent', '5', 'Bounce rate % threshold for alerts (24h window)'),
    ('complaint.alert_threshold_percent', '0.1', 'Complaint rate % threshold for alerts'),
    ('domain.recheck_interval_hours', '24', 'Hours between domain re-verification checks'),
    ('onboarding.completed', 'false', 'Whether initial onboarding has been completed');
