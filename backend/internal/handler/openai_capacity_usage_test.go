package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCapacityRecoveryHTTPDoesNotReplayConsumedFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		mode := "native"
		if passthrough {
			mode = "passthrough"
		}
		for _, usage := range []struct {
			name     string
			body     string
			preamble bool
		}{
			{name: "input tokens", body: `{"input_tokens":2}`},
			{name: "cached tokens", body: `{"input_tokens_details":{"cached_tokens":2}}`},
			{name: "reasoning tokens", body: `{"output_tokens_details":{"reasoning_tokens":3}}`},
			{name: "image tokens", body: `{"output_tokens_details":{"image_tokens":4}}`},
			{name: "earlier reasoning tokens", body: `{"output_tokens_details":{"reasoning_tokens":3}}`, preamble: true},
		} {
			t.Run(mode+"/"+usage.name, func(t *testing.T) {
				var attempts atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					_, _ = io.Copy(io.Discard, r.Body)
					w.Header().Set("Content-Type", "text/event-stream")
					terminalUsage := usage.body
					if usage.preamble {
						_, _ = io.WriteString(w, "event: response.in_progress\ndata: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_used\",\"usage\":"+usage.body+"}}\n\n")
						terminalUsage = `{}`
					}
					_, _ = io.WriteString(w, "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_used\",\"status\":\"failed\",\"usage\":"+terminalUsage+",\"error\":{\"code\":\"server_error\",\"message\":\"The service is busy. Please retry later.\"}}}\n\n")
				}))
				defer upstream.Close()
				router, _ := newCapacityRecoveryHandler(t, upstream.URL, service.OpenAIWSIngressModeHTTPBridge, passthrough, 1)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","input":"hello","stream":true}`))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(rec, req)
				require.Equal(t, int32(1), attempts.Load(), "consumed failure must not be replayed")
				require.Equal(t, http.StatusOK, rec.Code)
				require.Contains(t, rec.Body.String(), "response.failed")
			})
		}
	}
}
