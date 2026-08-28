-- Native Sub2API equivalent of the CPA Grok egress quality guard.
-- All columns are additive and retain the existing proxy-pool defaults.

ALTER TABLE proxy_pools
    ADD COLUMN IF NOT EXISTS quality_mode VARCHAR(20) NOT NULL DEFAULT 'hybrid',
    ADD COLUMN IF NOT EXISTS active_interval_seconds INT NOT NULL DEFAULT 1800,
    ADD COLUMN IF NOT EXISTS passive_window_seconds INT NOT NULL DEFAULT 300,
    ADD COLUMN IF NOT EXISTS quarantine_seconds INT NOT NULL DEFAULT 120,
    ADD COLUMN IF NOT EXISTS soft_tps DOUBLE PRECISION NOT NULL DEFAULT 500,
    ADD COLUMN IF NOT EXISTS hard_tps DOUBLE PRECISION NOT NULL DEFAULT 1000,
    ADD COLUMN IF NOT EXISTS consecutive_soft INT NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS consecutive_errors INT NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS min_healthy_proxies INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS min_generation_ms BIGINT NOT NULL DEFAULT 1000,
    ADD COLUMN IF NOT EXISTS min_output_tokens BIGINT NOT NULL DEFAULT 32,
    ADD COLUMN IF NOT EXISTS quality_model VARCHAR(100) NOT NULL DEFAULT 'grok-4.5',
    ADD COLUMN IF NOT EXISTS disable_account_on_hard BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS thinking_guard BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS consecutive_missing_thinking INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS thinking_cross_verify BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS soft_cross_verify BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS max_output_tokens_probe INT NOT NULL DEFAULT 384;

ALTER TABLE proxies
    ADD COLUMN IF NOT EXISTS pool_quality_class VARCHAR(20) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS pool_quality_strikes INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quality_thinking_strikes INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quality_error_strikes INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quarantined_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS pool_quality_output_tps DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quality_output_tokens BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quality_duration_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quality_first_token_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pool_quality_last_source VARCHAR(20),
    ADD COLUMN IF NOT EXISTS pool_quality_last_reason TEXT,
    ADD COLUMN IF NOT EXISTS pool_quality_observed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS pool_quality_probed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_proxies_pool_quarantined_until
    ON proxies(pool_id, pool_quarantined_until)
    WHERE pool_id IS NOT NULL AND deleted_at IS NULL;
