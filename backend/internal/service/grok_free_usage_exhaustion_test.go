//go:build unit

package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestParseGrokQuotaSnapshotWithBodyBuilds24HourFreeLimit(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"error":{"code":"subscription:free-usage-exhausted","message":"You've used all the included free usage. Usage resets over a rolling 24-hour window."}}`)

	snapshot := parseGrokQuotaSnapshotWithBody(nil, http.StatusTooManyRequests, body, now)

	require.NotNil(t, snapshot)
	require.Equal(t, "free", snapshot.SubscriptionTier)
	require.Equal(t, "upstream_error_body", snapshot.ObservationSource)
	require.NotNil(t, snapshot.Tokens)
	require.Equal(t, xai.GrokFreeRolling24hTokenLimit, *snapshot.Tokens.Limit)
	require.Zero(t, *snapshot.Tokens.Remaining)
	require.Equal(t, now.Add(24*time.Hour).Unix(), *snapshot.Tokens.ResetUnix)
}

func TestParseGrokQuotaSnapshotWithBodyHandlesRewrittenStatus(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"error":{"code":"subscription:free-usage-exhausted","message":"free usage exhausted"}}`)

	snapshot := parseGrokQuotaSnapshotWithBody(nil, http.StatusBadRequest, body, now)

	require.NotNil(t, snapshot)
	resetAt, limited := grokRateLimitResetAt(snapshot, now)
	require.True(t, limited)
	require.Equal(t, now.Add(24*time.Hour).Unix(), resetAt.Unix())
}
