CREATE TABLE IF NOT EXISTS sns_message_replays (
    topic_arn text NOT NULL,
    message_id text NOT NULL,
    message_timestamp timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (topic_arn, message_id)
);

CREATE INDEX IF NOT EXISTS idx_sns_message_replays_expires_at
    ON sns_message_replays (expires_at);
