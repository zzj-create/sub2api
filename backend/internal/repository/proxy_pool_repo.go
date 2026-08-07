package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type proxyPoolRepository struct {
	db *sql.DB
}

func NewProxyPoolRepository(db *sql.DB) service.ProxyPoolRepository {
	return &proxyPoolRepository{db: db}
}

type proxyPoolRowScanner interface {
	Scan(dest ...any) error
}

func scanProxyPool(row proxyPoolRowScanner) (*service.ProxyPool, error) {
	var (
		pool        service.ProxyPool
		description sql.NullString
		deletedAt   sql.NullTime
	)
	if err := row.Scan(
		&pool.ID,
		&pool.Name,
		&description,
		&pool.Status,
		&pool.HealthIntervalSeconds,
		&pool.FailureThreshold,
		&pool.AutoRebind,
		&pool.Mode,
		&pool.ActiveIntervalSeconds,
		&pool.PassiveWindowSeconds,
		&pool.QuarantineSeconds,
		&pool.SoftTPS,
		&pool.HardTPS,
		&pool.ConsecutiveSoft,
		&pool.ConsecutiveErrors,
		&pool.MinHealthyProxies,
		&pool.MinGenerationMs,
		&pool.MinOutputTokens,
		&pool.Model,
		&pool.DisableAccountOnHard,
		&pool.ThinkingGuard,
		&pool.ConsecutiveMissingThinking,
		&pool.ThinkingCrossVerify,
		&pool.SoftCrossVerify,
		&pool.MaxOutputTokensProbe,
		&pool.CreatedAt,
		&pool.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if description.Valid {
		pool.Description = &description.String
	}
	if deletedAt.Valid {
		pool.DeletedAt = &deletedAt.Time
	}
	pool.ProxyPoolQualityPolicy.Normalize()
	return &pool, nil
}

const proxyPoolColumns = `
id, name, description, status, health_interval_seconds,
	failure_threshold, auto_rebind,
	quality_mode, active_interval_seconds, passive_window_seconds,
	quarantine_seconds, soft_tps, hard_tps, consecutive_soft,
	consecutive_errors, min_healthy_proxies, min_generation_ms,
	min_output_tokens, quality_model, disable_account_on_hard,
	thinking_guard, consecutive_missing_thinking, thinking_cross_verify,
	soft_cross_verify, max_output_tokens_probe,
	created_at, updated_at, deleted_at
`

func qualityPolicyArgs(policy service.ProxyPoolQualityPolicy) []any {
	policy.Normalize()
	return []any{
		policy.Mode,
		policy.ActiveIntervalSeconds,
		policy.PassiveWindowSeconds,
		policy.QuarantineSeconds,
		policy.SoftTPS,
		policy.HardTPS,
		policy.ConsecutiveSoft,
		policy.ConsecutiveErrors,
		policy.MinHealthyProxies,
		policy.MinGenerationMs,
		policy.MinOutputTokens,
		policy.Model,
		policy.DisableAccountOnHard,
		policy.ThinkingGuard,
		policy.ConsecutiveMissingThinking,
		policy.ThinkingCrossVerify,
		policy.SoftCrossVerify,
		policy.MaxOutputTokensProbe,
	}
}

func (r *proxyPoolRepository) CreatePool(ctx context.Context, pool *service.ProxyPool) (*service.ProxyPool, error) {
	if r == nil || r.db == nil || pool == nil {
		return nil, errors.New("proxy pool repository unavailable")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO proxy_pools (
			name, description, status, health_interval_seconds, failure_threshold, auto_rebind,
			quality_mode, active_interval_seconds, passive_window_seconds,
			quarantine_seconds, soft_tps, hard_tps, consecutive_soft,
			consecutive_errors, min_healthy_proxies, min_generation_ms,
			min_output_tokens, quality_model, disable_account_on_hard,
			thinking_guard, consecutive_missing_thinking, thinking_cross_verify,
			soft_cross_verify, max_output_tokens_probe
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		RETURNING `+proxyPoolColumns,
		append([]any{
			pool.Name, pool.Description, pool.Status, pool.HealthIntervalSeconds, pool.FailureThreshold, pool.AutoRebind,
		}, qualityPolicyArgs(pool.ProxyPoolQualityPolicy)...)...,
	)
	created, err := scanProxyPool(row)
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrProxyPoolNameExists)
	}
	return created, nil
}

func (r *proxyPoolRepository) UpdatePool(ctx context.Context, pool *service.ProxyPool) error {
	if r == nil || r.db == nil || pool == nil {
		return errors.New("proxy pool repository unavailable")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE proxy_pools
		SET name = $2, description = $3, status = $4,
			health_interval_seconds = $5, failure_threshold = $6,
			auto_rebind = $7, quality_mode = $8,
			active_interval_seconds = $9, passive_window_seconds = $10,
			quarantine_seconds = $11, soft_tps = $12, hard_tps = $13,
			consecutive_soft = $14, consecutive_errors = $15,
			min_healthy_proxies = $16, min_generation_ms = $17,
			min_output_tokens = $18, quality_model = $19,
			disable_account_on_hard = $20, thinking_guard = $21,
			consecutive_missing_thinking = $22, thinking_cross_verify = $23,
			soft_cross_verify = $24, max_output_tokens_probe = $25,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, append([]any{pool.ID, pool.Name, pool.Description, pool.Status, pool.HealthIntervalSeconds, pool.FailureThreshold, pool.AutoRebind}, qualityPolicyArgs(pool.ProxyPoolQualityPolicy)...)...)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrProxyPoolNameExists)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrProxyPoolNotFound
	}
	return nil
}

func (r *proxyPoolRepository) DeletePool(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("proxy pool repository unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		UPDATE accounts
		SET pool_id = NULL, updated_at = NOW()
		WHERE pool_id = $1 AND deleted_at IS NULL
		RETURNING id
	`, id)
	if err != nil {
		return err
	}
	accountIDs, err := scanInt64Rows(rows)
	_ = rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE proxies
		SET pool_id = NULL, pool_health = 'unknown', pool_checked_at = NULL, pool_failures = 0,
			pool_grok_quality_status = 'unknown', pool_grok_quality_checked_at = NULL,
			pool_grok_quality_http_status = NULL, pool_grok_quality_message = NULL,
			pool_quality_class = 'unknown', pool_quality_strikes = 0,
			pool_quality_thinking_strikes = 0, pool_quality_error_strikes = 0,
			pool_quarantined_until = NULL,
			pool_quality_output_tps = 0, pool_quality_output_tokens = 0,
			pool_quality_duration_ms = 0, pool_quality_first_token_ms = 0,
			pool_quality_last_source = NULL, pool_quality_last_reason = NULL,
			pool_quality_observed_at = NULL, pool_quality_probed_at = NULL,
			pool_quality_account_id = NULL,
			updated_at = NOW()
		WHERE pool_id = $1
	`, id); err != nil {
		return err
	}
	// proxy_pool_groups references proxy pools with ON DELETE CASCADE, but pool
	// deletion is intentionally soft. Remove the bindings explicitly so groups
	// are not left owned by a pool that no longer exists in the admin UI.
	if _, err = tx.ExecContext(ctx, `DELETE FROM proxy_pool_groups WHERE pool_id = $1`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE proxy_pools SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrProxyPoolNotFound
	}
	if len(accountIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{"account_ids": accountIDs}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *proxyPoolRepository) GetPoolByID(ctx context.Context, id int64) (*service.ProxyPool, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("proxy pool repository unavailable")
	}
	pool, err := scanProxyPool(r.db.QueryRowContext(ctx, `SELECT `+proxyPoolColumns+` FROM proxy_pools WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrProxyPoolNotFound
	}
	return pool, err
}

func (r *proxyPoolRepository) ListPools(ctx context.Context) ([]service.ProxyPool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+proxyPoolColumns+` FROM proxy_pools WHERE deleted_at IS NULL ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ProxyPool, 0)
	for rows.Next() {
		pool, err := scanProxyPool(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *pool)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) ListPoolsWithStats(ctx context.Context) ([]service.ProxyPoolWithStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pp.id, pp.name, pp.description, pp.status, pp.health_interval_seconds,
			pp.failure_threshold, pp.auto_rebind,
			pp.quality_mode, pp.active_interval_seconds, pp.passive_window_seconds,
			pp.quarantine_seconds, pp.soft_tps, pp.hard_tps, pp.consecutive_soft,
			pp.consecutive_errors, pp.min_healthy_proxies, pp.min_generation_ms,
			pp.min_output_tokens, pp.quality_model, pp.disable_account_on_hard,
			pp.thinking_guard, pp.consecutive_missing_thinking, pp.thinking_cross_verify,
			pp.soft_cross_verify, pp.max_output_tokens_probe,
			pp.created_at, pp.updated_at, pp.deleted_at,
			COALESCE(ps.proxy_count, 0), COALESCE(ps.healthy_count, 0),
			COALESCE(ps.unhealthy_count, 0), COALESCE(ac.bound_account_count, 0),
			COALESCE(gc.bound_group_count, 0)
		FROM proxy_pools pp
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS proxy_count,
				COUNT(*) FILTER (
					WHERE p.pool_health = 'healthy' AND p.status = 'active'
						AND p.pool_grok_quality_status = 'pass'
						AND (p.pool_quarantined_until IS NULL OR p.pool_quarantined_until <= NOW())
				) AS healthy_count,
				COUNT(*) FILTER (
					WHERE p.pool_health = 'unhealthy' OR p.status <> 'active'
						OR p.pool_grok_quality_status NOT IN ('pass', 'unknown')
						OR p.pool_quarantined_until > NOW()
				) AS unhealthy_count
			FROM proxies p
			WHERE p.pool_id = pp.id AND p.deleted_at IS NULL
		) ps ON TRUE
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS bound_account_count
			FROM accounts a
			WHERE a.pool_id = pp.id AND a.deleted_at IS NULL
		) ac ON TRUE
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS bound_group_count
			FROM proxy_pool_groups ppg
			JOIN groups g ON g.id = ppg.group_id AND g.deleted_at IS NULL
			WHERE ppg.pool_id = pp.id
		) gc ON TRUE
		WHERE pp.deleted_at IS NULL
		ORDER BY pp.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ProxyPoolWithStats, 0)
	for rows.Next() {
		var (
			item        service.ProxyPoolWithStats
			description sql.NullString
			deletedAt   sql.NullTime
		)
		if err := rows.Scan(
			&item.ID, &item.Name, &description, &item.Status,
			&item.HealthIntervalSeconds, &item.FailureThreshold, &item.AutoRebind,
			&item.Mode, &item.ActiveIntervalSeconds, &item.PassiveWindowSeconds,
			&item.QuarantineSeconds, &item.SoftTPS, &item.HardTPS, &item.ConsecutiveSoft,
			&item.ConsecutiveErrors, &item.MinHealthyProxies, &item.MinGenerationMs,
			&item.MinOutputTokens, &item.Model, &item.DisableAccountOnHard,
			&item.ThinkingGuard, &item.ConsecutiveMissingThinking, &item.ThinkingCrossVerify,
			&item.SoftCrossVerify, &item.MaxOutputTokensProbe,
			&item.CreatedAt, &item.UpdatedAt, &deletedAt,
			&item.ProxyCount, &item.HealthyProxyCount, &item.UnhealthyProxyCount, &item.BoundAccountCount,
			&item.BoundGroupCount,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			item.Description = &description.String
		}
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		item.ProxyPoolQualityPolicy.Normalize()
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) ListPoolGroups(ctx context.Context, poolID int64) ([]service.ProxyPoolGroup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.platform, g.status, COUNT(DISTINCT a.id)
		FROM proxy_pool_groups ppg
		JOIN groups g ON g.id = ppg.group_id AND g.deleted_at IS NULL
		LEFT JOIN account_groups ag ON ag.group_id = g.id
		LEFT JOIN accounts a ON a.id = ag.account_id AND a.deleted_at IS NULL
		WHERE ppg.pool_id = $1
		GROUP BY g.id, g.name, g.platform, g.status
		ORDER BY g.id ASC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ProxyPoolGroup, 0)
	for rows.Next() {
		var group service.ProxyPoolGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.Platform, &group.Status, &group.AccountCount); err != nil {
			return nil, err
		}
		pool := poolID
		group.BoundPoolID = &pool
		result = append(result, group)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) ListPoolGroupOptions(ctx context.Context, poolID int64) ([]service.ProxyPoolGroup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.platform, g.status, COUNT(DISTINCT a.id),
			ppg.pool_id, COALESCE(pp.name, '')
		FROM groups g
		LEFT JOIN proxy_pool_groups ppg ON ppg.group_id = g.id
		LEFT JOIN proxy_pools pp ON pp.id = ppg.pool_id AND pp.deleted_at IS NULL
		LEFT JOIN account_groups ag ON ag.group_id = g.id
		LEFT JOIN accounts a ON a.id = ag.account_id AND a.deleted_at IS NULL
		WHERE g.deleted_at IS NULL
		GROUP BY g.id, g.name, g.platform, g.status, ppg.pool_id, pp.name
		ORDER BY CASE WHEN ppg.pool_id = $1 THEN 0 ELSE 1 END, g.id ASC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ProxyPoolGroup, 0)
	for rows.Next() {
		var (
			group       service.ProxyPoolGroup
			boundPoolID sql.NullInt64
			boundName   sql.NullString
		)
		if err := rows.Scan(
			&group.ID, &group.Name, &group.Platform, &group.Status, &group.AccountCount,
			&boundPoolID, &boundName,
		); err != nil {
			return nil, err
		}
		if boundPoolID.Valid && boundName.Valid {
			group.BoundPoolID = &boundPoolID.Int64
			group.BoundPoolName = boundName.String
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) BindGroupsToPool(ctx context.Context, poolID int64, groupIDs []int64) (*service.ProxyPoolGroupBindResult, error) {
	if len(groupIDs) == 0 {
		return &service.ProxyPoolGroupBindResult{}, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	validRows, err := tx.QueryContext(ctx, `
		SELECT id FROM groups
		WHERE id = ANY($1) AND deleted_at IS NULL AND status = 'active'
	`, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	validIDs, err := scanInt64Rows(validRows)
	_ = validRows.Close()
	if err != nil {
		return nil, err
	}
	if len(validIDs) != len(groupIDs) {
		return nil, service.ErrProxyPoolGroupInvalid
	}

	var conflictingGroupID int64
	err = tx.QueryRowContext(ctx, `
		SELECT group_id FROM proxy_pool_groups
		WHERE group_id = ANY($1) AND pool_id <> $2
		LIMIT 1
	`, pq.Array(groupIDs), poolID).Scan(&conflictingGroupID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil && conflictingGroupID > 0 {
		return nil, service.ErrProxyPoolGroupBound
	}

	inserted, err := tx.ExecContext(ctx, `
		INSERT INTO proxy_pool_groups (pool_id, group_id)
		SELECT $1, unnest($2::bigint[])
		ON CONFLICT DO NOTHING
	`, poolID, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	insertedCount, err := inserted.RowsAffected()
	if err != nil {
		return nil, err
	}

	accountIDs, err := syncPoolGroupAccountsTx(ctx, tx, poolID)
	if err != nil {
		return nil, err
	}
	if len(accountIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
			"account_ids": accountIDs,
			"pool_id":     poolID,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ProxyPoolGroupBindResult{BoundGroups: int(insertedCount), SyncedAccounts: len(accountIDs)}, nil
}

func (r *proxyPoolRepository) UnbindGroupsFromPool(ctx context.Context, poolID int64, groupIDs []int64) (*service.ProxyPoolGroupUnbindResult, error) {
	if len(groupIDs) == 0 {
		return &service.ProxyPoolGroupUnbindResult{}, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	accountRows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT ag.account_id
		FROM proxy_pool_groups ppg
		JOIN account_groups ag ON ag.group_id = ppg.group_id
		WHERE ppg.pool_id = $1 AND ppg.group_id = ANY($2)
	`, poolID, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	affectedAccountIDs, err := scanInt64Rows(accountRows)
	_ = accountRows.Close()
	if err != nil {
		return nil, err
	}

	deleted, err := tx.ExecContext(ctx, `
		DELETE FROM proxy_pool_groups
		WHERE pool_id = $1 AND group_id = ANY($2)
	`, poolID, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	deletedCount, err := deleted.RowsAffected()
	if err != nil {
		return nil, err
	}

	detachedIDs := make([]int64, 0)
	if len(affectedAccountIDs) > 0 {
		rows, updateErr := tx.QueryContext(ctx, `
			UPDATE accounts a
			SET pool_id = NULL, updated_at = NOW()
			WHERE a.id = ANY($1) AND a.pool_id = $2 AND a.deleted_at IS NULL
				AND NOT EXISTS (
					SELECT 1
					FROM account_groups ag
					JOIN proxy_pool_groups ppg ON ppg.group_id = ag.group_id AND ppg.pool_id = $2
					WHERE ag.account_id = a.id
				)
			RETURNING a.id
		`, pq.Array(affectedAccountIDs), poolID)
		if updateErr != nil {
			return nil, updateErr
		}
		detachedIDs, err = scanInt64Rows(rows)
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if len(detachedIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
			"account_ids": detachedIDs,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ProxyPoolGroupUnbindResult{UnboundGroups: int(deletedCount), DetachedAccounts: len(detachedIDs)}, nil
}

func (r *proxyPoolRepository) SyncPoolGroupAccounts(ctx context.Context, poolID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	accountIDs, err := syncPoolGroupAccountsTx(ctx, tx, poolID)
	if err != nil {
		return 0, err
	}
	if len(accountIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
			"account_ids": accountIDs,
			"pool_id":     poolID,
		}); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(accountIDs)), nil
}

func syncPoolGroupAccountsTx(ctx context.Context, tx *sql.Tx, poolID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		WITH desired AS (
			SELECT DISTINCT ON (ag.account_id)
				ag.account_id, ppg.pool_id
			FROM account_groups ag
			JOIN proxy_pool_groups ppg ON ppg.group_id = ag.group_id
			JOIN groups g ON g.id = ag.group_id AND g.deleted_at IS NULL
			JOIN proxy_pools pp ON pp.id = ppg.pool_id
			JOIN accounts candidate ON candidate.id = ag.account_id AND candidate.deleted_at IS NULL
			WHERE pp.deleted_at IS NULL AND pp.status = 'active'
			ORDER BY ag.account_id, ag.priority ASC, ppg.pool_id ASC, ag.group_id ASC
		), target AS (
			SELECT account_id FROM desired WHERE pool_id = $1
		)
		UPDATE accounts a
		SET pool_id = $1,
			proxy_id = CASE
				WHEN EXISTS (
					SELECT 1 FROM proxies p
					WHERE p.id = a.proxy_id AND p.pool_id = $1
						AND p.deleted_at IS NULL AND p.status = 'active'
				) THEN a.proxy_id
				ELSE NULL
			END,
			updated_at = NOW()
		FROM target
		WHERE a.id = target.account_id AND a.deleted_at IS NULL
			AND (
				a.pool_id IS DISTINCT FROM $1 OR
				(a.proxy_id IS NOT NULL AND NOT EXISTS (
					SELECT 1 FROM proxies p
					WHERE p.id = a.proxy_id AND p.pool_id = $1
						AND p.deleted_at IS NULL AND p.status = 'active'
				))
			)
		RETURNING a.id
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func (r *proxyPoolRepository) ListPoolProxies(ctx context.Context, poolID int64) ([]service.ProxyPoolProxy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.protocol, p.host, p.port, p.username, p.password, p.status,
			p.created_at, p.updated_at, p.pool_id, p.pool_health, p.pool_checked_at,
			p.pool_failures, p.pool_quality_class, p.pool_quality_strikes,
			p.pool_quality_thinking_strikes, p.pool_quality_error_strikes, p.pool_quarantined_until,
			p.pool_quality_output_tps, p.pool_quality_output_tokens,
			p.pool_quality_duration_ms, p.pool_quality_first_token_ms,
			p.pool_quality_last_source, p.pool_quality_last_reason,
			p.pool_quality_observed_at, p.pool_quality_probed_at,
			p.pool_quality_account_id, COALESCE(quality_account.name, ''),
			p.pool_grok_quality_status, p.pool_grok_quality_checked_at,
			p.pool_grok_quality_http_status, p.pool_grok_quality_message, COUNT(a.id)
		FROM proxies p
		LEFT JOIN accounts a ON a.proxy_id = p.id AND a.pool_id = $1 AND a.deleted_at IS NULL
		LEFT JOIN accounts quality_account ON quality_account.id = p.pool_quality_account_id
		WHERE p.pool_id = $1 AND p.deleted_at IS NULL
		GROUP BY p.id, quality_account.name
		ORDER BY p.id ASC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.ProxyPoolProxy, 0)
	for rows.Next() {
		var (
			item             service.ProxyPoolProxy
			username         sql.NullString
			password         sql.NullString
			checkedAt        sql.NullTime
			quarantinedAt    sql.NullTime
			qualityTPS       sql.NullFloat64
			qualityClass     sql.NullString
			qualitySource    sql.NullString
			qualityReason    sql.NullString
			qualityAt        sql.NullTime
			qualityProbed    sql.NullTime
			qualityAccountID sql.NullInt64
			grokCheckedAt    sql.NullTime
			grokHTTP         sql.NullInt64
			grokMessage      sql.NullString
		)
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Protocol, &item.Host, &item.Port,
			&username, &password, &item.Status, &item.CreatedAt, &item.UpdatedAt,
			&item.PoolID, &item.PoolHealth, &checkedAt, &item.PoolFailures,
			&qualityClass, &item.QualityStrikes, &item.QualityThinkingStrikes, &item.QualityErrorStrikes,
			&quarantinedAt, &qualityTPS, &item.QualityOutputTokens,
			&item.QualityDurationMs, &item.QualityFirstTokenMs,
			&qualitySource, &qualityReason, &qualityAt, &qualityProbed,
			&qualityAccountID, &item.QualityAccountName,
			&item.GrokQualityStatus, &grokCheckedAt, &grokHTTP, &grokMessage, &item.AccountCount,
		); err != nil {
			return nil, err
		}
		if username.Valid {
			item.Username = username.String
		}
		if password.Valid {
			item.Password = password.String
		}
		if checkedAt.Valid {
			item.PoolCheckedAt = &checkedAt.Time
		}
		if qualityClass.Valid {
			item.QualityClass = qualityClass.String
		}
		if quarantinedAt.Valid {
			item.QuarantinedUntil = &quarantinedAt.Time
		}
		if qualityTPS.Valid {
			item.QualityOutputTPS = qualityTPS.Float64
		}
		if qualitySource.Valid {
			item.QualityLastSource = qualitySource.String
		}
		if qualityReason.Valid {
			item.QualityLastReason = qualityReason.String
		}
		if qualityAt.Valid {
			item.QualityObservedAt = &qualityAt.Time
		}
		if qualityProbed.Valid {
			item.QualityProbedAt = &qualityProbed.Time
		}
		if qualityAccountID.Valid {
			item.QualityAccountID = &qualityAccountID.Int64
		}
		if grokCheckedAt.Valid {
			item.GrokQualityCheckedAt = &grokCheckedAt.Time
		}
		if grokHTTP.Valid {
			status := int(grokHTTP.Int64)
			item.GrokQualityHTTPStatus = &status
		}
		if grokMessage.Valid {
			item.GrokQualityMessage = grokMessage.String
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) AssignProxiesToPool(ctx context.Context, poolID int64, proxyIDs []int64) (int64, error) {
	if len(proxyIDs) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// If a proxy moves between pools, its old pool accounts must be reassigned.
	rows, err := tx.QueryContext(ctx, `
		UPDATE accounts a
		SET proxy_id = NULL, updated_at = NOW()
		WHERE a.proxy_id = ANY($2) AND a.deleted_at IS NULL
			AND a.pool_id IS NOT NULL AND a.pool_id IS DISTINCT FROM $1
		RETURNING a.id
	`, poolID, pq.Array(proxyIDs))
	if err != nil {
		return 0, err
	}
	accountIDs, err := scanInt64Rows(rows)
	_ = rows.Close()
	if err != nil {
		return 0, err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE proxies
		SET pool_id = $1, pool_health = 'unknown', pool_checked_at = NULL,
			pool_failures = 0, pool_grok_quality_status = 'unknown',
			pool_grok_quality_checked_at = NULL, pool_grok_quality_http_status = NULL,
			pool_grok_quality_message = NULL, pool_quality_class = 'unknown',
			pool_quality_strikes = 0, pool_quality_thinking_strikes = 0,
			pool_quality_error_strikes = 0,
			pool_quarantined_until = NULL, pool_quality_output_tps = 0,
			pool_quality_output_tokens = 0, pool_quality_duration_ms = 0,
			pool_quality_first_token_ms = 0, pool_quality_last_source = NULL,
			pool_quality_last_reason = NULL, pool_quality_observed_at = NULL,
			pool_quality_probed_at = NULL, pool_quality_account_id = NULL,
			updated_at = NOW()
		WHERE id = ANY($2) AND deleted_at IS NULL
	`, poolID, pq.Array(proxyIDs))
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if len(accountIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
			"account_ids": accountIDs,
		}); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *proxyPoolRepository) RemoveProxiesFromPool(ctx context.Context, poolID int64, proxyIDs []int64) (int64, error) {
	if len(proxyIDs) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Accounts stay bound to the pool and are picked up by the next assignment pass.
	rows, err := tx.QueryContext(ctx, `
		UPDATE accounts SET proxy_id = NULL, updated_at = NOW()
		WHERE pool_id = $1 AND proxy_id = ANY($2) AND deleted_at IS NULL
		RETURNING id
	`, poolID, pq.Array(proxyIDs))
	if err != nil {
		return 0, err
	}
	accountIDs, err := scanInt64Rows(rows)
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE proxies
		SET pool_id = NULL, pool_health = 'unknown', pool_checked_at = NULL,
			pool_failures = 0, pool_grok_quality_status = 'unknown',
			pool_grok_quality_checked_at = NULL, pool_grok_quality_http_status = NULL,
			pool_grok_quality_message = NULL, pool_quality_class = 'unknown',
			pool_quality_strikes = 0, pool_quality_thinking_strikes = 0,
			pool_quality_error_strikes = 0,
			pool_quarantined_until = NULL, pool_quality_output_tps = 0,
			pool_quality_output_tokens = 0, pool_quality_duration_ms = 0,
			pool_quality_first_token_ms = 0, pool_quality_last_source = NULL,
			pool_quality_last_reason = NULL, pool_quality_observed_at = NULL,
			pool_quality_probed_at = NULL, pool_quality_account_id = NULL,
			updated_at = NOW()
		WHERE pool_id = $1 AND id = ANY($2) AND deleted_at IS NULL
	`, poolID, pq.Array(proxyIDs))
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if len(accountIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
			"account_ids": accountIDs,
			"pool_id":     poolID,
		}); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *proxyPoolRepository) UpdateProxyPoolHealth(ctx context.Context, poolID, proxyID int64, snapshot service.ProxyPoolHealthSnapshot) error {
	if snapshot.QualityClass == "" && snapshot.QualityStrikes == 0 && snapshot.QualityThinkingStrikes == 0 && snapshot.QualityErrorStrikes == 0 &&
		snapshot.QuarantinedUntil == nil && snapshot.QualityOutputTPS == 0 && snapshot.QualityOutputTokens == 0 &&
		snapshot.QualityDurationMs == 0 && snapshot.QualityFirstTokenMs == 0 && snapshot.QualityLastSource == "" &&
		snapshot.QualityLastReason == "" && snapshot.QualityObservedAt == nil && snapshot.QualityProbedAt == nil &&
		snapshot.QualityAccountID == nil {
		_, err := r.db.ExecContext(ctx, `
			UPDATE proxies
			SET pool_health = $3, pool_failures = $4, pool_checked_at = $5,
				pool_grok_quality_status = $6, pool_grok_quality_checked_at = $7,
				pool_grok_quality_http_status = $8, pool_grok_quality_message = $9,
				updated_at = NOW()
			WHERE id = $1 AND pool_id = $2 AND deleted_at IS NULL
		`, proxyID, poolID, snapshot.Health, snapshot.Failures, snapshot.CheckedAt,
			snapshot.GrokQualityStatus, snapshot.GrokQualityCheckedAt,
			snapshot.GrokQualityHTTPStatus, snapshot.GrokQualityMessage)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE proxies
		SET pool_health = $3, pool_failures = $4, pool_checked_at = $5,
			pool_grok_quality_status = $6, pool_grok_quality_checked_at = $7,
			pool_grok_quality_http_status = $8, pool_grok_quality_message = $9,
			pool_quality_class = $10, pool_quality_strikes = $11,
			pool_quality_thinking_strikes = $12, pool_quality_error_strikes = $13,
			pool_quarantined_until = $14, pool_quality_output_tps = $15,
			pool_quality_output_tokens = $16, pool_quality_duration_ms = $17,
			pool_quality_first_token_ms = $18, pool_quality_last_source = $19,
			pool_quality_last_reason = $20, pool_quality_observed_at = $21,
			pool_quality_probed_at = $22, pool_quality_account_id = $23,
			updated_at = NOW()
		WHERE id = $1 AND pool_id = $2 AND deleted_at IS NULL
	`, proxyID, poolID, snapshot.Health, snapshot.Failures, snapshot.CheckedAt,
		snapshot.GrokQualityStatus, snapshot.GrokQualityCheckedAt,
		snapshot.GrokQualityHTTPStatus, snapshot.GrokQualityMessage,
		snapshot.QualityClass, snapshot.QualityStrikes, snapshot.QualityThinkingStrikes,
		snapshot.QualityErrorStrikes, snapshot.QuarantinedUntil, snapshot.QualityOutputTPS,
		snapshot.QualityOutputTokens, snapshot.QualityDurationMs, snapshot.QualityFirstTokenMs,
		snapshot.QualityLastSource, snapshot.QualityLastReason, snapshot.QualityObservedAt,
		snapshot.QualityProbedAt, snapshot.QualityAccountID)
	return err
}

// GetPoolIDByProxyID resolves the pool that currently owns an exit. It is kept
// as a narrow query so passive observations do not need to load proxy secrets.
func (r *proxyPoolRepository) GetPoolIDByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	if r == nil || r.db == nil || proxyID <= 0 {
		return 0, errors.New("proxy pool repository unavailable")
	}
	var poolID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT pool_id FROM proxies
		WHERE id = $1 AND deleted_at IS NULL AND pool_id IS NOT NULL
	`, proxyID).Scan(&poolID)
	if errors.Is(err, sql.ErrNoRows) || !poolID.Valid {
		return 0, service.ErrProxyPoolNotFound
	}
	return poolID.Int64, err
}

// ListGrokProbeAccountIDs returns a bounded set of credentials that can be
// used to test one pool exit. Prefer accounts already assigned to that exit,
// then fall back to other active accounts in the same pool. Credential errors
// are handled by the probe and must not affect the proxy state.
func (r *proxyPoolRepository) ListGrokProbeAccountIDs(ctx context.Context, poolID, preferredProxyID int64, limit int) ([]int64, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("proxy pool repository unavailable")
	}
	if limit <= 0 || limit > 32 {
		limit = 8
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id
		FROM accounts a
		LEFT JOIN proxy_pool_account_quality_snapshots aq ON aq.account_id = a.id
		WHERE a.pool_id = $1
			AND a.deleted_at IS NULL
			AND a.status = 'active'
			AND a.platform = $2
			AND a.type IN ('oauth', 'apikey')
			AND a.schedulable = TRUE
			AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())
			AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
		ORDER BY CASE WHEN a.proxy_id = $3 THEN 0 ELSE 1 END,
		         aq.observed_at NULLS FIRST,
		         a.last_used_at NULLS FIRST, a.id ASC
		LIMIT $4
	`, poolID, service.PlatformGrok, preferredProxyID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

// UpsertAccountQualitySnapshot stores the latest observation for one account.
// The timestamp guard prevents a slower concurrent probe from replacing a
// newer result that completed first.
func (r *proxyPoolRepository) UpsertAccountQualitySnapshot(ctx context.Context, snapshot service.ProxyPoolAccountQualitySnapshot) error {
	if r == nil || r.db == nil || snapshot.AccountID <= 0 || snapshot.PoolID <= 0 || snapshot.ProxyID <= 0 {
		return errors.New("invalid account quality snapshot")
	}
	qualityClass := strings.TrimSpace(snapshot.QualityClass)
	if qualityClass == "" {
		qualityClass = service.ProxyPoolQualityUnknown
	}
	source := strings.TrimSpace(snapshot.Source)
	if source == "" {
		source = "passive"
	}
	observedAt := snapshot.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	var hasThinking any
	if snapshot.HasThinking != nil {
		hasThinking = *snapshot.HasThinking
	}
	var httpStatus any
	if snapshot.HTTPStatus != nil {
		httpStatus = *snapshot.HTTPStatus
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO proxy_pool_account_quality_snapshots (
			account_id, pool_id, proxy_id, quality_class, output_tps,
			output_tokens, duration_ms, first_token_ms, has_thinking,
			source, reason, error_kind, http_status, observed_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		ON CONFLICT (account_id) DO UPDATE SET
			pool_id = EXCLUDED.pool_id,
			proxy_id = EXCLUDED.proxy_id,
			quality_class = EXCLUDED.quality_class,
			output_tps = EXCLUDED.output_tps,
			output_tokens = EXCLUDED.output_tokens,
			duration_ms = EXCLUDED.duration_ms,
			first_token_ms = EXCLUDED.first_token_ms,
			has_thinking = EXCLUDED.has_thinking,
			source = EXCLUDED.source,
			reason = EXCLUDED.reason,
			error_kind = EXCLUDED.error_kind,
			http_status = EXCLUDED.http_status,
			observed_at = EXCLUDED.observed_at,
			updated_at = NOW()
		WHERE proxy_pool_account_quality_snapshots.observed_at <= EXCLUDED.observed_at
	`, snapshot.AccountID, snapshot.PoolID, snapshot.ProxyID, qualityClass,
		snapshot.OutputTPS, snapshot.OutputTokens, snapshot.DurationMs,
		snapshot.FirstTokenMs, hasThinking, source, snapshot.Reason,
		snapshot.ErrorKind, httpStatus, observedAt)
	return err
}

// ListAccountQualitySnapshots returns one latest snapshot per requested
// account. Pool and proxy names are joined at read time so renames are
// immediately reflected in the admin table.
func (r *proxyPoolRepository) ListAccountQualitySnapshots(ctx context.Context, accountIDs []int64) (map[int64]*service.ProxyPoolAccountQualitySnapshot, error) {
	result := make(map[int64]*service.ProxyPoolAccountQualitySnapshot, len(accountIDs))
	if r == nil || r.db == nil || len(accountIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT q.account_id, q.pool_id, COALESCE(pp.name, ''),
		       q.proxy_id, COALESCE(p.name, ''), q.quality_class,
		       q.output_tps, q.output_tokens, q.duration_ms, q.first_token_ms,
		       q.has_thinking, q.source, COALESCE(q.reason, ''),
		       COALESCE(q.error_kind, ''), q.http_status, q.observed_at
		FROM proxy_pool_account_quality_snapshots q
		LEFT JOIN proxy_pools pp ON pp.id = q.pool_id
		LEFT JOIN proxies p ON p.id = q.proxy_id
		WHERE q.account_id = ANY($1)
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			accountID, poolID, proxyID             int64
			poolName, proxyName, qualityClass      string
			outputTPS                              float64
			outputTokens, durationMs, firstTokenMs int64
			hasThinking                            sql.NullBool
			source, reason, errorKind              string
			httpStatus                             sql.NullInt64
			observedAt                             time.Time
		)
		if err := rows.Scan(
			&accountID, &poolID, &poolName, &proxyID, &proxyName, &qualityClass,
			&outputTPS, &outputTokens, &durationMs, &firstTokenMs, &hasThinking,
			&source, &reason, &errorKind, &httpStatus, &observedAt,
		); err != nil {
			return nil, err
		}
		snapshot := &service.ProxyPoolAccountQualitySnapshot{
			AccountID: accountID, PoolID: poolID, PoolName: poolName,
			ProxyID: proxyID, ProxyName: proxyName, QualityClass: qualityClass,
			OutputTPS: outputTPS, OutputTokens: outputTokens, DurationMs: durationMs,
			FirstTokenMs: firstTokenMs, Source: source, Reason: reason,
			ErrorKind: errorKind, ObservedAt: observedAt,
		}
		if hasThinking.Valid {
			value := hasThinking.Bool
			snapshot.HasThinking = &value
		}
		if httpStatus.Valid {
			value := int(httpStatus.Int64)
			snapshot.HTTPStatus = &value
		}
		result[accountID] = snapshot
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *proxyPoolRepository) ListPoolUnassignedAccountIDs(ctx context.Context, poolID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id
		FROM accounts a
		LEFT JOIN proxies p ON p.id = a.proxy_id AND p.deleted_at IS NULL
		WHERE a.pool_id = $1 AND a.deleted_at IS NULL
			AND (a.proxy_id IS NULL OR p.pool_id IS DISTINCT FROM $1)
		ORDER BY a.id ASC
	`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func (r *proxyPoolRepository) ListAccountIDsByProxy(ctx context.Context, poolID, proxyID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id FROM accounts
		WHERE pool_id = $1 AND proxy_id = $2 AND deleted_at IS NULL
		ORDER BY id ASC
	`, poolID, proxyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanInt64Rows(rows)
}

func scanInt64Rows(rows *sql.Rows) ([]int64, error) {
	result := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) CountAccountsByProxyIDs(ctx context.Context, proxyIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(proxyIDs))
	if len(proxyIDs) == 0 {
		return counts, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT proxy_id, COUNT(*)
		FROM accounts
		WHERE proxy_id = ANY($1) AND deleted_at IS NULL
		GROUP BY proxy_id
	`, pq.Array(proxyIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var proxyID, count int64
		if err := rows.Scan(&proxyID, &count); err != nil {
			return nil, err
		}
		counts[proxyID] = count
	}
	return counts, rows.Err()
}

func (r *proxyPoolRepository) BindAccountsToPool(ctx context.Context, poolID int64, assignments []service.ProxyPoolAccountAssignment) ([]service.ProxyPoolAccountAssignment, error) {
	if len(assignments) == 0 {
		return []service.ProxyPoolAccountAssignment{}, nil
	}
	grouped := make(map[int64][]int64)
	for _, assignment := range assignments {
		grouped[assignment.ProxyID] = append(grouped[assignment.ProxyID], assignment.AccountID)
	}
	proxyIDs := make([]int64, 0, len(grouped))
	for proxyID := range grouped {
		proxyIDs = append(proxyIDs, proxyID)
	}
	sort.Slice(proxyIDs, func(i, j int) bool { return proxyIDs[i] < proxyIDs[j] })

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	applied := make([]service.ProxyPoolAccountAssignment, 0, len(assignments))
	for _, proxyID := range proxyIDs {
		rows, err := tx.QueryContext(ctx, `
			UPDATE accounts a
			SET pool_id = $1, proxy_id = $2, updated_at = NOW()
			WHERE a.id = ANY($3) AND a.deleted_at IS NULL
				AND EXISTS (
					SELECT 1 FROM proxies p
					WHERE p.id = $2 AND p.pool_id = $1 AND p.deleted_at IS NULL
						AND p.status = 'active' AND p.pool_health = 'healthy'
						AND p.pool_grok_quality_status = 'pass'
						AND (p.pool_quarantined_until IS NULL OR p.pool_quarantined_until <= NOW())
				)
			RETURNING a.id
		`, poolID, proxyID, pq.Array(grouped[proxyID]))
		if err != nil {
			return nil, err
		}
		ids, scanErr := scanInt64Rows(rows)
		_ = rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		for _, id := range ids {
			applied = append(applied, service.ProxyPoolAccountAssignment{AccountID: id, ProxyID: proxyID})
		}
	}
	if len(applied) > 0 {
		accountIDs := make([]int64, 0, len(applied))
		for _, assignment := range applied {
			accountIDs = append(accountIDs, assignment.AccountID)
		}
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{"account_ids": accountIDs, "pool_id": poolID}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	sort.Slice(applied, func(i, j int) bool { return applied[i].AccountID < applied[j].AccountID })
	return applied, nil
}

func (r *proxyPoolRepository) MarkAccountsPendingInPool(ctx context.Context, poolID int64, accountIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return []int64{}, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Keep the current proxy until a healthy pool member is available. The
	// regular assignment pass recognizes NULL or out-of-pool proxy IDs as pending.
	rows, err := tx.QueryContext(ctx, `
		UPDATE accounts
		SET pool_id = $1, updated_at = NOW()
		WHERE id = ANY($2) AND deleted_at IS NULL
		RETURNING id
	`, poolID, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	appliedIDs, err := scanInt64Rows(rows)
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if len(appliedIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
			"account_ids": appliedIDs,
			"pool_id":     poolID,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	sort.Slice(appliedIDs, func(i, j int) bool { return appliedIDs[i] < appliedIDs[j] })
	return appliedIDs, nil
}

func (r *proxyPoolRepository) UnbindAccountsFromPool(ctx context.Context, poolID int64, accountIDs []int64) (int64, error) {
	if len(accountIDs) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
		UPDATE accounts
		SET pool_id = NULL, updated_at = NOW()
		WHERE pool_id = $1 AND id = ANY($2) AND deleted_at IS NULL
		RETURNING id
	`, poolID, pq.Array(accountIDs))
	if err != nil {
		return 0, err
	}
	appliedIDs, err := scanInt64Rows(rows)
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	if len(appliedIDs) > 0 {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{"account_ids": appliedIDs}); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(appliedIDs)), nil
}

func (r *proxyPoolRepository) RecordRebindLog(ctx context.Context, entry *service.ProxyPoolRebindLog) error {
	if entry == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO proxy_pool_rebind_logs (pool_id, from_proxy_id, to_proxy_id, account_count, reason)
		VALUES ($1, $2, $3, $4, $5)
	`, entry.PoolID, entry.FromProxyID, entry.ToProxyID, entry.AccountCount, entry.Reason)
	return err
}

func (r *proxyPoolRepository) ListRebindLogs(ctx context.Context, poolID int64, limit int) ([]service.ProxyPoolRebindLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.pool_id, l.from_proxy_id, l.to_proxy_id,
			COALESCE(fp.name, ''), COALESCE(tp.name, ''),
			l.account_count, l.reason, l.created_at
		FROM proxy_pool_rebind_logs l
		LEFT JOIN proxies fp ON fp.id = l.from_proxy_id
		LEFT JOIN proxies tp ON tp.id = l.to_proxy_id
		WHERE l.pool_id = $1
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $2
	`, poolID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]service.ProxyPoolRebindLog, 0)
	for rows.Next() {
		var (
			entry service.ProxyPoolRebindLog
			from  sql.NullInt64
			to    sql.NullInt64
		)
		if err := rows.Scan(
			&entry.ID, &entry.PoolID, &from, &to, &entry.FromProxyName, &entry.ToProxyName,
			&entry.AccountCount, &entry.Reason, &entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		if from.Valid {
			entry.FromProxyID = &from.Int64
		}
		if to.Valid {
			entry.ToProxyID = &to.Int64
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r *proxyPoolRepository) String() string {
	return fmt.Sprintf("proxyPoolRepository(%p)", r)
}
