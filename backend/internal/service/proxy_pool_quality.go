package service

import (
	"math"
	"net/http"
	"strings"
	"time"
)

// Quality guard modes mirror the CPA plugin semantics while remaining native
// to Sub2API's proxy-pool scheduler.
const (
	ProxyPoolQualityModePassive = "passive"
	ProxyPoolQualityModeActive  = "active"
	ProxyPoolQualityModeHybrid  = "hybrid"

	ProxyPoolQualityUnknown = "unknown"
	ProxyPoolQualityHealthy = "healthy"
	ProxyPoolQualityIgnored = "ignored"
	ProxyPoolQualitySoft    = "soft"
	ProxyPoolQualityHard    = "hard"
	ProxyPoolQualityError   = "error"
)

// ProxyPoolQualityPolicy is persisted per pool. Zero values are normalized to
// conservative defaults so pools created before the quality guard migration
// keep their original health-check behavior.
type ProxyPoolQualityPolicy struct {
	Mode                       string  `json:"quality_mode"`
	ActiveIntervalSeconds      int     `json:"active_interval_seconds"`
	PassiveWindowSeconds       int     `json:"passive_window_seconds"`
	QuarantineSeconds          int     `json:"quarantine_seconds"`
	SoftTPS                    float64 `json:"soft_tps"`
	HardTPS                    float64 `json:"hard_tps"`
	ConsecutiveSoft            int     `json:"consecutive_soft"`
	ConsecutiveErrors          int     `json:"consecutive_errors"`
	MinHealthyProxies          int     `json:"min_healthy_proxies"`
	MinGenerationMs            int64   `json:"min_generation_ms"`
	MinOutputTokens            int64   `json:"min_output_tokens"`
	Model                      string  `json:"quality_model"`
	DisableAccountOnHard       bool    `json:"disable_account_on_hard"`
	ThinkingGuard              bool    `json:"thinking_guard"`
	ConsecutiveMissingThinking int     `json:"consecutive_missing_thinking"`
	ThinkingCrossVerify        bool    `json:"thinking_cross_verify"`
	SoftCrossVerify            bool    `json:"soft_cross_verify"`
	MaxOutputTokensProbe       int     `json:"max_output_tokens_probe"`
}

// ProxyPoolQualityPolicyPatch preserves field presence for the nested admin
// API object. In particular, a missing boolean must not be confused with an
// explicit false when an existing pool is edited.
type ProxyPoolQualityPolicyPatch struct {
	Mode                       *string  `json:"quality_mode"`
	ActiveIntervalSeconds      *int     `json:"active_interval_seconds"`
	PassiveWindowSeconds       *int     `json:"passive_window_seconds"`
	QuarantineSeconds          *int     `json:"quarantine_seconds"`
	SoftTPS                    *float64 `json:"soft_tps"`
	HardTPS                    *float64 `json:"hard_tps"`
	ConsecutiveSoft            *int     `json:"consecutive_soft"`
	ConsecutiveErrors          *int     `json:"consecutive_errors"`
	MinHealthyProxies          *int     `json:"min_healthy_proxies"`
	MinGenerationMs            *int64   `json:"min_generation_ms"`
	MinOutputTokens            *int64   `json:"min_output_tokens"`
	Model                      *string  `json:"quality_model"`
	DisableAccountOnHard       *bool    `json:"disable_account_on_hard"`
	ThinkingGuard              *bool    `json:"thinking_guard"`
	ConsecutiveMissingThinking *int     `json:"consecutive_missing_thinking"`
	ThinkingCrossVerify        *bool    `json:"thinking_cross_verify"`
	SoftCrossVerify            *bool    `json:"soft_cross_verify"`
	MaxOutputTokensProbe       *int     `json:"max_output_tokens_probe"`
}

func (p *ProxyPoolQualityPolicyPatch) Apply(base ProxyPoolQualityPolicy) ProxyPoolQualityPolicy {
	if p == nil {
		base.Normalize()
		return base
	}
	if p.Mode != nil {
		base.Mode = *p.Mode
	}
	if p.ActiveIntervalSeconds != nil {
		base.ActiveIntervalSeconds = *p.ActiveIntervalSeconds
	}
	if p.PassiveWindowSeconds != nil {
		base.PassiveWindowSeconds = *p.PassiveWindowSeconds
	}
	if p.QuarantineSeconds != nil {
		base.QuarantineSeconds = *p.QuarantineSeconds
	}
	if p.SoftTPS != nil {
		base.SoftTPS = *p.SoftTPS
	}
	if p.HardTPS != nil {
		base.HardTPS = *p.HardTPS
	}
	if p.ConsecutiveSoft != nil {
		base.ConsecutiveSoft = *p.ConsecutiveSoft
	}
	if p.ConsecutiveErrors != nil {
		base.ConsecutiveErrors = *p.ConsecutiveErrors
	}
	if p.MinHealthyProxies != nil {
		base.MinHealthyProxies = *p.MinHealthyProxies
	}
	if p.MinGenerationMs != nil {
		base.MinGenerationMs = *p.MinGenerationMs
	}
	if p.MinOutputTokens != nil {
		base.MinOutputTokens = *p.MinOutputTokens
	}
	if p.Model != nil {
		base.Model = *p.Model
	}
	if p.DisableAccountOnHard != nil {
		base.DisableAccountOnHard = *p.DisableAccountOnHard
	}
	if p.ThinkingGuard != nil {
		base.ThinkingGuard = *p.ThinkingGuard
	}
	if p.ConsecutiveMissingThinking != nil {
		base.ConsecutiveMissingThinking = *p.ConsecutiveMissingThinking
	}
	if p.ThinkingCrossVerify != nil {
		base.ThinkingCrossVerify = *p.ThinkingCrossVerify
	}
	if p.SoftCrossVerify != nil {
		base.SoftCrossVerify = *p.SoftCrossVerify
	}
	if p.MaxOutputTokensProbe != nil {
		base.MaxOutputTokensProbe = *p.MaxOutputTokensProbe
	}
	base.Normalize()
	return base
}

func DefaultProxyPoolQualityPolicy() ProxyPoolQualityPolicy {
	return ProxyPoolQualityPolicy{
		Mode:                       ProxyPoolQualityModeHybrid,
		ActiveIntervalSeconds:      1800,
		PassiveWindowSeconds:       300,
		QuarantineSeconds:          120,
		SoftTPS:                    500,
		HardTPS:                    1000,
		ConsecutiveSoft:            2,
		ConsecutiveErrors:          2,
		MinHealthyProxies:          1,
		MinGenerationMs:            1000,
		MinOutputTokens:            32,
		Model:                      "grok-4.5",
		DisableAccountOnHard:       false,
		ThinkingGuard:              true,
		ConsecutiveMissingThinking: 1,
		ThinkingCrossVerify:        true,
		SoftCrossVerify:            true,
		MaxOutputTokensProbe:       384,
	}
}

func (p *ProxyPoolQualityPolicy) Normalize() {
	if p == nil {
		return
	}
	def := DefaultProxyPoolQualityPolicy()
	if p.Mode != ProxyPoolQualityModePassive && p.Mode != ProxyPoolQualityModeActive && p.Mode != ProxyPoolQualityModeHybrid {
		p.Mode = def.Mode
	}
	if p.ActiveIntervalSeconds <= 0 {
		p.ActiveIntervalSeconds = def.ActiveIntervalSeconds
	}
	if p.PassiveWindowSeconds <= 0 {
		p.PassiveWindowSeconds = def.PassiveWindowSeconds
	}
	if p.QuarantineSeconds <= 0 {
		p.QuarantineSeconds = def.QuarantineSeconds
	}
	if p.SoftTPS <= 0 {
		p.SoftTPS = def.SoftTPS
	}
	if p.HardTPS <= p.SoftTPS {
		p.HardTPS = def.HardTPS
		if p.HardTPS <= p.SoftTPS {
			p.HardTPS = p.SoftTPS * 2
		}
	}
	if p.ConsecutiveSoft <= 0 {
		p.ConsecutiveSoft = def.ConsecutiveSoft
	}
	if p.ConsecutiveErrors <= 0 {
		p.ConsecutiveErrors = def.ConsecutiveErrors
	}
	if p.MinHealthyProxies <= 0 {
		p.MinHealthyProxies = def.MinHealthyProxies
	}
	if p.MinGenerationMs <= 0 {
		p.MinGenerationMs = def.MinGenerationMs
	}
	if p.MinOutputTokens <= 0 {
		p.MinOutputTokens = def.MinOutputTokens
	}
	if strings.TrimSpace(p.Model) == "" {
		p.Model = def.Model
	}
	if p.ConsecutiveMissingThinking <= 0 {
		p.ConsecutiveMissingThinking = def.ConsecutiveMissingThinking
	}
	if p.MaxOutputTokensProbe <= 0 {
		p.MaxOutputTokensProbe = def.MaxOutputTokensProbe
	}
	// Keep admin-provided policies bounded so a malformed value cannot create
	// an unbounded probe cadence, quarantine, or request body.
	if p.ActiveIntervalSeconds > 7*24*60*60 {
		p.ActiveIntervalSeconds = 7 * 24 * 60 * 60
	}
	if p.PassiveWindowSeconds > 24*60*60 {
		p.PassiveWindowSeconds = 24 * 60 * 60
	}
	if p.QuarantineSeconds > 24*60*60 {
		p.QuarantineSeconds = 24 * 60 * 60
	}
	if p.SoftTPS > 1_000_000 {
		p.SoftTPS = 1_000_000
	}
	if p.HardTPS > 1_000_000 {
		p.HardTPS = 1_000_000
	}
	if p.MinOutputTokens > 1_000_000 {
		p.MinOutputTokens = 1_000_000
	}
	if p.MaxOutputTokensProbe > 4096 {
		p.MaxOutputTokensProbe = 4096
	}
	if p.HardTPS <= p.SoftTPS {
		if p.SoftTPS >= 1_000_000 {
			p.SoftTPS = 500_000
		}
		p.HardTPS = math.Min(1_000_000, p.SoftTPS*2)
	}
}

func (p ProxyPoolQualityPolicy) ActiveInterval() time.Duration {
	p.Normalize()
	return time.Duration(p.ActiveIntervalSeconds) * time.Second
}

func (p ProxyPoolQualityPolicy) QuarantineDuration() time.Duration {
	p.Normalize()
	return time.Duration(p.QuarantineSeconds) * time.Second
}

func (p ProxyPoolQualityPolicy) PassiveWindow() time.Duration {
	p.Normalize()
	return time.Duration(p.PassiveWindowSeconds) * time.Second
}

// ComputeProxyPoolTPS uses the same denominator as the CPA quality guard:
// output tokens divided by generation time after the first token. Very short
// windows fall back to total duration to avoid a single tiny response causing
// an exaggerated hard hit.
func ComputeProxyPoolTPS(outputTokens, durationMs, firstTokenMs, minGenerationMs int64) float64 {
	if outputTokens <= 0 || durationMs <= 0 {
		return 0
	}
	denom := durationMs - firstTokenMs
	if minGenerationMs <= 0 {
		minGenerationMs = DefaultProxyPoolQualityPolicy().MinGenerationMs
	}
	if denom < minGenerationMs {
		denom = durationMs
	}
	if denom < minGenerationMs {
		return 0
	}
	return float64(outputTokens) / (float64(denom) / 1000.0)
}

func ClassifyProxyPoolTPS(tps float64, outputTokens int64, policy ProxyPoolQualityPolicy) string {
	policy.Normalize()
	if outputTokens <= 0 || tps <= 0 {
		return ProxyPoolQualityUnknown
	}
	if outputTokens < policy.MinOutputTokens {
		return ProxyPoolQualityIgnored
	}
	if tps >= policy.HardTPS {
		return ProxyPoolQualityHard
	}
	if tps >= policy.SoftTPS {
		return ProxyPoolQualitySoft
	}
	return ProxyPoolQualityHealthy
}

// ClassifyProxyPoolQuality applies the CPA guard's thinking signal before the
// Token/s thresholds. A sufficiently long response without reasoning is a
// hard quality hit when the thinking guard is enabled; account and transport
// errors are classified by the caller and must not be passed here.
func ClassifyProxyPoolQuality(tps float64, outputTokens int64, hasThinking bool, policy ProxyPoolQualityPolicy) string {
	policy.Normalize()
	if outputTokens <= 0 || tps <= 0 {
		return ProxyPoolQualityUnknown
	}
	if outputTokens < policy.MinOutputTokens {
		return ProxyPoolQualityIgnored
	}
	if policy.ThinkingGuard && !hasThinking {
		return ProxyPoolQualityHard
	}
	return ClassifyProxyPoolTPS(tps, outputTokens, policy)
}

// ProxyPoolQualityErrorKind values are persisted only indirectly through the
// last-reason field, but keeping them centralized prevents status-code handling
// from drifting between active probes and passive observations.
const (
	ProxyPoolQualityErrorAccount   = "account_error"
	ProxyPoolQualityErrorTransport = "transport_error"
	ProxyPoolQualityErrorUpstream  = "upstream_error"
	ProxyPoolQualityErrorNoAccount = "no_account"
	ProxyPoolQualityErrorRequest   = "request_error"
)

func proxyPoolQualityErrorIsAccount(kind string) bool {
	return kind == ProxyPoolQualityErrorAccount || kind == ProxyPoolQualityErrorNoAccount
}

func proxyPoolQualityErrorIsIgnorable(kind string) bool {
	return proxyPoolQualityErrorIsAccount(kind) || kind == ProxyPoolQualityErrorUpstream || kind == ProxyPoolQualityErrorRequest
}

// classifyProxyPoolQualityFailure follows the CPA guard's important boundary:
// HTTP/auth/quota failures belong to the credential, while link failures belong
// to the egress. The returned kind is consumed by the state machine and is not
// exposed as a security-sensitive upstream error body.
func classifyProxyPoolQualityFailure(status int, body string) string {
	lower := strings.ToLower(body)
	if status == http.StatusProxyAuthRequired {
		return ProxyPoolQualityErrorTransport
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden ||
		status == http.StatusBadRequest || status == http.StatusNotFound ||
		status == http.StatusConflict || status == http.StatusUnprocessableEntity ||
		status == http.StatusTooManyRequests {
		return ProxyPoolQualityErrorAccount
	}
	for _, marker := range []string{
		"invalid token", "expired", "no auth", "quota", "rate limit", "ratelimit",
		"too many requests", "permission denied", "forbidden", "free-usage-exhausted",
	} {
		if strings.Contains(lower, marker) {
			return ProxyPoolQualityErrorAccount
		}
	}
	for _, marker := range []string{
		"connection refused", "connection reset", "dial tcp", "timeout", "timed out",
		"eof", "tls handshake", "no such host", "proxyconnect", "proxy authentication",
	} {
		if strings.Contains(lower, marker) {
			return ProxyPoolQualityErrorTransport
		}
	}
	if status >= 500 && status <= 599 {
		return ProxyPoolQualityErrorUpstream
	}
	return ProxyPoolQualityErrorRequest
}

func clampProxyPoolTPS(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 1_000_000 {
		return 1_000_000
	}
	return value
}
