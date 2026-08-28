package service

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	proxyPoolSSOQualityWorkerLimit = 12
	proxyPoolSSOQualityTimeout     = 30 * time.Second
	proxyPoolSSOQualityRunTimeout  = time.Hour
	proxyPoolSSOQualityBatchLimit  = 10000
)

type ProxyPoolSSOQualityStartResult struct {
	Started        bool `json:"started"`
	AlreadyRunning bool `json:"already_running"`
	AccountCount   int  `json:"account_count"`
}

// SetSSOQualityProber attaches the HTTP implementation after repository wiring.
func (s *ProxyPoolService) SetSSOQualityProber(prober ProxyPoolSSOQualityProber) {
	if s == nil {
		return
	}
	s.ssoQualityProber = prober
}

// StartSSOQualityCheck launches one background risk scan for every Grok OAuth
// account in the pool that has both a stored SSO cookie and a proxy assignment.
func (s *ProxyPoolService) StartSSOQualityCheck(ctx context.Context, poolID int64) (*ProxyPoolSSOQualityStartResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrProxyPoolNotFound
	}
	if s.ssoQualityRepo == nil || s.accountRepo == nil || s.ssoQualityProber == nil {
		return nil, ErrProxyPoolDisabled
	}
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if !pool.IsActive() {
		return nil, ErrProxyPoolDisabled
	}
	if _, running := s.ssoRuns.LoadOrStore(poolID, struct{}{}); running {
		return &ProxyPoolSSOQualityStartResult{AlreadyRunning: true}, nil
	}
	ids, err := s.ssoQualityRepo.ListGrokSSOQualityAccountIDs(ctx, poolID, proxyPoolSSOQualityBatchLimit)
	if err != nil {
		s.ssoRuns.Delete(poolID)
		return nil, err
	}
	if len(ids) == 0 {
		s.ssoRuns.Delete(poolID)
		return &ProxyPoolSSOQualityStartResult{AccountCount: 0}, nil
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.ssoRuns.Delete(poolID)
		runCtx, cancel := context.WithTimeout(context.Background(), proxyPoolSSOQualityRunTimeout)
		defer cancel()
		watchDone := make(chan struct{})
		defer close(watchDone)
		go func() {
			select {
			case <-s.stopCh:
				cancel()
			case <-watchDone:
			}
		}()
		s.runSSOQualityCheck(runCtx, pool.ID, ids)
	}()
	return &ProxyPoolSSOQualityStartResult{Started: true, AccountCount: len(ids)}, nil
}

func (s *ProxyPoolService) runSSOQualityCheck(ctx context.Context, poolID int64, ids []int64) {
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		log.Printf("[ProxyPool] SSO quality scan pool %d load accounts failed: %v", poolID, err)
		return
	}
	workers := proxyPoolSSOQualityWorkerLimit
	if workers > len(accounts) {
		workers = len(accounts)
	}
	jobs := make(chan *Account)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for account := range jobs {
				if ctx.Err() != nil {
					return
				}
				probeCtx, cancel := context.WithTimeout(ctx, proxyPoolSSOQualityTimeout)
				proxyURL := ""
				if account.Proxy != nil {
					proxyURL = account.Proxy.URL()
				}
				result := s.ssoQualityProber.ProbeGrokSSOQuality(probeCtx, account, proxyURL)
				cancel()
				proxyID := int64(0)
				if account.Proxy != nil {
					proxyID = account.Proxy.ID
				}
				snapshot := ProxyPoolAccountQualitySnapshot{
					AccountID:        account.ID,
					PoolID:           poolID,
					ProxyID:          proxyID,
					QualityClass:     ProxyPoolQualityUnknown,
					Source:           "sso",
					SSOState:         result.State,
					SSOReason:        result.Reason,
					SSOBotFlagSource: result.BotFlagSource,
					SSORisk:          result.Risk,
					SSOPolicy:        result.Policy,
					SSOEvent:         result.Event,
					SSOHTTPStatus:    result.HTTPStatus,
					SSOCheckedAt:     &result.CheckedAt,
				}
				writeCtx, writeCancel := context.WithTimeout(context.Background(), proxyPoolHealthWriteTimeout)
				if err := s.ssoQualityRepo.UpsertAccountSSOQualitySnapshot(writeCtx, snapshot); err != nil {
					log.Printf("[ProxyPool] SSO quality scan account %d persist failed: %v", account.ID, err)
				}
				writeCancel()
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range accounts {
			select {
			case jobs <- accounts[i]:
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
}
