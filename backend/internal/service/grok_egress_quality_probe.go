package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

const (
	proxyPoolQualityProbeTimeout      = 90 * time.Second
	proxyPoolQualityProbeAccountLimit = 8
	proxyPoolQualityProbeBodyLimit    = 8 << 20
	proxyPoolQualityProbePrompt       = "Explain how TCP slow start works in at least twelve concise sentences. Include the congestion window, retransmission timeout, and slow-start exit conditions."
	proxyPoolQualityProbeUserAgent    = "sub2api-egress-quality/1.0"
)

// grokEgressQualityProber is the native Sub2API equivalent of the CPA guard's
// real-model probe. It deliberately sends the request through the selected
// proxy even when the credential is currently assigned to another pool exit.
type grokEgressQualityProber struct {
	accountRepo   AccountRepository
	qualityRepo   ProxyPoolQualityRepository
	tokenProvider *GrokTokenProvider
	httpUpstream  HTTPUpstream
	cfg           *config.Config
}

// NewGrokEgressQualityProber creates an active quality prober. The config is
// accepted through the concrete type below to keep this constructor easy to
// use in unit tests without exposing config details in the service contract.
func NewGrokEgressQualityProber(
	accountRepo AccountRepository,
	qualityRepo ProxyPoolQualityRepository,
	tokenProvider *GrokTokenProvider,
	httpUpstream HTTPUpstream,
	cfg *config.Config,
) ProxyPoolQualityProber {
	return &grokEgressQualityProber{
		accountRepo:   accountRepo,
		qualityRepo:   qualityRepo,
		tokenProvider: tokenProvider,
		httpUpstream:  httpUpstream,
		cfg:           cfg,
	}
}

func (p *grokEgressQualityProber) ProbeProxyQuality(ctx context.Context, pool *ProxyPool, proxy *ProxyPoolProxy, policy ProxyPoolQualityPolicy) ProxyPoolQualityObservation {
	policy.Normalize()
	base := ProxyPoolQualityObservation{Classification: ProxyPoolQualityUnknown, Source: "active"}
	if p == nil || p.accountRepo == nil || p.qualityRepo == nil || p.httpUpstream == nil {
		base.Classification = ProxyPoolQualityError
		base.ErrorKind = ProxyPoolQualityErrorRequest
		base.Reason = "grok quality probe is not configured"
		return base
	}
	if pool == nil || proxy == nil || proxy.ID <= 0 || strings.TrimSpace(proxy.URL()) == "" {
		base.Classification = ProxyPoolQualityError
		base.ErrorKind = ProxyPoolQualityErrorRequest
		base.Reason = "proxy pool member is invalid"
		return base
	}
	ids, err := p.qualityRepo.ListGrokProbeAccountIDs(ctx, pool.ID, proxy.ID, proxyPoolQualityProbeAccountLimit)
	if err != nil {
		base.Classification = ProxyPoolQualityError
		base.ErrorKind = ProxyPoolQualityErrorRequest
		base.Reason = truncateProxyPoolQualityMessage(err.Error())
		return base
	}
	if len(ids) == 0 {
		base.Classification = ProxyPoolQualityIgnored
		base.ErrorKind = ProxyPoolQualityErrorNoAccount
		base.Reason = "no schedulable Grok account is available for this pool"
		return base
	}

	var last ProxyPoolQualityObservation
	for index, accountID := range ids {
		if ctx.Err() != nil {
			base.Classification = ProxyPoolQualityError
			base.ErrorKind = ProxyPoolQualityErrorTransport
			base.Reason = ctx.Err().Error()
			return base
		}
		account, loadErr := p.accountRepo.GetByID(ctx, accountID)
		if loadErr != nil || account == nil {
			last = ProxyPoolQualityObservation{
				Classification: ProxyPoolQualityIgnored,
				Source:         "active",
				ErrorKind:      ProxyPoolQualityErrorAccount,
				AccountID:      accountID,
				Reason:         "probe account is unavailable",
			}
			continue
		}
		token, tokenErr := p.accessToken(ctx, account)
		if tokenErr != nil {
			last = ProxyPoolQualityObservation{
				Classification: ProxyPoolQualityIgnored,
				Source:         "active",
				ErrorKind:      ProxyPoolQualityErrorAccount,
				AccountID:      account.ID,
				Reason:         truncateProxyPoolQualityMessage(tokenErr.Error()),
			}
			continue
		}
		observation := p.probeWithAccount(ctx, account, proxy.URL(), token, policy)
		observation.AccountID = account.ID
		last = observation
		// Credential/quota failures are retried on another account. A proxy
		// transport failure is intentionally returned immediately: switching
		// credentials cannot repair a broken egress.
		if proxyPoolQualityErrorIsAccount(observation.ErrorKind) && index+1 < len(ids) {
			continue
		}
		return observation
	}
	if last.Classification == "" {
		return base
	}
	return last
}

func (p *grokEgressQualityProber) accessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil || !account.IsGrok() {
		return "", fmt.Errorf("not a Grok account")
	}
	if account.IsGrokOAuth() {
		if p.tokenProvider == nil {
			return "", fmt.Errorf("Grok token provider is unavailable")
		}
		return p.tokenProvider.GetAccessTokenForManualTest(ctx, account)
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return "", fmt.Errorf("Grok API key is empty")
	}
	return token, nil
}

func (p *grokEgressQualityProber) probeWithAccount(ctx context.Context, account *Account, proxyURL, token string, policy ProxyPoolQualityPolicy) ProxyPoolQualityObservation {
	result := ProxyPoolQualityObservation{Classification: ProxyPoolQualityError, Source: "active"}
	targetURL, err := buildGrokChatCompletionsURL(account, p.config())
	if err != nil {
		result.ErrorKind = ProxyPoolQualityErrorRequest
		result.Reason = truncateProxyPoolQualityMessage(err.Error())
		return result
	}
	maxTokens := policy.MaxOutputTokensProbe
	if maxTokens <= 0 {
		maxTokens = 384
	}
	body, err := json.Marshal(map[string]any{
		"model": policy.Model,
		"messages": []map[string]string{{
			"role":    "user",
			"content": proxyPoolQualityProbePrompt,
		}},
		"stream":      true,
		"max_tokens":  maxTokens,
		"temperature": 0.2,
	})
	if err != nil {
		result.ErrorKind = ProxyPoolQualityErrorRequest
		result.Reason = err.Error()
		return result
	}
	probeCtx, cancel := context.WithTimeout(ctx, proxyPoolQualityProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		result.ErrorKind = ProxyPoolQualityErrorRequest
		result.Reason = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("User-Agent", proxyPoolQualityProbeUserAgent)
	if account.IsGrokOAuth() {
		applyGrokCLIHeaders(req.Header)
	}
	account.ApplyHeaderOverrides(req.Header)

	started := time.Now()
	resp, err := p.httpUpstream.Do(req, proxyURL, account.ID, maxInt(account.Concurrency, 1))
	if err != nil {
		result.ErrorKind = ProxyPoolQualityErrorTransport
		result.Reason = truncateProxyPoolQualityMessage(err.Error())
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	if resp == nil {
		result.ErrorKind = ProxyPoolQualityErrorTransport
		result.Reason = "empty upstream response"
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
		result.DurationMs = time.Since(started).Milliseconds()
		result.HTTPStatus = resp.StatusCode
		result.Reason = fmt.Sprintf("Grok upstream HTTP %d: %s", resp.StatusCode, truncateProxyPoolQualityMessage(string(raw)))
		result.ErrorKind = classifyProxyPoolQualityFailure(resp.StatusCode, string(raw))
		if result.ErrorKind == ProxyPoolQualityErrorTransport {
			result.Classification = ProxyPoolQualityError
		} else {
			result.Classification = ProxyPoolQualityIgnored
		}
		return result
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		// Keep the body live while scanning so FirstTokenMs measures the first
		// generated chunk rather than the end of the complete response.
		return parseGrokQualitySSE(resp.Body, started, policy)
	}

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, proxyPoolQualityProbeBodyLimit))
	result.DurationMs = time.Since(started).Milliseconds()
	if readErr != nil {
		result.Classification = ProxyPoolQualityError
		result.ErrorKind = ProxyPoolQualityErrorTransport
		result.Reason = truncateProxyPoolQualityMessage(readErr.Error())
		return result
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") || bodyHasSSEFramingForQuality(raw) {
		// The helper consumes an in-memory copy. It is intentionally kept
		// separate from the gateway response writer so active probes never reach
		// clients.
		return parseGrokQualitySSE(bytes.NewReader(raw), started, policy)
	}
	result.OutputTokens = qualityOutputTokens(raw)
	result.HasThinking = openAIResponsePayloadHasThinking(raw)
	// A non-streaming response has no trustworthy TTFT signal because the
	// upstream buffers the complete body before returning it.
	result.FirstTokenMs = 0
	result.OutputTPS = clampProxyPoolTPS(ComputeProxyPoolTPS(result.OutputTokens, result.DurationMs, result.FirstTokenMs, policy.MinGenerationMs))
	result.Classification = ClassifyProxyPoolQuality(result.OutputTPS, result.OutputTokens, result.HasThinking, policy)
	if result.Classification == ProxyPoolQualityUnknown {
		result.ErrorKind = ProxyPoolQualityErrorUpstream
		result.Reason = "Grok probe returned no usable output"
	}
	return result
}

func bodyHasSSEFramingForQuality(body []byte) bool {
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if bytes.HasPrefix(line, []byte("data:")) || bytes.HasPrefix(line, []byte("event:")) {
			return true
		}
	}
	return false
}

func (p *grokEgressQualityProber) config() *config.Config {
	if p == nil {
		return nil
	}
	return p.cfg
}

func parseGrokQualitySSE(body io.Reader, started time.Time, policy ProxyPoolQualityPolicy) ProxyPoolQualityObservation {
	return parseGrokQualitySSEWithClock(body, started, policy, time.Now)
}

func parseGrokQualitySSEWithClock(body io.Reader, started time.Time, policy ProxyPoolQualityPolicy, now func() time.Time) ProxyPoolQualityObservation {
	result := ProxyPoolQualityObservation{Classification: ProxyPoolQualityUnknown, Source: "active"}
	var contentLen int64
	var usageTokens int64
	firstTokenSeen := false
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}
		data := []byte(payload)
		if !gjson.ValidBytes(data) {
			continue
		}
		if tokens := qualityOutputTokens(data); tokens > usageTokens {
			usageTokens = tokens
		}
		if openAIResponsePayloadHasThinking(data) {
			result.HasThinking = true
		}
		chunkLen := qualityContentLength(data)
		contentLen += chunkLen
		if !firstTokenSeen && chunkLen > 0 {
			firstTokenSeen = true
			result.FirstTokenMs = nonNegativeElapsedMilliseconds(started, now())
		}
	}
	result.DurationMs = nonNegativeElapsedMilliseconds(started, now())
	result.OutputTokens = usageTokens
	if result.OutputTokens <= 0 && contentLen > 0 {
		result.OutputTokens = contentLen / 4
		if result.OutputTokens == 0 {
			result.OutputTokens = 1
		}
	}
	result.OutputTPS = clampProxyPoolTPS(ComputeProxyPoolTPS(result.OutputTokens, result.DurationMs, result.FirstTokenMs, policy.MinGenerationMs))
	if scanErr := scanner.Err(); scanErr != nil {
		result.Classification = ProxyPoolQualityError
		result.ErrorKind = ProxyPoolQualityErrorTransport
		result.Reason = truncateProxyPoolQualityMessage(scanErr.Error())
		return result
	}
	result.Classification = ClassifyProxyPoolQuality(result.OutputTPS, result.OutputTokens, result.HasThinking, policy)
	if result.Classification == ProxyPoolQualityUnknown {
		result.ErrorKind = ProxyPoolQualityErrorUpstream
		result.Reason = "Grok probe returned no usable output"
	}
	return result
}

func nonNegativeElapsedMilliseconds(started, finished time.Time) int64 {
	elapsed := finished.Sub(started).Milliseconds()
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func qualityOutputTokens(data []byte) int64 {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return 0
	}
	var best int64
	for _, path := range []string{
		"usage", "response.usage", "usage.output_tokens_details",
		"response.usage.output_tokens_details", "response.output_tokens_details", "output_tokens_details",
	} {
		value := gjson.GetBytes(data, path)
		if !value.Exists() || !value.IsObject() {
			continue
		}
		for _, key := range []string{"output_tokens", "completion_tokens", "total_output_tokens", "reasoning_tokens"} {
			if n := value.Get(key).Int(); n > best {
				best = n
			}
		}
	}
	return best
}

func openAIResponsePayloadHasThinking(data []byte) bool {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return false
	}
	for _, path := range []string{
		"usage.output_tokens_details.reasoning_tokens",
		"usage.output_tokens_details.reasoningTokens",
		"response.usage.output_tokens_details.reasoning_tokens",
		"response.usage.output_tokens_details.reasoningTokens",
		"usage.reasoning_tokens",
		"response.usage.reasoning_tokens",
	} {
		if gjson.GetBytes(data, path).Int() > 0 {
			return true
		}
	}
	for _, path := range []string{
		"thinking_content", "reasoning_content", "thinking", "reasoning_summary_text",
		"reasoning_summary", "delta.thinking_content", "delta.reasoning_content", "delta.thinking",
		"delta.reasoning_summary_text", "delta.reasoning_summary",
		"message.reasoning_content", "choices.0.delta.reasoning_content",
		"choices.0.delta.thinking_content", "choices.0.delta.thinking",
		"choices.0.message.reasoning_content", "choices.0.message.thinking_content",
	} {
		if value := gjson.GetBytes(data, path); value.Exists() && strings.TrimSpace(value.String()) != "" {
			return true
		}
	}
	for _, path := range []string{"response.output", "output", "item"} {
		value := gjson.GetBytes(data, path)
		if value.IsObject() && strings.EqualFold(strings.TrimSpace(value.Get("type").String()), "reasoning") {
			return true
		}
		for _, item := range value.Array() {
			if strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "reasoning") {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(gjson.GetBytes(data, "type").String()), "reasoning")
}

func qualityPayloadHasOutput(data []byte) bool {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return false
	}
	if qualityContentLength(data) > 0 {
		return true
	}
	for _, path := range []string{
		"choices.0.delta.content", "choices.0.message.content", "output_text.delta",
		"response.output_text.delta", "content", "response.output",
	} {
		value := gjson.GetBytes(data, path)
		if value.Exists() && strings.TrimSpace(value.String()) != "" {
			return true
		}
	}
	return false
}

func qualityContentLength(data []byte) int64 {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return 0
	}
	var total int64
	for _, path := range []string{
		"choices.0.delta.content", "choices.0.message.content", "choices.0.delta.thinking_content",
		"choices.0.delta.reasoning_content", "delta.content", "delta.thinking_content",
		"delta.reasoning_content", "content", "output_text.delta", "response.output_text.delta",
	} {
		if value := gjson.GetBytes(data, path); value.Exists() && value.Type == gjson.String {
			total += int64(len([]rune(value.String())))
		}
	}
	// Responses API streams encode output_text/reasoning deltas as a bare
	// `delta` string alongside an event type such as response.output_text.delta.
	eventType := strings.ToLower(gjson.GetBytes(data, "type").String())
	if strings.HasSuffix(eventType, ".delta") || strings.Contains(eventType, "reasoning") {
		if value := gjson.GetBytes(data, "delta"); value.Exists() && value.Type == gjson.String {
			total += int64(len([]rune(value.String())))
		}
	}
	return total
}
