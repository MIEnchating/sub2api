package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRequestProxyExplicitSingleKeepsExistingProxy(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"single\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))}}
	svc := openAIClientToolsTestService(upstream)
	account := excelAccount()
	account.Extra["openai_excel_bps"] = false
	account.Extra["openai_proxy_mode"] = "single"
	account.Extra["openai_proxy_ids"] = []int64{987654321}
	account.Extra["openai_excel_bps_proxy_mode"] = "request"
	account.Extra["openai_excel_bps_proxy_ids"] = []int64{987654321}
	proxyID := int64(77)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{Protocol: "http", Host: "single.example", Port: 8080}
	body := []byte(`{"model":"gpt-5.4","input":"test","stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	_, err := svc.Forward(t.Context(), c, account, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, account.Proxy.URL(), upstream.lastProxyURL)
}
