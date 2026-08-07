package service

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	proxyPoolQualityWorkerLimit = 4
	proxyPoolQualityBatchLimit  = 8
)

func proxyQualityEligible(proxy *ProxyPoolProxy, pool *ProxyPool, now time.Time) bool {
	if proxy == nil || proxy.Status != StatusActive {
		return false
	}
	return !proxyQualityIsolated(proxy, now)
}

func proxyQualityIsolated(proxy *ProxyPoolProxy, now time.Time) bool {
	if proxy == nil {
		return false
	}
	if proxy.QuarantinedUntil != nil && proxy.QuarantinedUntil.After(now) {
		return true
	}
	if proxy.PoolHealth != ProxyPoolHealthUnhealthy {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(proxy.QualityClass)) {
	case ProxyPoolQualityHard, ProxyPoolQualityError:
		return true
	default:
		return false
	}
}

func poolQualityProbeDue(proxy *ProxyPoolProxy, pool *ProxyPool, force bool, now time.Time) bool {
	return poolQualityProbeDueForTarget(proxy, pool, force, 0, now)
}

func poolQualityProbeDueForTarget(proxy *ProxyPoolProxy, pool *ProxyPool, force bool, priorityProxyID int64, now time.Time) bool {
	if proxy == nil || pool == nil || proxy.Status != StatusActive {
		return false
	}
	if force || (priorityProxyID > 0 && proxy.ID == priorityProxyID) {
		return true
	}
	if proxy.QuarantinedUntil != nil {
		return !proxy.QuarantinedUntil.After(now)
	}
	if pool.Mode == ProxyPoolQualityModePassive {
		return false
	}
	if proxy.QualityProbedAt == nil {
		return true
	}
	return now.Sub(*proxy.QualityProbedAt) >= pool.ActiveInterval()
}

func (s *ProxyPoolService) probePoolQualityMembers(ctx context.Context, pool *ProxyPool, proxies []ProxyPoolProxy, force bool, now time.Time) {
	s.probePoolQualityMembersWithPriority(ctx, pool, proxies, force, 0, now)
}

func (s *ProxyPoolService) probePoolQualityMembersWithPriority(ctx context.Context, pool *ProxyPool, proxies []ProxyPoolProxy, force bool, priorityProxyID int64, now time.Time) {
	if s == nil || s.qualityProbe == nil || pool == nil {
		return
	}
	pool.ProxyPoolQualityPolicy.Normalize()
	due := make([]*ProxyPoolProxy, 0, len(proxies))
	for i := range proxies {
		if poolQualityProbeDueForTarget(&proxies[i], pool, force, priorityProxyID, now) {
			due = append(due, &proxies[i])
		}
	}
	if len(due) == 0 {
		return
	}
	sort.SliceStable(due, func(i, j int) bool {
		if priorityProxyID > 0 {
			leftPriority := due[i].ID == priorityProxyID
			rightPriority := due[j].ID == priorityProxyID
			if leftPriority != rightPriority {
				return leftPriority
			}
		}
		leftRecovery := due[i].QuarantinedUntil != nil && !due[i].QuarantinedUntil.After(now)
		rightRecovery := due[j].QuarantinedUntil != nil && !due[j].QuarantinedUntil.After(now)
		if leftRecovery != rightRecovery {
			return leftRecovery
		}
		if due[i].QualityProbedAt == nil && due[j].QualityProbedAt != nil {
			return true
		}
		if due[i].QualityProbedAt != nil && due[j].QualityProbedAt == nil {
			return false
		}
		if due[i].QualityProbedAt != nil && due[j].QualityProbedAt != nil && !due[i].QualityProbedAt.Equal(*due[j].QualityProbedAt) {
			return due[i].QualityProbedAt.Before(*due[j].QualityProbedAt)
		}
		return due[i].ID < due[j].ID
	})
	if len(due) > proxyPoolQualityBatchLimit {
		due = due[:proxyPoolQualityBatchLimit]
	}
	workerCount := proxyPoolQualityWorkerLimit
	if workerCount > len(due) {
		workerCount = len(due)
	}
	jobs := make(chan *ProxyPoolProxy)
	type qualityOutcome struct {
		proxy       *ProxyPoolProxy
		observation ProxyPoolQualityObservation
	}
	outcomes := make(chan qualityOutcome, workerCount)
	var workers sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for proxy := range jobs {
				if ctx.Err() != nil {
					return
				}
				probeCtx, cancel := context.WithTimeout(ctx, proxyPoolQualityProbeTimeout)
				observation := s.qualityProbe.ProbeProxyQuality(probeCtx, pool, proxy, pool.ProxyPoolQualityPolicy)
				cancel()
				if observation.Source == "" {
					observation.Source = "active"
				}
				select {
				case outcomes <- qualityOutcome{proxy: proxy, observation: observation}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, proxy := range due {
			select {
			case jobs <- proxy:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(outcomes)
	}()
	for outcome := range outcomes {
		s.applyProxyPoolQualityObservation(ctx, pool, outcome.proxy, proxies, outcome.observation)
	}
}

// applyProxyPoolQualityObservation updates the durable quality state and
// returns whether the observation isolated the exit or scheduled a recheck.
// The caller may pass the in-memory pool member list from a running sweep;
// passive observations load a fresh list before calling this method.
func (s *ProxyPoolService) applyProxyPoolQualityObservation(
	ctx context.Context,
	pool *ProxyPool,
	proxy *ProxyPoolProxy,
	all []ProxyPoolProxy,
	observation ProxyPoolQualityObservation,
) (quarantined, crossVerify bool) {
	if s == nil || s.repo == nil || pool == nil || proxy == nil {
		return false, false
	}
	policy := pool.ProxyPoolQualityPolicy
	policy.Normalize()
	now := time.Now().UTC()
	class := strings.ToLower(strings.TrimSpace(observation.Classification))
	if class == "" {
		class = ProxyPoolQualityUnknown
	}
	if class == ProxyPoolQualityError && !proxyPoolQualityErrorIsTransport(observation.ErrorKind) {
		class = ProxyPoolQualityIgnored
	}
	if observation.ErrorKind == ProxyPoolQualityErrorTransport {
		class = ProxyPoolQualityError
	}
	observation.OutputTPS = clampProxyPoolTPS(observation.OutputTPS)
	if observation.Source == "" {
		observation.Source = "passive"
	}
	reason := truncateProxyPoolQualityMessage(observation.Reason)
	if reason == "" {
		reason = qualityClassReason(class, observation.OutputTPS)
	}

	hadQuarantine := proxy.QuarantinedUntil != nil
	wasQuarantined := hadQuarantine && proxy.QuarantinedUntil.After(now)
	previousClass := strings.ToLower(strings.TrimSpace(proxy.QualityClass))
	shouldQuarantine := false
	shouldCrossVerify := false
	forceRecoveryIsolation := false
	quarantineReason := reason
	switch class {
	case ProxyPoolQualityHealthy:
		if !hadQuarantine || (observation.Source == "active" && !wasQuarantined) {
			proxy.QualityStrikes = 0
			proxy.QualityThinkingStrikes = 0
			proxy.QualityErrorStrikes = 0
			proxy.QualityClass = ProxyPoolQualityHealthy
			if proxy.Status == StatusActive && proxy.GrokQualityStatus == proxyPoolGrokQualityPass {
				proxy.PoolHealth = ProxyPoolHealthHealthy
				proxy.PoolFailures = 0
			}
			if observation.Source == "active" && proxy.QuarantinedUntil != nil && !proxy.QuarantinedUntil.After(now) {
				proxy.QuarantinedUntil = nil
			}
		}
	case ProxyPoolQualitySoft:
		proxy.QualityThinkingStrikes = 0
		proxy.QualityErrorStrikes = 0
		proxy.QualityStrikes++
		proxy.QualityClass = ProxyPoolQualitySoft
		if proxy.QualityStrikes >= policy.ConsecutiveSoft {
			if observation.Source == "passive" && policy.SoftCrossVerify {
				shouldCrossVerify = true
				quarantineReason = "soft threshold reached; active cross-verification scheduled"
			} else {
				shouldQuarantine = true
				quarantineReason = reason
			}
		}
	case ProxyPoolQualityHard:
		proxy.QualityErrorStrikes = 0
		if observation.Source == "passive" && !observation.HasThinking {
			proxy.QualityStrikes = 0
			proxy.QualityThinkingStrikes++
			if proxy.QualityThinkingStrikes < policy.ConsecutiveMissingThinking {
				proxy.QualityClass = ProxyPoolQualitySoft
				reason = "missing thinking signal; waiting for consecutive confirmation"
			} else if policy.ThinkingCrossVerify {
				shouldCrossVerify = true
				proxy.QualityClass = ProxyPoolQualitySoft
				quarantineReason = "missing thinking signal; active cross-verification scheduled"
			} else {
				proxy.QualityClass = ProxyPoolQualityHard
				shouldQuarantine = true
			}
		} else {
			proxy.QualityThinkingStrikes = 0
			proxy.QualityStrikes++
			proxy.QualityClass = ProxyPoolQualityHard
			shouldQuarantine = true
		}
	case ProxyPoolQualityError:
		proxy.QualityStrikes = 0
		proxy.QualityThinkingStrikes = 0
		proxy.QualityErrorStrikes++
		proxy.QualityClass = ProxyPoolQualityError
		if proxy.QualityErrorStrikes >= policy.ConsecutiveErrors {
			shouldQuarantine = true
		}
	case ProxyPoolQualityIgnored, ProxyPoolQualityUnknown:
		// Account, quota, upstream and tiny/no-output observations are kept for
		// diagnostics but never spend proxy strikes.
		if !hadQuarantine {
			proxy.QualityClass = class
		}
	}

	// An exit remains isolated until an active real-model probe succeeds. A
	// probe that runs after the cooldown has expired may return no account,
	// quota, upstream, or another quality anomaly; none of those observations
	// is evidence that the quarantined exit is safe to reuse.
	keepIsolated := hadQuarantine && !(observation.Source == "active" && !wasQuarantined && class == ProxyPoolQualityHealthy)
	if keepIsolated {
		shouldQuarantine = false
		shouldCrossVerify = false
		forceRecoveryIsolation = true
		proxy.PoolHealth = ProxyPoolHealthUnhealthy
		if previousClass == ProxyPoolQualityHard || previousClass == ProxyPoolQualityError {
			proxy.QualityClass = previousClass
		} else if proxy.QualityClass != ProxyPoolQualityHard && proxy.QualityClass != ProxyPoolQualityError {
			proxy.QualityClass = ProxyPoolQualityError
		}
		// No-account/quota results should not cause a tight active-probe loop.
		// A forced healthy check during an active quarantine does not shorten or
		// extend the configured cooldown; the normal recovery probe still runs at
		// its end.
		if !wasQuarantined || class != ProxyPoolQualityHealthy {
			retryFor := policy.QuarantineDuration()
			if policy.ActiveInterval() > retryFor {
				retryFor = policy.ActiveInterval()
			}
			until := now.Add(retryFor)
			proxy.QuarantinedUntil = &until
		}
		proxy.QualityLastReason = truncateProxyPoolQualityMessage("recovery probe did not pass: " + reason)
	}

	if shouldQuarantine {
		if len(all) == 0 {
			if loaded, err := s.repo.ListPoolProxies(ctx, pool.ID); err == nil {
				all = loaded
			}
		}
		if !forceRecoveryIsolation && countHealthyQualityProxies(all, pool, proxy.ID, now) < policy.MinHealthyProxies {
			// Keep the exit usable when isolating it would violate the pool's
			// minimum-health floor. This mirrors the CPA guard's suppression rule.
			shouldQuarantine = false
			shouldCrossVerify = false
			proxy.QualityClass = ProxyPoolQualitySoft
			reason = "quarantine suppressed by minimum healthy proxy floor: " + reason
			quarantineReason = reason
		}
	}
	if shouldQuarantine {
		until := now.Add(policy.QuarantineDuration())
		proxy.QuarantinedUntil = &until
		proxy.PoolHealth = ProxyPoolHealthUnhealthy
		if proxy.PoolFailures < pool.FailureThresholdValue() {
			proxy.PoolFailures = pool.FailureThresholdValue()
		}
		proxy.QualityLastReason = truncateProxyPoolQualityMessage(quarantineReason)
	}
	if shouldCrossVerify {
		proxy.QualityLastReason = truncateProxyPoolQualityMessage(quarantineReason)
	}
	if class == ProxyPoolQualityHealthy && proxy.PoolHealth == ProxyPoolHealthHealthy {
		proxy.QualityLastReason = ""
	}
	observedAt := now
	proxy.QualityOutputTPS = observation.OutputTPS
	proxy.QualityOutputTokens = observation.OutputTokens
	proxy.QualityDurationMs = observation.DurationMs
	proxy.QualityFirstTokenMs = observation.FirstTokenMs
	proxy.QualityLastSource = observation.Source
	if observation.Reason != "" && !shouldQuarantine && !shouldCrossVerify && !forceRecoveryIsolation {
		proxy.QualityLastReason = reason
	}
	proxy.QualityObservedAt = &observedAt
	if observation.Source == "active" {
		proxy.QualityProbedAt = &observedAt
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), proxyPoolHealthWriteTimeout)
	defer cancel()
	if err := s.repo.UpdateProxyPoolHealth(writeCtx, pool.ID, proxy.ID, proxyPoolHealthSnapshot(proxy)); err != nil {
		log.Printf("[ProxyPool] update quality for proxy %d failed: %v", proxy.ID, err)
	}
	if shouldQuarantine || shouldCrossVerify {
		return shouldQuarantine, shouldCrossVerify
	}
	return false, false
}

func proxyPoolQualityErrorIsTransport(kind string) bool {
	return kind == ProxyPoolQualityErrorTransport
}

func qualityClassReason(class string, tps float64) string {
	switch class {
	case ProxyPoolQualityHard:
		return "hard quality threshold reached"
	case ProxyPoolQualitySoft:
		return "soft quality threshold reached"
	case ProxyPoolQualityError:
		return "proxy transport error"
	case ProxyPoolQualityIgnored:
		return "observation ignored"
	default:
		if tps > 0 {
			return "quality observation recorded"
		}
		return "no quality signal"
	}
}

func proxyPoolHealthSnapshot(proxy *ProxyPoolProxy) ProxyPoolHealthSnapshot {
	if proxy == nil {
		return ProxyPoolHealthSnapshot{}
	}
	return ProxyPoolHealthSnapshot{
		Health:                 proxy.PoolHealth,
		Failures:               proxy.PoolFailures,
		CheckedAt:              timeValueOrNow(proxy.PoolCheckedAt),
		GrokQualityStatus:      proxy.GrokQualityStatus,
		GrokQualityCheckedAt:   proxy.GrokQualityCheckedAt,
		GrokQualityHTTPStatus:  proxy.GrokQualityHTTPStatus,
		GrokQualityMessage:     proxy.GrokQualityMessage,
		QualityClass:           proxy.QualityClass,
		QualityStrikes:         proxy.QualityStrikes,
		QualityThinkingStrikes: proxy.QualityThinkingStrikes,
		QualityErrorStrikes:    proxy.QualityErrorStrikes,
		QuarantinedUntil:       proxy.QuarantinedUntil,
		QualityOutputTPS:       proxy.QualityOutputTPS,
		QualityOutputTokens:    proxy.QualityOutputTokens,
		QualityDurationMs:      proxy.QualityDurationMs,
		QualityFirstTokenMs:    proxy.QualityFirstTokenMs,
		QualityLastSource:      proxy.QualityLastSource,
		QualityLastReason:      proxy.QualityLastReason,
		QualityObservedAt:      proxy.QualityObservedAt,
		QualityProbedAt:        proxy.QualityProbedAt,
	}
}

func timeValueOrNow(value *time.Time) time.Time {
	if value != nil {
		return *value
	}
	return time.Now().UTC()
}

func countHealthyQualityProxies(proxies []ProxyPoolProxy, pool *ProxyPool, excludeID int64, now time.Time) int {
	count := 0
	for i := range proxies {
		proxy := &proxies[i]
		if proxy.ID == excludeID || proxy.Status != StatusActive || proxy.PoolHealth != ProxyPoolHealthHealthy || proxy.GrokQualityStatus != proxyPoolGrokQualityPass {
			continue
		}
		if proxyQualityEligible(proxy, pool, now) {
			count++
		}
	}
	return count
}

// ObserveGrokResponse is the passive hot-path observer. It is deliberately
// best-effort and never returns an error to the gateway.
func (s *ProxyPoolService) ObserveGrokResponse(ctx context.Context, account *Account, result *OpenAIForwardResult) {
	if s == nil || s.repo == nil || s.qualityRepo == nil || account == nil || result == nil || !account.IsGrok() || result.Usage.OutputTokens <= 0 || result.Duration <= 0 || result.ImageCount > 0 || result.VideoCount > 0 {
		return
	}
	if account.ProxyID == nil || *account.ProxyID <= 0 {
		return
	}
	poolID, err := s.qualityRepo.GetPoolIDByProxyID(ctx, *account.ProxyID)
	if err != nil || poolID <= 0 {
		return
	}
	release, locked := s.acquirePoolRun(ctx, poolID)
	if !locked {
		return
	}
	defer func() {
		if locked {
			release()
		}
	}()
	pool, err := s.repo.GetPoolByID(ctx, poolID)
	if err != nil || pool == nil || pool.Mode == ProxyPoolQualityModeActive {
		return
	}
	proxies, err := s.repo.ListPoolProxies(ctx, poolID)
	if err != nil {
		return
	}
	var target *ProxyPoolProxy
	for i := range proxies {
		if proxies[i].ID == *account.ProxyID {
			target = &proxies[i]
			break
		}
	}
	if target == nil {
		return
	}
	firstTokenMs := int64(0)
	if result.FirstTokenMs != nil {
		firstTokenMs = int64(*result.FirstTokenMs)
	}
	policy := pool.ProxyPoolQualityPolicy
	policy.Normalize()
	if target.QualityObservedAt != nil && time.Since(*target.QualityObservedAt) < policy.PassiveWindow() {
		return
	}
	obs := ProxyPoolQualityObservation{
		Classification: ClassifyProxyPoolQuality(
			ComputeProxyPoolTPS(int64(result.Usage.OutputTokens), result.Duration.Milliseconds(), firstTokenMs, policy.MinGenerationMs),
			int64(result.Usage.OutputTokens), result.HasThinking, policy,
		),
		OutputTokens: int64(result.Usage.OutputTokens),
		DurationMs:   result.Duration.Milliseconds(),
		FirstTokenMs: firstTokenMs,
		HasThinking:  result.HasThinking,
		Source:       "passive",
	}
	obs.OutputTPS = clampProxyPoolTPS(ComputeProxyPoolTPS(obs.OutputTokens, obs.DurationMs, obs.FirstTokenMs, policy.MinGenerationMs))
	quarantined, crossVerify := s.applyProxyPoolQualityObservation(ctx, pool, target, proxies, obs)
	release()
	locked = false
	if quarantined || crossVerify {
		// Rebind is handled by the normal pool run so account changes and group
		// synchronization use the same distributed lock as scheduled sweeps.
		if crossVerify {
			s.startPoolQualityVerification(pool, target.ID)
		} else {
			s.startPoolRun(pool, false)
		}
	}
}
