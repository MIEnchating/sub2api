package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

type quotaMaintenanceProxyRepo struct {
	ProxyRepository
	proxies map[int64]*Proxy
	err     error
}

func (r *quotaMaintenanceProxyRepo) GetByID(_ context.Context, id int64) (*Proxy, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.proxies[id], nil
}

func quotaMaintenanceProxy(t *testing.T, id int64, endpoint string) *Proxy {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	require.NoError(t, err)
	host, portText, err := net.SplitHostPort(parsed.Host)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	return &Proxy{ID: id, Protocol: parsed.Scheme, Host: host, Port: port, Status: StatusActive}
}

// Keep the actual transport behind local HTTP proxies. An unexpected direct or
// group-external URL fails before any transport can reach the network.
func quotaMaintenanceFactory(allowed ...string) PrivacyClientFactory {
	return func(proxyURL string) (*req.Client, error) {
		permitted := false
		for _, candidate := range allowed {
			permitted = permitted || proxyURL == candidate
		}
		if !permitted {
			return nil, fmt.Errorf("unexpected quota proxy")
		}
		client := req.C().SetProxyURL(proxyURL).SetTimeout(2 * time.Second)
		client.WrapRoundTripFunc(func(rt req.RoundTripper) req.RoundTripFunc {
			return func(r *req.Request) (*req.Response, error) {
				// Preserve the original hostname and path so the local server
				// receives an HTTP proxy request for the real quota endpoint.
				r.URL.Scheme = "http"
				return rt.RoundTrip(r)
			}
		})
		return client, nil
	}
}

func quotaMaintenanceResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/backend-api/wham/usage":
		_, _ = w.Write([]byte(`{"plan_type":"plus","rate_limit_reset_credits":{"available_count":0}}`))
	case "/backend-api/wham/rate-limit-reset-credits":
		_, _ = w.Write([]byte(`{"available_count":0,"credits":[]}`))
	case "/backend-api/wham/rate-limit-reset-credits/consume":
		_, _ = w.Write([]byte(`{"code":"ok","windows_reset":1}`))
	default:
		http.NotFound(w, r)
	}
}

func TestOpenAIQuotaMaintenanceSinglePreservesExistingProxyResolution(t *testing.T) {
	for _, eager := range []bool{true, false} {
		t.Run(fmt.Sprintf("eager=%t", eager), func(t *testing.T) {
			var hits atomic.Int32
			single := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				quotaMaintenanceResponse(w, r)
			}))
			defer single.Close()
			proxy := quotaMaintenanceProxy(t, 99, single.URL)
			account := &Account{
				ID: 100, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ProxyID: &proxy.ID,
				Credentials: map[string]any{"chatgpt_account_id": "quota-test-account"},
				Extra: map[string]any{"openai_proxy_mode": "single", "openai_proxy_ids": []int64{404},
					"openai_excel_bps": true, ExcelBPSProxyModeKey: ExcelBPSProxyModeRequest, ExcelBPSProxyIDsKey: []int64{404}},
			}
			var proxies ProxyRepository = &quotaMaintenanceProxyRepo{proxies: map[int64]*Proxy{proxy.ID: proxy}}
			if eager {
				account.Proxy = proxy
				proxies = nil
			}
			accounts := &stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			tokens := &stubQuotaTokenCache{tokens: map[string]string{OpenAITokenCacheKey(account): "fake-token"}}
			svc := NewOpenAIQuotaService(accounts, proxies, NewOpenAITokenProvider(accounts, tokens, nil), quotaMaintenanceFactory(single.URL), nil)
			_, err := svc.QueryUsage(context.Background(), account.ID)
			require.NoError(t, err)
			_, err = svc.ResetCredit(context.Background(), account.ID)
			require.NoError(t, err)
			require.EqualValues(t, 3, hits.Load())
		})
	}
}
