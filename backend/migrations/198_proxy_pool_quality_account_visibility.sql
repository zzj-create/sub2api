-- Expose the account behind the latest active probe or passive observation to
-- administrators without coupling quality history to account lifecycle.

ALTER TABLE proxies
    ADD COLUMN IF NOT EXISTS pool_quality_account_id BIGINT;
