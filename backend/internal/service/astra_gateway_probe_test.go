package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAstraGatewayVerifyAlwaysUsesStateProbe(t *testing.T) {
	for _, kind := range []string{"", "state_probe", "candy"} {
		t.Run(kind, func(t *testing.T) {
			gateway, repo, seed := unifiedAstraFixture(t)
			target := mint780State(time.Now().Add(-time.Second))
			calls := 0
			gateway.httpUpstream = &harvestProxyUpstream{do: func(req *http.Request, proxy string) (*http.Response, error) {
				calls++
				require.True(t, IsOpenAICodexStateProbeRequest(req.Context()))
				require.Equal(t, seed.HarvestProxyURL, proxy)
				require.Equal(t, "Bearer target-token", req.Header.Get("Authorization"))
				require.Equal(t, "target-workspace", req.Header.Get("ChatGPT-Account-ID"))
				h := http.Header{}
				if calls == 1 {
					require.Equal(t, seed.State, req.Header.Get(openAICodexTurnStateHeader))
					h.Set(openAICodexTurnStateHeader, target)
				} else {
					require.Equal(t, target, req.Header.Get(openAICodexTurnStateHeader))
				}
				return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader(harvestProbeCompleteFixture("gpt-6-astra")))}, nil
			}}
			svc := &AccountTestService{cfg: gateway.cfg, accountRepo: repo, openaiGatewayService: gateway}
			result, err := svc.TestAstraGateway(t.Context(), "verify", 2, 1, kind)
			require.NoError(t, err)
			require.True(t, result.Success, result.Reason)
			require.Equal(t, "state_probe", result.TestKind)
			require.Empty(t, result.Answer)
			require.Empty(t, result.Expected)
			require.Equal(t, 2, calls, "verification must not send a separate quality question")
		})
	}

	gateway, repo, _ := unifiedAstraFixture(t)
	gateway.httpUpstream = &harvestProxyUpstream{do: func(req *http.Request, _ string) (*http.Response, error) {
		require.True(t, IsOpenAICodexStateProbeRequest(req.Context()))
		return &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("private upstream error"))}, nil
	}}
	svc := &AccountTestService{cfg: gateway.cfg, accountRepo: repo, openaiGatewayService: gateway}
	result, err := svc.TestAstraGateway(t.Context(), "verify", 2, 1)
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Equal(t, "target_probe_failed", result.Reason)
}
