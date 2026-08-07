package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type recordingProxyPoolQualityProber struct {
	mu  sync.Mutex
	ids []int64
}

type groupSyncAwareQualityProber struct {
	repo    *proxyPoolServiceTestRepo
	sawSync bool
}

type qualityProbeAccountRepo struct {
	AccountRepository
	account *Account
}

type qualityProbeRepository struct {
	accountIDs []int64
}

type qualityProbeHTTPUpstream struct {
	request  *http.Request
	proxyURL string
}

type accountStateCall struct {
	accountID int64
	until     time.Time
	reason    string
}

type recordingProxyPoolAccountState struct {
	calls []accountStateCall
}

type failAfterDataReader struct {
	data   []byte
	served bool
}

func (r *failAfterDataReader) Read(p []byte) (int, error) {
	if r.served {
		return 0, errors.New("reader was consumed after the SSE done marker")
	}
	r.served = true
	return copy(p, r.data), nil
}

func (r *qualityProbeAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, errors.New("account not found")
	}
	return r.account, nil
}

func (r *qualityProbeRepository) GetPoolIDByProxyID(_ context.Context, _ int64) (int64, error) {
	return 1, nil
}

func (r *qualityProbeRepository) ListGrokProbeAccountIDs(_ context.Context, _, _ int64, _ int) ([]int64, error) {
	return append([]int64(nil), r.accountIDs...), nil
}

func (u *qualityProbeHTTPUpstream) Do(req *http.Request, proxyURL string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	u.proxyURL = proxyURL
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"choices":[{"delta":{"reasoning_content":"plan","content":"answer"}}]}`,
			`data: {"usage":{"completion_tokens":64,"output_tokens_details":{"reasoning_tokens":8}}}`,
			`data: [DONE]`,
		}, "\n"))),
	}, nil
}

func (u *qualityProbeHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (r *recordingProxyPoolAccountState) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.calls = append(r.calls, accountStateCall{accountID: id, until: until, reason: reason})
	return nil
}

func (p *recordingProxyPoolQualityProber) ProbeProxyQuality(_ context.Context, _ *ProxyPool, proxy *ProxyPoolProxy, _ ProxyPoolQualityPolicy) ProxyPoolQualityObservation {
	p.mu.Lock()
	p.ids = append(p.ids, proxy.ID)
	p.mu.Unlock()
	return ProxyPoolQualityObservation{Classification: ProxyPoolQualityIgnored, ErrorKind: ProxyPoolQualityErrorNoAccount, Source: "active"}
}

func (p *groupSyncAwareQualityProber) ProbeProxyQuality(_ context.Context, _ *ProxyPool, _ *ProxyPoolProxy, _ ProxyPoolQualityPolicy) ProxyPoolQualityObservation {
	p.repo.mu.Lock()
	p.sawSync = p.repo.groupSynced
	p.repo.mu.Unlock()
	return ProxyPoolQualityObservation{Classification: ProxyPoolQualityIgnored, ErrorKind: ProxyPoolQualityErrorNoAccount, Source: "active"}
}

func qualityGuardTestPool() *ProxyPool {
	policy := DefaultProxyPoolQualityPolicy()
	policy.MinHealthyProxies = 1
	policy.ConsecutiveErrors = 2
	policy.ConsecutiveMissingThinking = 2
	policy.QuarantineSeconds = 30
	return &ProxyPool{
		ID:                     1,
		Status:                 ProxyPoolStatusActive,
		HealthIntervalSeconds:  300,
		FailureThreshold:       2,
		AutoRebind:             true,
		ProxyPoolQualityPolicy: policy,
	}
}

func qualityGuardTestProxy(id int64, poolID int64, health string) ProxyPoolProxy {
	now := time.Now().UTC()
	return ProxyPoolProxy{
		Proxy:             Proxy{ID: id, Name: "quality-proxy", Protocol: "http", Host: "proxy.test", Port: 8080, Status: StatusActive},
		PoolID:            poolID,
		PoolHealth:        health,
		PoolCheckedAt:     &now,
		GrokQualityStatus: proxyPoolGrokQualityPass,
	}
}

func applyQualityTestObservation(t *testing.T, svc *ProxyPoolService, repo *proxyPoolServiceTestRepo, pool *ProxyPool, proxyID int64, observation ProxyPoolQualityObservation) (bool, bool) {
	t.Helper()
	proxy := repo.proxies[proxyID]
	all, err := repo.ListPoolProxies(context.Background(), pool.ID)
	require.NoError(t, err)
	return svc.applyProxyPoolQualityObservation(context.Background(), pool, &proxy, all, observation)
}

func TestComputeProxyPoolTPSUsesGenerationWindow(t *testing.T) {
	require.InDelta(t, 100.0, ComputeProxyPoolTPS(100, 2000, 1000, 1000), 0.001)
	// A response whose generation window is below the guard floor falls back to
	// total duration; if the total is also too short it is not a quality sample.
	require.Zero(t, ComputeProxyPoolTPS(100, 500, 400, 1000))
}

func TestProxyPoolQualityPolicyPatchPreservesOmittedBooleans(t *testing.T) {
	base := DefaultProxyPoolQualityPolicy()
	mode := ProxyPoolQualityModePassive
	patched := (&ProxyPoolQualityPolicyPatch{Mode: &mode}).Apply(base)
	require.Equal(t, ProxyPoolQualityModePassive, patched.Mode)
	require.True(t, patched.ThinkingGuard)
	require.True(t, patched.ThinkingCrossVerify)
	require.True(t, patched.SoftCrossVerify)

	disabled := false
	patched = (&ProxyPoolQualityPolicyPatch{ThinkingGuard: &disabled}).Apply(base)
	require.False(t, patched.ThinkingGuard)
	require.True(t, patched.ThinkingCrossVerify)
}

func TestProxyPoolQualityProbeRunsAfterGroupAccountSync(t *testing.T) {
	pool := qualityGuardTestPool()
	repo := newProxyPoolServiceTestRepo(pool, qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy))
	prober := &groupSyncAwareQualityProber{repo: repo}
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)
	svc.SetQualityProber(prober)

	svc.runPoolLocked(context.Background(), pool, false)

	require.True(t, prober.sawSync)
}

func TestParseGrokQualitySSERecordsFirstTokenAndResponsesThinking(t *testing.T) {
	started := time.Unix(100, 0)
	clock := []time.Time{started.Add(250 * time.Millisecond), started.Add(2250 * time.Millisecond)}
	now := func() time.Time {
		if len(clock) == 0 {
			return started.Add(2250 * time.Millisecond)
		}
		value := clock[0]
		clock = clock[1:]
		return value
	}
	policy := DefaultProxyPoolQualityPolicy()
	policy.ThinkingGuard = false
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"item":{"type":"reasoning","summary":[{"text":"plan"}]}}`,
		`data: {"usage":{"completion_tokens":40,"output_tokens_details":{"reasoning_tokens":8}}}`,
		`data: [DONE]`,
	}, "\n")

	observation := parseGrokQualitySSEWithClock(bytes.NewBufferString(body), started, policy, now)

	require.Equal(t, int64(250), observation.FirstTokenMs)
	require.Equal(t, int64(2250), observation.DurationMs)
	require.Equal(t, int64(40), observation.OutputTokens)
	require.True(t, observation.HasThinking)
	require.Equal(t, ProxyPoolQualityHealthy, observation.Classification)
}

func TestParseGrokQualitySSEStopsAtDoneMarker(t *testing.T) {
	started := time.Unix(200, 0)
	policy := DefaultProxyPoolQualityPolicy()
	policy.ThinkingGuard = false
	reader := &failAfterDataReader{data: []byte(strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hello"}}],"usage":{"completion_tokens":40}}`,
		`data: [DONE]`,
		``,
	}, "\n"))}

	observation := parseGrokQualitySSEWithClock(reader, started, policy, func() time.Time {
		return started.Add(2 * time.Second)
	})

	require.Empty(t, observation.ErrorKind)
	require.Equal(t, int64(40), observation.OutputTokens)
	require.Equal(t, ProxyPoolQualityHealthy, observation.Classification)
}

func TestClassifyProxyPoolQualityFailureKeepsCredentialErrorsSeparate(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests} {
		require.Equal(t, ProxyPoolQualityErrorAccount, classifyProxyPoolQualityFailure(status, "quota exhausted"))
	}
	require.Equal(t, ProxyPoolQualityErrorTransport, classifyProxyPoolQualityFailure(http.StatusProxyAuthRequired, "proxy auth"))
	require.Equal(t, ProxyPoolQualityErrorTransport, classifyProxyPoolQualityFailure(0, "connection reset by peer"))
}

func TestGrokQualityProbeReadsAPIKeyCredentials(t *testing.T) {
	prober := &grokEgressQualityProber{}
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "grok-api-key",
		},
	}

	token, err := prober.accessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "grok-api-key", token)
}

func TestGrokQualityProbeForcesTargetProxy(t *testing.T) {
	account := &Account{
		ID:       9,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "grok-api-key",
		},
	}
	upstream := &qualityProbeHTTPUpstream{}
	prober := &grokEgressQualityProber{
		accountRepo:  &qualityProbeAccountRepo{account: account},
		qualityRepo:  &qualityProbeRepository{accountIDs: []int64{account.ID}},
		httpUpstream: upstream,
	}
	pool := qualityGuardTestPool()
	proxy := qualityGuardTestProxy(3, pool.ID, ProxyPoolHealthHealthy)

	observation := prober.ProbeProxyQuality(context.Background(), pool, &proxy, pool.ProxyPoolQualityPolicy)

	require.NotNil(t, upstream.request)
	require.Equal(t, proxy.URL(), upstream.proxyURL)
	require.Equal(t, "/v1/chat/completions", upstream.request.URL.Path)
	require.Equal(t, "Bearer grok-api-key", upstream.request.Header.Get("Authorization"))
	require.Equal(t, account.ID, observation.AccountID)
}

func TestProxyPoolQualityAccountErrorDoesNotSpendProxyStrikes(t *testing.T) {
	pool := qualityGuardTestPool()
	repo := newProxyPoolServiceTestRepo(pool,
		qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy),
		qualityGuardTestProxy(2, pool.ID, ProxyPoolHealthHealthy),
	)
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)

	quarantined, crossVerify := applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityError,
		ErrorKind:      ProxyPoolQualityErrorAccount,
		Source:         "active",
		Reason:         "Grok upstream HTTP 429",
	})

	proxy := repo.proxies[2]
	require.False(t, quarantined)
	require.False(t, crossVerify)
	require.Zero(t, proxy.QualityStrikes)
	require.Zero(t, proxy.QualityErrorStrikes)
	require.Nil(t, proxy.QuarantinedUntil)
	require.Equal(t, ProxyPoolQualityIgnored, proxy.QualityClass)
}

func TestProxyPoolQualityObservationTracksAndClearsProbeAccount(t *testing.T) {
	pool := qualityGuardTestPool()
	repo := newProxyPoolServiceTestRepo(pool,
		qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy),
		qualityGuardTestProxy(2, pool.ID, ProxyPoolHealthHealthy),
	)
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)

	applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityIgnored,
		ErrorKind:      ProxyPoolQualityErrorAccount,
		Source:         "active",
		AccountID:      99,
		Reason:         "probe quota exhausted",
	})
	require.NotNil(t, repo.proxies[2].QualityAccountID)
	require.Equal(t, int64(99), *repo.proxies[2].QualityAccountID)

	applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityIgnored,
		ErrorKind:      ProxyPoolQualityErrorNoAccount,
		Source:         "active",
		Reason:         "no schedulable Grok account is available for this pool",
	})
	require.Nil(t, repo.proxies[2].QualityAccountID)
}

func TestProxyPoolQualityTransportErrorsQuarantineAfterConsecutiveHits(t *testing.T) {
	pool := qualityGuardTestPool()
	repo := newProxyPoolServiceTestRepo(pool,
		qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy),
		qualityGuardTestProxy(2, pool.ID, ProxyPoolHealthHealthy),
	)
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)
	observation := ProxyPoolQualityObservation{Classification: ProxyPoolQualityError, ErrorKind: ProxyPoolQualityErrorTransport, Source: "active"}

	first, _ := applyQualityTestObservation(t, svc, repo, pool, 2, observation)
	firstSnapshot := repo.proxies[2]
	second, _ := applyQualityTestObservation(t, svc, repo, pool, 2, observation)

	require.False(t, first)
	require.True(t, proxyQualityEligible(&firstSnapshot, pool, time.Now()))
	require.Equal(t, ProxyPoolHealthHealthy, firstSnapshot.PoolHealth)
	require.Nil(t, firstSnapshot.QuarantinedUntil)
	require.True(t, second)
	require.NotNil(t, repo.proxies[2].QuarantinedUntil)
	require.Equal(t, ProxyPoolHealthUnhealthy, repo.proxies[2].PoolHealth)
}

func TestProxyPoolQualityRecoveryRequiresHealthyActiveProbe(t *testing.T) {
	pool := qualityGuardTestPool()
	proxy := qualityGuardTestProxy(2, pool.ID, ProxyPoolHealthUnhealthy)
	proxy.QualityClass = ProxyPoolQualityHard
	expired := time.Now().UTC().Add(-time.Second)
	proxy.QuarantinedUntil = &expired
	repo := newProxyPoolServiceTestRepo(pool,
		qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy), proxy,
	)
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)

	applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityIgnored,
		ErrorKind:      ProxyPoolQualityErrorNoAccount,
		Source:         "active",
	})
	keptIsolated := repo.proxies[2]
	require.Equal(t, ProxyPoolHealthUnhealthy, keptIsolated.PoolHealth)
	require.Equal(t, ProxyPoolQualityHard, keptIsolated.QualityClass)
	require.NotNil(t, keptIsolated.QuarantinedUntil)
	require.True(t, keptIsolated.QuarantinedUntil.After(time.Now()))

	// Simulate the next retry after the renewed cooldown.
	retryExpired := time.Now().UTC().Add(-time.Second)
	keptIsolated.QuarantinedUntil = &retryExpired
	repo.proxies[2] = keptIsolated
	applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityHealthy,
		OutputTokens:   100,
		OutputTPS:      50,
		Source:         "active",
	})
	recovered := repo.proxies[2]
	require.Equal(t, ProxyPoolHealthHealthy, recovered.PoolHealth)
	require.Equal(t, ProxyPoolQualityHealthy, recovered.QualityClass)
	require.Nil(t, recovered.QuarantinedUntil)
}

func TestProxyPoolQualityMinimumHealthyFloorSuppressesIsolation(t *testing.T) {
	pool := qualityGuardTestPool()
	repo := newProxyPoolServiceTestRepo(pool, qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy))
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)

	quarantined, _ := applyQualityTestObservation(t, svc, repo, pool, 1, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityHard,
		OutputTokens:   100,
		OutputTPS:      2000,
		HasThinking:    true,
		Source:         "active",
	})

	require.False(t, quarantined)
	require.Nil(t, repo.proxies[1].QuarantinedUntil)
	require.Equal(t, ProxyPoolHealthHealthy, repo.proxies[1].PoolHealth)
	require.Equal(t, ProxyPoolQualitySoft, repo.proxies[1].QualityClass)
}

func TestProxyPoolQualityRebindFailureTemporarilyDisablesRemainingAccounts(t *testing.T) {
	pool := qualityGuardTestPool()
	pool.DisableAccountOnHard = true
	healthy := qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy)
	isolated := qualityGuardTestProxy(2, pool.ID, ProxyPoolHealthUnhealthy)
	isolated.QualityClass = ProxyPoolQualityHard
	quarantinedUntil := time.Now().UTC().Add(time.Minute)
	isolated.QuarantinedUntil = &quarantinedUntil
	repo := newProxyPoolServiceTestRepo(pool, healthy, isolated)
	repo.accountProxy[101] = isolated.ID
	repo.counts[isolated.ID] = 1
	repo.bindErr = errors.New("binding write failed")
	accountState := &recordingProxyPoolAccountState{}
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)
	svc.SetAccountStateRepository(accountState)

	changed := svc.rebindUnhealthy(context.Background(), pool, []ProxyPoolProxy{healthy, isolated}, []*ProxyPoolProxy{&healthy})

	require.Zero(t, changed)
	require.Len(t, accountState.calls, 1)
	require.Equal(t, int64(101), accountState.calls[0].accountID)
	require.WithinDuration(t, quarantinedUntil, accountState.calls[0].until, time.Second)
	require.Contains(t, accountState.calls[0].reason, "proxy 2")
}

func TestProxyPoolQualityThinkingStrikesAreIndependentFromSoftStrikes(t *testing.T) {
	pool := qualityGuardTestPool()
	repo := newProxyPoolServiceTestRepo(pool,
		qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy),
		qualityGuardTestProxy(2, pool.ID, ProxyPoolHealthHealthy),
	)
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)

	applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualitySoft, OutputTokens: 100, OutputTPS: 600, Source: "passive",
	})
	_, crossVerify := applyQualityTestObservation(t, svc, repo, pool, 2, ProxyPoolQualityObservation{
		Classification: ProxyPoolQualityHard, OutputTokens: 100, OutputTPS: 10, HasThinking: false, Source: "passive",
	})

	require.False(t, crossVerify)
	require.Zero(t, repo.proxies[2].QualityStrikes)
	require.Equal(t, 1, repo.proxies[2].QualityThinkingStrikes)
}

func TestProxyPoolQualitySweepIsBoundedAndPrioritizesRecovery(t *testing.T) {
	pool := qualityGuardTestPool()
	proxies := make([]ProxyPoolProxy, 0, proxyPoolQualityBatchLimit+3)
	for id := int64(1); id <= int64(proxyPoolQualityBatchLimit+3); id++ {
		proxies = append(proxies, qualityGuardTestProxy(id, pool.ID, ProxyPoolHealthHealthy))
	}
	expired := time.Now().UTC().Add(-time.Second)
	proxies[len(proxies)-1].QuarantinedUntil = &expired
	proxies[len(proxies)-1].QualityClass = ProxyPoolQualityHard
	proxies[len(proxies)-1].PoolHealth = ProxyPoolHealthUnhealthy
	repo := newProxyPoolServiceTestRepo(pool, proxies...)
	prober := &recordingProxyPoolQualityProber{}
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)
	svc.SetQualityProber(prober)

	svc.probePoolQualityMembers(context.Background(), pool, proxies, false, time.Now().UTC())

	prober.mu.Lock()
	ids := append([]int64(nil), prober.ids...)
	prober.mu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	require.Len(t, ids, proxyPoolQualityBatchLimit)
	require.Contains(t, ids, int64(proxyPoolQualityBatchLimit+3))
}

func TestProxyPoolQualityTargetedSweepOnlyForcesRequestedProxy(t *testing.T) {
	pool := qualityGuardTestPool()
	pool.Mode = ProxyPoolQualityModePassive
	now := time.Now().UTC()
	proxies := make([]ProxyPoolProxy, 0, proxyPoolQualityBatchLimit+3)
	for id := int64(1); id <= int64(proxyPoolQualityBatchLimit+3); id++ {
		proxy := qualityGuardTestProxy(id, pool.ID, ProxyPoolHealthHealthy)
		proxy.QualityObservedAt = &now
		proxies = append(proxies, proxy)
	}
	targetID := int64(proxyPoolQualityBatchLimit + 3)
	repo := newProxyPoolServiceTestRepo(pool, proxies...)
	prober := &recordingProxyPoolQualityProber{}
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)
	svc.SetQualityProber(prober)

	svc.probePoolQualityMembersWithPriority(context.Background(), pool, proxies, false, targetID, now)

	prober.mu.Lock()
	ids := append([]int64(nil), prober.ids...)
	prober.mu.Unlock()
	require.Equal(t, []int64{targetID}, ids)
}

func TestProxyPoolQualityHybridActiveScheduleIgnoresFreshPassiveObservation(t *testing.T) {
	pool := qualityGuardTestPool()
	pool.Mode = ProxyPoolQualityModeHybrid
	pool.ActiveIntervalSeconds = 60
	now := time.Now().UTC()
	lastActive := now.Add(-2 * time.Minute)
	freshPassive := now.Add(-time.Second)
	proxy := qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthHealthy)
	proxy.QualityProbedAt = &lastActive
	proxy.QualityObservedAt = &freshPassive
	proxy.QualityLastSource = "passive"

	require.True(t, poolQualityProbeDue(&proxy, pool, false, now))
}

func TestProxyPoolConnectivityProbeCannotClearQualityIsolation(t *testing.T) {
	pool := qualityGuardTestPool()
	now := time.Now().UTC()
	proxy := qualityGuardTestProxy(1, pool.ID, ProxyPoolHealthUnhealthy)
	proxy.QualityClass = ProxyPoolQualityHard
	quarantinedUntil := now.Add(time.Minute)
	proxy.QuarantinedUntil = &quarantinedUntil
	repo := newProxyPoolServiceTestRepo(pool, proxy)
	svc := NewProxyPoolService(repo, nil, nil, nil, nil)

	svc.applyPoolProbeResult(pool, &proxy, poolProbeResult{
		ok:            true,
		grokItem:      ProxyQualityCheckItem{Target: "grok", Status: proxyPoolGrokQualityPass},
		grokCheckedAt: now,
	}, now)

	updated := repo.proxies[proxy.ID]
	require.Equal(t, ProxyPoolHealthUnhealthy, updated.PoolHealth)
	require.GreaterOrEqual(t, updated.PoolFailures, pool.FailureThresholdValue())
	require.Empty(t, poolProxiesDueForProbe([]ProxyPoolProxy{updated}, pool, false, now))
	require.Len(t, poolProxiesDueForProbe([]ProxyPoolProxy{updated}, pool, true, now), 1)
}

func TestProxyPoolQualityUnknownTransportIsNotAccountError(t *testing.T) {
	require.NotEqual(t, ProxyPoolQualityErrorAccount, classifyProxyPoolQualityFailure(0, errors.New("connection reset").Error()))
}
