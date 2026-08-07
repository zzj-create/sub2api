package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestProxyPoolRepositoryPersistsGrokQualityHealth(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Now().UTC()
	httpStatus := 401
	mock.ExpectExec("UPDATE proxies[\\s\\S]+pool_grok_quality_status").
		WithArgs(
			int64(9), int64(7), service.ProxyPoolHealthHealthy, 0, checkedAt,
			"pass", checkedAt, httpStatus, "HTTP 401 (target reachable)",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewProxyPoolRepository(db)
	err = repo.UpdateProxyPoolHealth(context.Background(), 7, 9, service.ProxyPoolHealthSnapshot{
		Health:                service.ProxyPoolHealthHealthy,
		CheckedAt:             checkedAt,
		GrokQualityStatus:     "pass",
		GrokQualityCheckedAt:  &checkedAt,
		GrokQualityHTTPStatus: &httpStatus,
		GrokQualityMessage:    "HTTP 401 (target reachable)",
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryPersistsQualityAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	checkedAt := time.Now().UTC()
	accountID := int64(42)
	mock.ExpectExec("UPDATE proxies[\\s\\S]+pool_quality_account_id = \\$23").
		WithArgs(
			int64(9), int64(7), service.ProxyPoolHealthHealthy, 0, checkedAt,
			"pass", nil, nil, "",
			service.ProxyPoolQualityHealthy, 0, 0, 0, nil, float64(120),
			int64(64), int64(2000), int64(250), "active", "quality observation recorded",
			checkedAt, checkedAt, &accountID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewProxyPoolRepository(db)
	err = repo.UpdateProxyPoolHealth(context.Background(), 7, 9, service.ProxyPoolHealthSnapshot{
		Health:              service.ProxyPoolHealthHealthy,
		CheckedAt:           checkedAt,
		GrokQualityStatus:   "pass",
		QualityClass:        service.ProxyPoolQualityHealthy,
		QualityOutputTPS:    120,
		QualityOutputTokens: 64,
		QualityDurationMs:   2000,
		QualityFirstTokenMs: 250,
		QualityLastSource:   "active",
		QualityLastReason:   "quality observation recorded",
		QualityObservedAt:   &checkedAt,
		QualityProbedAt:     &checkedAt,
		QualityAccountID:    &accountID,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryListsQualityAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT p.id[\\s\\S]+quality_account.name[\\s\\S]+LEFT JOIN accounts quality_account").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "protocol", "host", "port", "username", "password", "status",
			"created_at", "updated_at", "pool_id", "pool_health", "pool_checked_at", "pool_failures",
			"quality_class", "quality_strikes", "quality_thinking_strikes", "quality_error_strikes", "quarantined_until",
			"quality_output_tps", "quality_output_tokens", "quality_duration_ms", "quality_first_token_ms",
			"quality_last_source", "quality_last_reason", "quality_observed_at", "quality_probed_at",
			"quality_account_id", "quality_account_name", "grok_quality_status", "grok_quality_checked_at",
			"grok_quality_http_status", "grok_quality_message", "account_count",
		}).AddRow(
			int64(9), "exit", "http", "proxy.example", 8080, nil, nil, service.StatusActive,
			now, now, int64(7), service.ProxyPoolHealthHealthy, now, 0,
			service.ProxyPoolQualityHealthy, 0, 0, 0, nil,
			float64(120), int64(64), int64(2000), int64(250),
			"active", "quality observation recorded", now, now,
			int64(42), "probe-account@example.com", "pass", now, 200, "", int64(3),
		))

	repo := NewProxyPoolRepository(db)
	proxies, err := repo.ListPoolProxies(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, proxies, 1)
	require.NotNil(t, proxies[0].QualityAccountID)
	require.Equal(t, int64(42), *proxies[0].QualityAccountID)
	require.Equal(t, "probe-account@example.com", proxies[0].QualityAccountName)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryUpsertsAccountQualitySnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	observedAt := time.Now().UTC()
	hasThinking := true
	httpStatus := 200
	mock.ExpectExec("INSERT INTO proxy_pool_account_quality_snapshots").
		WithArgs(
			int64(42), int64(7), int64(9), service.ProxyPoolQualityHealthy,
			float64(123.5), int64(64), int64(2000), int64(250), true,
			"passive", "quality observation recorded", "", 200, observedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewProxyPoolRepository(db)
	qualityRepo, ok := repo.(service.ProxyPoolAccountQualityRepository)
	require.True(t, ok)
	err = qualityRepo.UpsertAccountQualitySnapshot(context.Background(), service.ProxyPoolAccountQualitySnapshot{
		AccountID: 42, PoolID: 7, ProxyID: 9,
		QualityClass: service.ProxyPoolQualityHealthy,
		OutputTPS:    123.5, OutputTokens: 64, DurationMs: 2000, FirstTokenMs: 250,
		HasThinking: &hasThinking, Source: "passive", Reason: "quality observation recorded",
		HTTPStatus: &httpStatus, ObservedAt: observedAt,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryListsAccountQualitySnapshots(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	observedAt := time.Now().UTC()
	mock.ExpectQuery("SELECT q.account_id[\\s\\S]+proxy_pool_account_quality_snapshots").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "pool_id", "pool_name", "proxy_id", "proxy_name", "quality_class",
			"output_tps", "output_tokens", "duration_ms", "first_token_ms", "has_thinking",
			"source", "reason", "error_kind", "http_status", "observed_at",
		}).AddRow(
			int64(42), int64(7), "Grok pool", int64(9), "TN exit", service.ProxyPoolQualityHealthy,
			float64(123.5), int64(64), int64(2000), int64(250), true,
			"passive", "quality observation recorded", "", 200, observedAt,
		))

	repo := NewProxyPoolRepository(db)
	qualityRepo, ok := repo.(service.ProxyPoolAccountQualityRepository)
	require.True(t, ok)
	snapshots, err := qualityRepo.ListAccountQualitySnapshots(context.Background(), []int64{42})

	require.NoError(t, err)
	require.Contains(t, snapshots, int64(42))
	snapshot := snapshots[42]
	require.Equal(t, "Grok pool", snapshot.PoolName)
	require.Equal(t, "TN exit", snapshot.ProxyName)
	require.InDelta(t, 123.5, snapshot.OutputTPS, 0.001)
	require.NotNil(t, snapshot.HasThinking)
	require.True(t, *snapshot.HasThinking)
	require.NotNil(t, snapshot.HTTPStatus)
	require.Equal(t, 200, *snapshot.HTTPStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryCreateTranslatesDuplicateName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("INSERT INTO proxy_pools").
		WillReturnError(&pq.Error{Code: "23505"})

	repo := NewProxyPoolRepository(db)
	_, err = repo.CreatePool(context.Background(), &service.ProxyPool{
		Name:                  "primary",
		Status:                service.ProxyPoolStatusActive,
		HealthIntervalSeconds: 300,
		FailureThreshold:      2,
		AutoRebind:            true,
	})

	require.ErrorIs(t, err, service.ErrProxyPoolNameExists)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryUpdateTranslatesDuplicateName(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec("UPDATE proxy_pools").
		WillReturnError(&pq.Error{Code: "23505"})

	repo := NewProxyPoolRepository(db)
	err = repo.UpdatePool(context.Background(), &service.ProxyPool{
		ID:                    1,
		Name:                  "primary",
		Status:                service.ProxyPoolStatusActive,
		HealthIntervalSeconds: 300,
		FailureThreshold:      2,
		AutoRebind:            true,
	})

	require.True(t, errors.Is(err, service.ErrProxyPoolNameExists))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryDeleteRefreshesOnlyAffectedAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE accounts").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(11)).AddRow(int64(12)))
	mock.ExpectExec("UPDATE proxies").
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM proxy_pool_groups").
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE proxy_pools SET deleted_at").
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, []byte(`{"account_ids":[11,12]}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewProxyPoolRepository(db)
	require.NoError(t, repo.DeletePool(context.Background(), 7))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryUnbindRefreshesOnlyAffectedAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE accounts").
		WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(11)))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, []byte(`{"account_ids":[11]}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewProxyPoolRepository(db)
	affected, err := repo.UnbindAccountsFromPool(context.Background(), 7, []int64{11, 99})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryBindGroupsSynchronizesAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM groups").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(21)).AddRow(int64(22)))
	mock.ExpectQuery("SELECT group_id FROM proxy_pool_groups").
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO proxy_pool_groups").
		WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("WITH desired AS").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)).AddRow(int64(102)))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, []byte(`{"account_ids":[101,102],"pool_id":7}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewProxyPoolRepository(db)
	result, err := repo.BindGroupsToPool(context.Background(), 7, []int64{21, 22})

	require.NoError(t, err)
	require.Equal(t, 2, result.BoundGroups)
	require.Equal(t, 2, result.SyncedAccounts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryListGroupOptionsUsesPoolForOrdering(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("ORDER BY CASE WHEN ppg.pool_id = \\$1").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "platform", "status", "account_count", "pool_id", "pool_name",
		}).AddRow(int64(21), "Grok", service.PlatformGrok, service.StatusActive, int64(4), int64(7), "primary"))

	repo := NewProxyPoolRepository(db)
	groups, err := repo.ListPoolGroupOptions(context.Background(), 7)

	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.NotNil(t, groups[0].BoundPoolID)
	require.Equal(t, int64(7), *groups[0].BoundPoolID)
	require.Equal(t, "primary", groups[0].BoundPoolName)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryRejectsGroupOwnedByAnotherPool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM groups").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(21)))
	mock.ExpectQuery("SELECT group_id FROM proxy_pool_groups").
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(21)))
	mock.ExpectRollback()

	repo := NewProxyPoolRepository(db)
	_, err = repo.BindGroupsToPool(context.Background(), 7, []int64{21})

	require.ErrorIs(t, err, service.ErrProxyPoolGroupBound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProxyPoolRepositoryUnbindGroupsDetachesUncoveredAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT DISTINCT ag.account_id").
		WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(101)))
	mock.ExpectExec("DELETE FROM proxy_pool_groups").
		WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("UPDATE accounts a").
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountBulkChanged, nil, nil, []byte(`{"account_ids":[101]}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewProxyPoolRepository(db)
	result, err := repo.UnbindGroupsFromPool(context.Background(), 7, []int64{21})

	require.NoError(t, err)
	require.Equal(t, 1, result.UnboundGroups)
	require.Equal(t, 1, result.DetachedAccounts)
	require.NoError(t, mock.ExpectationsWereMet())
}
