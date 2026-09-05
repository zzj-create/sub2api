package service

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// The restored gate is synchronous and hard: unknown Grok OAuth tiers are
// treated as Free, and accounts are stopped at the nominal 500k rolling limit.
// Admin quota/import probes do not call these scheduler filters.

var grokFreeQuotaGateQueryFailureTotal atomic.Int64
var grokFreeQuotaGateBlockedTotal atomic.Int64

func (s *defaultOpenAIAccountScheduler) filterGrokFreeQuotaAccounts(ctx context.Context, accounts []Account) []Account {
	if s == nil || s.service == nil {
		return accounts
	}
	return filterGrokFreeQuotaAccountsCore(ctx, s.service.usageLogRepo, s.service.accountRepo, accounts)
}

func (s *GatewayService) filterGrokFreeQuotaAccountsForGateway(ctx context.Context, accounts []Account) []Account {
	if s == nil {
		return accounts
	}
	return filterGrokFreeQuotaAccountsCore(ctx, s.usageLogRepo, s.accountRepo, accounts)
}

func filterGrokFreeQuotaAccountsCore(
	ctx context.Context,
	usageLogRepo UsageLogRepository,
	accountRepo AccountRepository,
	accounts []Account,
) []Account {
	if len(accounts) == 0 || usageLogRepo == nil {
		return accounts
	}

	now := time.Now().UTC()
	usageCtx := withGrokFreeQuotaUsagePrefetch(ctx, usageLogRepo, accounts, now)
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		exhausted, tokens, known := grokFreeRollingQuotaExhausted(usageCtx, usageLogRepo, account, now)
		if known && exhausted {
			// Scheduler snapshots can lag a provider-tier transition or credential
			// refresh. Before durably rate-limiting an apparently-Free account,
			// re-read that one account and honor authoritative paid evidence from
			// the database (JWT tier, billing, credentials, or a Heavy 4.5 window).
			if durableGrokAccountExemptFromFreeRollingQuota(ctx, accountRepo, account.ID) {
				filtered = append(filtered, *account)
				continue
			}
			persistGrokFreeLocalUsageRateLimit(ctx, accountRepo, account, now, true, true)
			grokFreeQuotaGateBlockedTotal.Add(1)
			slog.Info("grok_free_quota_hard_gate_blocked",
				"account_id", account.ID,
				"tokens", tokens,
				"limit_tokens", xai.GrokFreeRolling24hTokenLimit,
				"window_hours", 24)
			continue
		}
		if grokOAuthUsesFreeRollingQuota(account) && !known {
			grokFreeQuotaGateQueryFailureTotal.Add(1)
		}
		filtered = append(filtered, *account)
	}
	return filtered
}

func durableGrokAccountExemptFromFreeRollingQuota(ctx context.Context, repo AccountRepository, accountID int64) bool {
	if repo == nil || accountID <= 0 {
		return false
	}
	durable, err := repo.GetByID(ctx, accountID)
	if err != nil || durable == nil || !durable.IsGrokOAuth() {
		return false
	}
	return !grokOAuthUsesFreeRollingQuota(durable)
}
