-- Keep the latest quality observation per Grok account as well as the
-- per-proxy state. A proxy can serve many accounts, so proxy columns alone
-- cannot answer which account produced a given t/s result.

CREATE TABLE IF NOT EXISTS proxy_pool_account_quality_snapshots (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    pool_id BIGINT NOT NULL REFERENCES proxy_pools(id) ON DELETE CASCADE,
    proxy_id BIGINT NOT NULL REFERENCES proxies(id) ON DELETE CASCADE,
    quality_class VARCHAR(20) NOT NULL DEFAULT 'unknown',
    output_tps DOUBLE PRECISION NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    first_token_ms BIGINT NOT NULL DEFAULT 0,
    has_thinking BOOLEAN,
    source VARCHAR(20) NOT NULL DEFAULT 'passive',
    reason TEXT,
    error_kind VARCHAR(40),
    http_status INT,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proxy_pool_account_quality_observed
    ON proxy_pool_account_quality_snapshots(observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_proxy_pool_account_quality_pool
    ON proxy_pool_account_quality_snapshots(pool_id, observed_at DESC);

-- Preserve the useful proxy-level result for installations upgrading from
-- the previous guard. These rows are marked with an unknown thinking signal
-- because that field was not persisted by the old schema.
INSERT INTO proxy_pool_account_quality_snapshots (
    account_id, pool_id, proxy_id, quality_class, output_tps, output_tokens,
    duration_ms, first_token_ms, source, reason, observed_at, updated_at
)
SELECT p.pool_quality_account_id, p.pool_id, p.id, p.pool_quality_class,
       p.pool_quality_output_tps, p.pool_quality_output_tokens,
       p.pool_quality_duration_ms, p.pool_quality_first_token_ms,
       COALESCE(p.pool_quality_last_source, 'active'),
       p.pool_quality_last_reason, p.pool_quality_observed_at, NOW()
FROM proxies p
JOIN accounts a ON a.id = p.pool_quality_account_id
WHERE p.pool_quality_account_id IS NOT NULL
  AND p.pool_id IS NOT NULL
  AND p.pool_quality_observed_at IS NOT NULL
ON CONFLICT (account_id) DO UPDATE
SET pool_id = EXCLUDED.pool_id,
    proxy_id = EXCLUDED.proxy_id,
    quality_class = EXCLUDED.quality_class,
    output_tps = EXCLUDED.output_tps,
    output_tokens = EXCLUDED.output_tokens,
    duration_ms = EXCLUDED.duration_ms,
    first_token_ms = EXCLUDED.first_token_ms,
    source = EXCLUDED.source,
    reason = EXCLUDED.reason,
    observed_at = EXCLUDED.observed_at,
    updated_at = NOW()
WHERE proxy_pool_account_quality_snapshots.observed_at <= EXCLUDED.observed_at;
