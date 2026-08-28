//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

type grokFreeQuotaUsageRepoStub struct {
	UsageLogRepository

	mu      sync.Mutex
	stats   map[int64]*usagestats.AccountStats
	err     error
	calls   int
	lastIDs []int64
	start   time.Time
}

type grokFreeQuotaAccountRepoStub struct {
	AccountRepository

	mu                   sync.Mutex
	accounts             []Account
	activeByID           map[int64]bool
	activationCalls      int
	rateLimitedCalls     int
	lastRateLimitedID    int64
	lastRateLimitResetAt time.Time
}

func (r *grokFreeQuotaAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

func (r *grokFreeQuotaAccountRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rateLimitedCalls++
	r.lastRateLimitedID = id
	r.lastRateLimitResetAt = resetAt
	return nil
}

func (r *grokFreeQuotaAccountRepoStub) SetRateLimitedIfInactive(_ context.Context, id int64, resetAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activationCalls++
	if r.activeByID == nil {
		r.activeByID = make(map[int64]bool)
	}
	if r.activeByID[id] {
		return false, nil
	}
	r.activeByID[id] = true
	r.rateLimitedCalls++
	r.lastRateLimitedID = id
	r.lastRateLimitResetAt = resetAt
	return true, nil
}

func (r *grokFreeQuotaUsageRepoStub) GetAccountWindowStatsBatch(_ context.Context, accountIDs []int64, start time.Time) (map[int64]*usagestats.AccountStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.lastIDs = append([]int64(nil), accountIDs...)
	r.start = start
	if r.err != nil {
		return nil, r.err
	}
	result := make(map[int64]*usagestats.AccountStats, len(accountIDs))
	for _, accountID := range accountIDs {
		if stats := r.stats[accountID]; stats != nil {
			copyStats := *stats
			result[accountID] = &copyStats
		}
	}
	return result, nil
}

func grokFreeQuotaTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.Grok.FreeQuotaSoftGateEnabled = true
	cfg.Gateway.Grok.FreeQuotaTokenLimit = xai.GrokFreeRolling24hTokenLimit
	cfg.Gateway.Grok.FreeQuotaSoftGatePercent = 95
	cfg.Gateway.Grok.FreeQuotaWindowHours = 24
	cfg.Gateway.Grok.FreeQuotaStatsCacheSeconds = 60
	return cfg
}

func TestGrokOAuthUsesFreeRollingQuotaDefaultsUnknownToFree(t *testing.T) {
	monthlyLimit := 10.0
	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{name: "nil", account: nil},
		{name: "api key", account: &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey}},
		{name: "unclassified oauth", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}, want: true},
		{name: "unknown oauth", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "unknown"}}, want: true},
		{name: "explicit free", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "free"}}, want: true},
		{name: "paid tier", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "supergrok"}}},
		{name: "paid plan type", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "pro"}}},
		{name: "paid billing", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{grokBillingExtraKey: &xai.BillingSummary{MonthlyLimitCents: &monthlyLimit}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, grokOAuthUsesFreeRollingQuota(tt.account))
		})
	}
}

func TestFilterGrokFreeQuotaAccountsUsesSynchronousHardLimit(t *testing.T) {
	repo := &grokFreeQuotaUsageRepoStub{stats: map[int64]*usagestats.AccountStats{
		1: {Tokens: xai.GrokFreeRolling24hTokenLimit},
		2: {Tokens: xai.GrokFreeRolling24hTokenLimit - 1},
		3: {Tokens: xai.GrokFreeRolling24hTokenLimit * 2},
		4: {Tokens: xai.GrokFreeRolling24hTokenLimit * 2},
		5: {Tokens: xai.GrokFreeRolling24hTokenLimit + 1},
	}}
	accountRepo := &grokFreeQuotaAccountRepoStub{}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{
		cfg: grokFreeQuotaTestConfig(), accountRepo: accountRepo, usageLogRepo: repo,
	}}
	accounts := []Account{
		{ID: 1, Platform: PlatformGrok, Type: AccountTypeOAuth},
		{ID: 2, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "free"}},
		{ID: 3, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "pro"}},
		{ID: 4, Platform: PlatformGrok, Type: AccountTypeAPIKey},
		{ID: 5, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"subscription_tier": "unknown"}},
	}

	filtered := scheduler.filterGrokFreeQuotaAccounts(context.Background(), accounts)

	require.Equal(t, []int64{2, 3, 4}, accountIDs(filtered))
	require.Equal(t, 1, repo.calls, "the first scheduling pass must synchronously query usage")
	require.Equal(t, []int64{1, 2, 5}, repo.lastIDs, "paid and API-key accounts are exempt")
	require.WithinDuration(t, time.Now().UTC().Add(-24*time.Hour), repo.start, time.Second)
	require.Equal(t, 2, accountRepo.rateLimitedCalls, "each exhausted account gets one durable rate-limit generation")
	require.WithinDuration(t, time.Now().Add(grokFreeLocalUsageCooldown), accountRepo.lastRateLimitResetAt, time.Second)
}

func TestFilterGrokFreeQuotaAccountsDoesNotExtendActiveCooldown(t *testing.T) {
	usageRepo := &grokFreeQuotaUsageRepoStub{stats: map[int64]*usagestats.AccountStats{
		9: {Tokens: xai.GrokFreeRolling24hTokenLimit},
	}}
	accountRepo := &grokFreeQuotaAccountRepoStub{}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{
		cfg: grokFreeQuotaTestConfig(), accountRepo: accountRepo, usageLogRepo: usageRepo,
	}}
	account := Account{ID: 9, Platform: PlatformGrok, Type: AccountTypeOAuth}

	require.Empty(t, scheduler.filterGrokFreeQuotaAccounts(context.Background(), []Account{account}))
	firstReset := accountRepo.lastRateLimitResetAt
	require.Empty(t, scheduler.filterGrokFreeQuotaAccounts(context.Background(), []Account{account}))

	require.Equal(t, 2, accountRepo.activationCalls)
	require.Equal(t, 1, accountRepo.rateLimitedCalls)
	require.Equal(t, firstReset, accountRepo.lastRateLimitResetAt)
}

func TestFilterGrokFreeQuotaAccountsStatsFailureFailsOpen(t *testing.T) {
	repo := &grokFreeQuotaUsageRepoStub{err: errors.New("usage database unavailable")}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{
		cfg: grokFreeQuotaTestConfig(), usageLogRepo: repo,
	}}
	account := Account{ID: 10, Platform: PlatformGrok, Type: AccountTypeOAuth}

	filtered := scheduler.filterGrokFreeQuotaAccounts(context.Background(), []Account{account})

	require.Equal(t, []int64{10}, accountIDs(filtered))
	require.Equal(t, 1, repo.calls)
}

func TestFilterGrokFreeQuotaAccountsRecoversAfterUsageFalls(t *testing.T) {
	repo := &grokFreeQuotaUsageRepoStub{stats: map[int64]*usagestats.AccountStats{
		11: {Tokens: xai.GrokFreeRolling24hTokenLimit},
	}}
	scheduler := &defaultOpenAIAccountScheduler{service: &OpenAIGatewayService{
		cfg: grokFreeQuotaTestConfig(), usageLogRepo: repo,
	}}
	account := Account{ID: 11, Platform: PlatformGrok, Type: AccountTypeOAuth}
	require.Empty(t, scheduler.filterGrokFreeQuotaAccounts(context.Background(), []Account{account}))

	repo.mu.Lock()
	repo.stats[11] = &usagestats.AccountStats{Tokens: xai.GrokFreeRolling24hTokenLimit - 1}
	repo.mu.Unlock()

	require.Equal(t, []int64{11}, accountIDs(scheduler.filterGrokFreeQuotaAccounts(context.Background(), []Account{account})))
}

func TestOpenAIAccountSchedulerLoadBalanceAppliesGrokFreeHardGate(t *testing.T) {
	cfg := grokFreeQuotaTestConfig()
	cfg.RunMode = config.RunModeSimple
	accounts := []Account{
		{ID: 21, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 22, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}
	svc := &OpenAIGatewayService{
		cfg:         cfg,
		accountRepo: &grokFreeQuotaAccountRepoStub{accounts: accounts},
		usageLogRepo: &grokFreeQuotaUsageRepoStub{stats: map[int64]*usagestats.AccountStats{
			21: {Tokens: xai.GrokFreeRolling24hTokenLimit},
			22: {Tokens: xai.GrokFreeRolling24hTokenLimit - 1},
		}},
	}
	scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}

	selection, _, _, _, err := scheduler.selectByLoadBalance(context.Background(), OpenAIAccountScheduleRequest{Platform: PlatformGrok})

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(22), selection.Account.ID)
}

func accountIDs(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	return ids
}
