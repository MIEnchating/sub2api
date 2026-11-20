package service

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCodex780ProbeDoesNotReuseCachedRoutesAcrossCandidates(t *testing.T) {
	account := ticketTestAccount(1)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICodexTicket: config.OpenAICodexTicketConfig{TargetLength: 780}}}}
	pair := mint780Pair(time.Now().Add(time.Hour), "unified-123")
	state := mint780State(time.Now())
	calls := 0
	svc.httpUpstream = &harvestProxyUpstream{do: func(req *http.Request, _ string) (*http.Response, error) {
		calls++
		header := http.Header{}
		if calls%2 == 1 {
			require.Empty(t, req.Header.Get("Cookie"), "every candidate must self-mint")
			require.Empty(t, req.Header.Get(openAICodexTurnStateHeader))
			header["Set-Cookie"] = pair
			header.Set(openAICodexTurnStateHeader, state)
		} else {
			require.Equal(t, strings.Join(pair, "; "), req.Header.Get("Cookie"))
			require.Equal(t, state, req.Header.Get(openAICodexTurnStateHeader))
		}
		// The short state probe deliberately does not infer quality from this label.
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(harvestProbeCompleteFixture("gpt-6-luna")))}, nil
	}}
	for _, model := range []string{"gpt-6-astra", "gpt-6-sol", "gpt-6-astra"} {
		out := svc.executeCodexHarvestProbe(context.Background(), account, "token", model, "", time.Second, func() bool { return true }, "session")
		require.Equal(t, "success", out.Kind)
		require.True(t, out.ProbeVerified)
		require.Nil(t, svc.lookupOpenAICodexTicket(account, model), "probing alone must not store a ticket")
	}
	require.Equal(t, 6, calls)
}

func TestCodex780RouteCookieDeletionAndAliases(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	pair := mint780Pair(now.Add(time.Hour), "unified-88")
	for _, suffix := range []string{"; Max-Age=0", "; Max-Age=-1", "; Expires=" + now.Add(-time.Hour).Format(http.TimeFormat)} {
		resp := &http.Response{Header: http.Header{"Set-Cookie": {pair[0] + suffix, pair[1]}}}
		_, err := codex780ResponseRoute(resp, pair, "unified-88", now)
		require.EqualError(t, err, "route_cookie_deleted")
	}
	for _, input := range []string{"88", "unified88", "unified_88", "unified.88", " UNIFIED-88 ", "chat.gateway.unified-88.api.openai.com"} {
		v := CodexHarvestControls{Version: 1, TargetGateway: input, Speed: CodexHarvestSpeedPresets()["standard"]}
		normalizeCodexHarvestControls(&v)
		require.Equal(t, "unified-88", v.TargetGateway)
		require.NoError(t, ValidateCodexHarvestControls(v))
	}
}

func TestCodex780RouteCacheIsolationAndExpiry(t *testing.T) {
	now := time.Unix(1800000000, 0)
	account := &Account{ID: 1}
	key := codex780RouteKey(account, "token", "header", "unified-95", "sse")
	for _, other := range [][32]byte{
		codex780RouteKey(&Account{ID: 2}, "token", "header", "unified-95", "sse"),
		codex780RouteKey(&Account{ID: 1, Credentials: map[string]any{"chatgpt_account_id": "changed-identity"}}, "token", "header", "unified-95", "sse"),
		codex780RouteKey(account, "rotated-token", "header", "unified-95", "sse"),
		codex780RouteKey(account, "token", "other-header", "unified-95", "sse"),
		codex780RouteKey(account, "token", "header", "unified-88", "sse"),
		codex780RouteKey(account, "token", "header", "unified-95", "websocket"),
	} {
		if key == other {
			t.Fatal("route cache key must isolate account, credential and route identity")
		}
	}
	service := &OpenAIGatewayService{}
	cache := &service.codex780Routes
	pair := mint780Pair(now.Add(time.Hour), "unified-95")
	cache.update(key, "unified-95", nil, codexHarvestProbeResult{Sent: true, Status: http.StatusOK, Cookies: pair}, now)
	got, ok := cache.get(key, now)
	if !ok || !slices.Equal(pair, got) {
		t.Fatal("fresh route was not retained")
	}
	got[0] = "mutated"
	got, _ = cache.get(key, now)
	if !slices.Equal(pair, got) {
		t.Fatal("reader mutated cached cookies")
	}
	if _, ok := cache.get(key, now.Add(time.Hour)); ok {
		t.Fatal("expired route must not be reused")
	}
}

func TestCodex780RouteCacheLateFailureCannotEvictNewRoute(t *testing.T) {
	now := time.Unix(1800000000, 0)
	key := codex780RouteKey(&Account{ID: 1}, "token", "header", "unified-95", "sse")
	oldPair := mint780Pair(now.Add(time.Hour), "unified-95")
	newPair := mint780Pair(now.Add(2*time.Hour), "unified-95")
	cache := &codex780RouteCache{}
	cache.update(key, "unified-95", nil, codexHarvestProbeResult{Sent: true, Status: http.StatusOK, Cookies: oldPair}, now)
	cache.update(key, "unified-95", oldPair, codexHarvestProbeResult{Sent: true, Status: http.StatusOK, Cookies: newPair}, now)
	cache.update(key, "unified-95", oldPair, codexHarvestProbeResult{Sent: true, Status: http.StatusUnauthorized}, now)
	got, ok := cache.get(key, now)
	if !ok || !slices.Equal(newPair, got) {
		t.Fatal("late failure evicted a newer route from another lane")
	}
	cache.update(key, "unified-95", newPair, codexHarvestProbeResult{Sent: true, Status: http.StatusUnauthorized}, now)
	got, ok = cache.get(key, now)
	if !ok || len(got) != 0 {
		t.Fatal("rejected route must retain a tombstone until expiry")
	}
}
