-- A group belongs to at most one proxy pool. A pool may own many groups.
-- Accounts are synchronized from account_groups by the proxy-pool worker.

CREATE TABLE IF NOT EXISTS proxy_pool_groups (
    pool_id    BIGINT NOT NULL REFERENCES proxy_pools(id) ON DELETE CASCADE,
    group_id   BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pool_id, group_id),
    UNIQUE (group_id)
);

CREATE INDEX IF NOT EXISTS idx_proxy_pool_groups_pool_id
    ON proxy_pool_groups(pool_id);
