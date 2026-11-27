//go:build bps_request_integration

package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mihomo"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const bpsRequestCompletedWire = "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_local\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"

type requestProxyObservation struct {
	id   int
	path string
	body []byte
}

// This adapter replaces the external HTTPS boundary with controlled local HTTP
// proxies. Selection, leases, request construction and bridge consumption stay
// on the public Forward path; no external host is resolved or contacted.
func localBPSRequestProxyFixture(t *testing.T, count int, handler func(int, http.ResponseWriter, *http.Request)) (*OpenAIGatewayService, *Account) {
	t.Helper()
	restore := mihomo.InstallBPSRequestIntegrationProbe(func(context.Context, string) error { return nil })
	t.Cleanup(restore)
	clients := make(map[string]*http.Client, count)
	proxies := make(map[int64]*Proxy, count)
	records := make([]mihomo.BPSStaticProxy, 0, count)
	ids := make([]int64, 0, count)
	for i := 1; i <= count; i++ {
		id := i
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler(id, w, r) }))
		t.Cleanup(proxy.Close)
		u, err := url.Parse(proxy.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(u.Port())
		require.NoError(t, err)
		proxies[int64(i)] = &Proxy{ID: int64(i), Protocol: "http", Host: u.Hostname(), Port: port, Status: StatusActive}
		tr := &http.Transport{Proxy: http.ProxyURL(u)}
		t.Cleanup(tr.CloseIdleConnections)
		clients[proxy.URL] = &http.Client{Transport: tr, Timeout: 10 * time.Second}
		ids = append(ids, int64(i))
		records = append(records, mihomo.BPSStaticProxy{ID: int64(i), URL: proxy.URL})
	}
	mihomo.SetBPSStaticProxyRecords(records)
	mihomo.SetBPSStaticRequestGroups(map[int64][]int64{300: ids})
	mihomo.WarmBPSPools(t.Context(), 0, 1)
	svc := openAIClientToolsTestService(nil)
	svc.proxyRepo = &openAIMaintenanceProxyRepo{proxies: proxies}
	svc.httpUpstream = &nativeAttachmentUpstream{do: func(r *http.Request, proxy string, _ int64, _ int) (*http.Response, error) {
		client := clients[proxy]
		if client == nil {
			return nil, fmt.Errorf("unexpected proxy outside selected fixture")
		}
		local := r.Clone(r.Context())
		u := *r.URL
		u.Scheme = "http"
		local.URL = &u
		local.RequestURI = ""
		return client.Do(local)
	}}
	account := excelAccount()
	account.Concurrency = 6
	account.Extra["openai_excel_bps_proxy_mode"] = "request"
	account.Extra["openai_excel_bps_proxy_ids"] = ids
	account.Extra["openai_excel_bps_mihomo"] = false
	return svc, account
}

func forwardLocalBPSRequest(ctx context.Context, svc *OpenAIGatewayService, account *Account, body []byte) (int, string, error) {
	// Scheduler hydration gives each request its own Account model caches. Keep
	// that contract here while sharing the account ID for actual pool load.
	requestAccount := *account
	requestAccount.Extra = cloneExcelBPSRecoveryExtra(account.Extra)
	requestAccount.Credentials = cloneExcelBPSRecoveryExtra(account.Credentials)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	_, err := svc.Forward(ctx, c, &requestAccount, body)
	return rec.Code, rec.Body.String(), err
}
