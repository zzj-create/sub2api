-- Persist Grok SSO risk-state checks independently from the real-model
-- throughput observation. Both views share one row per account so the admin
-- account list can show model quality and account degradation together.

ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_state VARCHAR(24) NOT NULL DEFAULT 'unknown';
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_reason TEXT;
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_bot_flag_source INT;
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_risk DOUBLE PRECISION;
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_policy VARCHAR(40);
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_event VARCHAR(100);
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_http_status INT;
ALTER TABLE proxy_pool_account_quality_snapshots
    ADD COLUMN IF NOT EXISTS sso_checked_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_proxy_pool_account_quality_sso_checked
    ON proxy_pool_account_quality_snapshots(sso_checked_at DESC);
