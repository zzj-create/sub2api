package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	GrokSSOQualityClean                = "clean"
	GrokSSOQualityFlaggedAccount       = "flagged_account"
	GrokSSOQualityFlaggedIP            = "flagged_ip"
	GrokSSOQualityError                = "error"
	GrokSSOQualityUnknown              = "unknown"
	grokSSOQualityHomeURL              = "https://grok.com/"
	grokSSOQualityUserAgent            = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"
	grokSSOQualityBodyLimit      int64 = 8 << 20
)

var (
	grokSSOBotFlagSourceRE  = regexp.MustCompile(`botFlagSource"\s*:\s*(null|-?\d+)`)
	grokSSOBotFlagDetailsRE = regexp.MustCompile(`botFlagDetails"\s*:\s*(null|"([^"]*)")`)
	grokSSOIPFarmMarkers    = []string{"eapi_ip_bot_farm", "no_token_farm"}
)

type grokSSOQualityProber struct {
	httpUpstream HTTPUpstream
}

// NewGrokSSOQualityProber reads the same Grok web risk fields as the external
// SSO checker, but keeps the request inside Sub2API and routes it through the
// account's assigned pool proxy.
func NewGrokSSOQualityProber(httpUpstream HTTPUpstream) ProxyPoolSSOQualityProber {
	return &grokSSOQualityProber{httpUpstream: httpUpstream}
}

func (p *grokSSOQualityProber) ProbeGrokSSOQuality(ctx context.Context, account *Account, proxyURL string) GrokSSOQualityResult {
	checkedAt := time.Now().UTC()
	result := GrokSSOQualityResult{
		State:     GrokSSOQualityUnknown,
		CheckedAt: checkedAt,
	}
	if p == nil || p.httpUpstream == nil {
		result.State = GrokSSOQualityError
		result.Reason = "SSO quality prober is unavailable"
		return result
	}
	if account == nil || !account.IsGrokOAuth() {
		result.State = GrokSSOQualityError
		result.Reason = "account is not Grok OAuth"
		return result
	}
	sso := strings.TrimSpace(account.GetCredential("sso"))
	if sso == "" {
		result.State = GrokSSOQualityError
		result.Reason = "account has no stored SSO cookie"
		return result
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, grokSSOQualityHomeURL, nil)
	if err != nil {
		result.State = GrokSSOQualityError
		result.Reason = "invalid Grok SSO quality request"
		return result
	}
	req.Header.Set("User-Agent", grokSSOQualityUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Cookie", "sso="+sso+"; sso-rw="+sso)
	concurrency := account.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	resp, err := p.httpUpstream.Do(req, proxyURL, account.ID, concurrency)
	if err != nil {
		result.State = GrokSSOQualityError
		result.Reason = truncateProxyPoolQualityMessage(redactProxyCredentials(err.Error(), proxyURL))
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	httpStatus := resp.StatusCode
	result.HTTPStatus = &httpStatus
	if resp.StatusCode != http.StatusOK {
		result.State = GrokSSOQualityError
		result.Reason = fmt.Sprintf("grok.com HTTP %d", resp.StatusCode)
		return result
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, grokSSOQualityBodyLimit+1))
	if err != nil {
		result.State = GrokSSOQualityError
		result.Reason = truncateProxyPoolQualityMessage(redactProxyCredentials(err.Error(), proxyURL))
		return result
	}
	if int64(len(body)) > grokSSOQualityBodyLimit {
		result.State = GrokSSOQualityError
		result.Reason = "grok.com response body is too large"
		return result
	}
	parseGrokSSOQualityBody(string(body), &result)
	return result
}

func parseGrokSSOQualityBody(body string, result *GrokSSOQualityResult) {
	if result == nil {
		return
	}
	normalized := strings.ReplaceAll(body, `\"`, `"`)
	sourceMatch := grokSSOBotFlagSourceRE.FindStringSubmatch(normalized)
	detailsMatch := grokSSOBotFlagDetailsRE.FindStringSubmatch(normalized)
	if len(sourceMatch) < 2 && len(detailsMatch) < 3 {
		result.State = GrokSSOQualityUnknown
		result.Reason = "botFlag fields were not found"
		return
	}

	var source *int
	if len(sourceMatch) == 2 && sourceMatch[1] != "null" {
		value, err := strconv.Atoi(sourceMatch[1])
		if err == nil {
			source = &value
		}
	}
	result.BotFlagSource = source
	details := ""
	if len(detailsMatch) == 3 && detailsMatch[2] != "" {
		details = detailsMatch[2]
	}
	fields := parseGrokSSODetailFields(details)
	result.Reason = truncateProxyPoolQualityMessage(details)
	if riskText := fields["risk"]; riskText != "" {
		if risk, err := strconv.ParseFloat(riskText, 64); err == nil {
			result.Risk = &risk
		}
	}
	result.Policy = strings.TrimSpace(fields["policy"])
	result.Event = strings.TrimSpace(fields["event"])

	flagged := (source != nil && (*source == 1 || *source == 2)) || strings.EqualFold(result.Policy, "deny")
	if flagged {
		lower := strings.ToLower(details)
		for _, marker := range grokSSOIPFarmMarkers {
			if strings.Contains(lower, marker) {
				result.State = GrokSSOQualityFlaggedIP
				if result.Reason == "" {
					result.Reason = marker
				}
				return
			}
		}
		result.State = GrokSSOQualityFlaggedAccount
		if result.Reason == "" {
			result.Reason = "policy=deny"
		}
		return
	}
	result.State = GrokSSOQualityClean
	if result.Reason == "" && source != nil {
		result.Reason = fmt.Sprintf("botFlagSource=%d", *source)
	}
}

func parseGrokSSODetailFields(details string) map[string]string {
	result := make(map[string]string)
	for _, item := range strings.Split(details, ",") {
		key, value, found := strings.Cut(item, "=")
		if !found || strings.TrimSpace(key) == "" {
			continue
		}
		result[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return result
}

func redactProxyCredentials(message string, proxyURL string) string {
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return message
	}
	message = strings.ReplaceAll(message, trimmed, "<proxy>")
	if index := strings.LastIndex(message, "@"); index >= 0 && strings.Contains(message[:index], "://") {
		if schemeStart := strings.Index(message, "://"); schemeStart >= 0 && index > schemeStart {
			message = message[:schemeStart+3] + "<redacted>" + message[index:]
		}
	}
	return message
}
