package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type ssoQualityUpstreamStub struct {
	response http.Response
	request  *http.Request
	proxyURL string
}

func (s *ssoQualityUpstreamStub) Do(req *http.Request, proxyURL string, _ int64, _ int) (*http.Response, error) {
	s.request = req
	s.proxyURL = proxyURL
	return &s.response, nil
}

func (s *ssoQualityUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, concurrency)
}

func TestGrokSSOQualityProberParsesCleanState(t *testing.T) {
	body := `{\"botFlagSource\":0,\"botFlagDetails\":\"policy=allow,risk=0.10,event=$login\"}`
	upstream := &ssoQualityUpstreamStub{response: http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body))}}
	prober := NewGrokSSOQualityProber(upstream)
	account := &Account{ID: 12, Platform: PlatformGrok, Type: AccountTypeOAuth, Concurrency: 2, Credentials: map[string]any{"sso": " cookie "}}

	result := prober.ProbeGrokSSOQuality(context.Background(), account, "socks5://user:pass@proxy:1080")
	require.Equal(t, GrokSSOQualityClean, result.State)
	require.NotNil(t, result.BotFlagSource)
	require.Equal(t, 0, *result.BotFlagSource)
	require.NotNil(t, result.Risk)
	require.InDelta(t, 0.10, *result.Risk, 0.0001)
	require.Equal(t, "allow", result.Policy)
	require.Equal(t, "$login", result.Event)
	require.NotNil(t, result.HTTPStatus)
	require.Equal(t, 200, *result.HTTPStatus)
	require.Equal(t, "sso=cookie; sso-rw=cookie", upstream.request.Header.Get("Cookie"))
	require.Equal(t, "socks5://user:pass@proxy:1080", upstream.proxyURL)
}

func TestGrokSSOQualityProberParsesIPAndAccountFlags(t *testing.T) {
	ip := GrokSSOQualityResult{}
	parseGrokSSOQualityBody(`{\"botFlagSource\":2,\"botFlagDetails\":\"eapi_ip_bot_farm free-tier\"}`, &ip)
	require.Equal(t, GrokSSOQualityFlaggedIP, ip.State)

	account := GrokSSOQualityResult{}
	parseGrokSSOQualityBody(`{\"botFlagSource\":1,\"botFlagDetails\":\"policy=deny,risk=0.95,event=$registration\"}`, &account)
	require.Equal(t, GrokSSOQualityFlaggedAccount, account.State)
	require.NotNil(t, account.Risk)
	require.InDelta(t, 0.95, *account.Risk, 0.0001)
}

func TestGrokSSOQualityProberRedactsProxyCredentials(t *testing.T) {
	message := `connect socks5://user:secret@proxy.example:1080 failed`
	got := redactProxyCredentials(message, "socks5://user:secret@proxy.example:1080")
	require.NotContains(t, got, "user:secret")
	require.NotContains(t, got, "proxy.example:1080")
}
