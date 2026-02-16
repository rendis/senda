CREATE UNLOGGED TABLE cache (
    key         VARCHAR(512) PRIMARY KEY,
    value       JSONB NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cache_expires ON cache(expires_at);

CREATE UNLOGGED TABLE token_buckets (
    adapter_id  UUID PRIMARY KEY REFERENCES adapters(id),
    tokens      FLOAT NOT NULL,
    max_tokens  INT NOT NULL,
    refill_rate FLOAT NOT NULL,
    last_refill TIMESTAMPTZ NOT NULL DEFAULT now()
);
