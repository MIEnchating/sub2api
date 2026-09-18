package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIWSHTTPBridgeSuppressedReasoningUsagePreventsCapacityReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stream := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_consumed"}}`,
		`data: {"type":"error","error":{"code":"server_error","message":"temporary processing issue"}}`,
		`data: {"type":"response.in_progress","response":{"id":"resp_consumed","usage":{"output_tokens_details":{"reasoning_tokens":3}}}}`,
		`data: {"type":"response.failed","response":{"id":"resp_consumed","status":"failed","error":{"code":"server_error","message":"The service is busy. Please retry later."}}}`,
		"",
	}, "\n\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 5901, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
	payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hello"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	var events []string

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-token", payload, len(payload),
		"gpt-5", "", "", "", "", 2, func(message []byte) error {
			events = append(events, gjson.GetBytes(message, "type").String())
			return nil
		})

	var failoverErr *UpstreamFailoverError
	require.NotErrorAs(t, err, &failoverErr, "previously consumed reasoning must prevent replay even when its event was suppressed")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "response.failed", result.UpstreamTerminalEvent)
	require.Equal(t, []string{"response.created", "response.failed"}, events)
}
