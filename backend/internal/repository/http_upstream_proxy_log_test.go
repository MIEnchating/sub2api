package repository

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type proxyLogRoundTripper struct{}

func (proxyLogRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
}

func TestHTTPUpstreamProxyLogsDoNotExposeCredentials(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	cfg := &config.Config{Gateway: config.GatewayConfig{
		ConnectionPoolIsolation: config.ConnectionPoolIsolationAccountProxy,
		OpenAIHTTP2: config.GatewayOpenAIHTTP2Config{
			Enabled: true, AllowProxyFallbackToHTTP1: true,
			FallbackErrorThreshold: 1, FallbackWindowSeconds: 60, FallbackTTLSeconds: 600,
		},
	}}
	svc, ok := NewHTTPUpstream(cfg).(*httpUpstreamService)
	require.True(t, ok)
	proxy := "http://private-user:private-password@proxy.example:8080/private-path?token=private-token#private-fragment"
	profile := &tlsfingerprint.Profile{Name: "proxy-log-test"}
	entry, err := svc.getClientEntryWithTLS(proxy, 42, 2, profile, service.HTTPUpstreamProfileDefault, false, false)
	require.NoError(t, err)
	require.Contains(t, entry.proxyKey, "private-user:private-password", "redaction must not alter the transport credentials")
	reused, err := svc.getClientEntryWithTLS(proxy, 42, 2, profile, service.HTTPUpstreamProfileDefault, false, false)
	require.NoError(t, err)
	require.Same(t, entry, reused)
	entry.poolKey = "stale"
	replacement, err := svc.getClientEntryWithTLS(proxy, 42, 2, profile, service.HTTPUpstreamProfileDefault, false, false)
	require.NoError(t, err)
	require.NotSame(t, entry, replacement)
	replacement.client.CloseIdleConnections()
	replacement.client.Transport = proxyLogRoundTripper{}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://upstream.example/", nil)
	require.NoError(t, err)
	resp, err := svc.DoWithTLS(req, proxy, 42, 2, profile)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	svc.recordOpenAIHTTP2Failure(service.HTTPUpstreamProfileOpenAI, upstreamProtocolModeOpenAIH2, proxy, errors.New("http2: protocol error"))
	require.True(t, svc.isOpenAIHTTP2FallbackActive(proxy))
	logs := output.String()
	for _, event := range []string{"tls_fingerprint_enabled", "tls_fingerprint_creating_new_client", "tls_fingerprint_reusing_client", "tls_fingerprint_evicting_stale_client", "openai_http2_proxy_fallback_activated"} {
		require.Contains(t, logs, event)
	}
	require.Contains(t, logs, "http://proxy.example:8080")
	require.NotContains(t, logs, "private-")
	require.NotContains(t, logs, "cache_key")
}
