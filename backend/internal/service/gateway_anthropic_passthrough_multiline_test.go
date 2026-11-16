//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAnthropicPassthroughMultilineEventPreservesBillingAndPublicModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, composite := range []bool{false, true} {
		name := "direct"
		ctx := context.Background()
		if composite {
			name = "composite"
			ctx, _ = routeCompositeBody(t, publicModelRoute(), CompositeRouteEndpointMessages, []byte(`{"model":"public-model"}`))
		}
		t.Run(name, func(t *testing.T) {
			start := "event: message_start\nid: start-1\ndata: {\ndata: \"type\":\"message_start\",\ndata: \"message\":{\"model\":\"actual-model\",\"usage\":{\ndata: \"input_tokens\":13,\"cache_creation_input_tokens\":7,\"cache_read_input_tokens\":5,\ndata: \"cache_creation\":{\"ephemeral_5m_input_tokens\":3,\"ephemeral_1h_input_tokens\":4}}}}\n\n"
			tool := `event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"actual-model"}}` + "\n\n"
			delta := "event: message_delta\ndata: {\ndata: \"type\":\"message_delta\",\ndata: \"usage\":{\"output_tokens\":11}}\n\n"
			// A data-only, multiline terminal frame is valid SSE too.
			stop := "data: {\ndata: \"type\":\"message_stop\"}\n\n"
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(start + tool + delta + stop))}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
			observer := beginUpstreamResponseModelObservation(c)
			svc := &GatewayService{}
			result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, &Account{ID: 1}, time.Now(), "sent-model")
			require.NoError(t, err)
			require.Equal(t, 13, result.usage.InputTokens)
			require.Equal(t, 11, result.usage.OutputTokens)
			require.Equal(t, 7, result.usage.CacheCreationInputTokens)
			require.Equal(t, 5, result.usage.CacheReadInputTokens)
			require.Equal(t, 3, result.usage.CacheCreation5mTokens)
			require.Equal(t, 4, result.usage.CacheCreation1hTokens)
			require.Equal(t, "actual-model", observer.Model())
			require.NotNil(t, result.firstTokenMs)
			output := rec.Body.String()
			require.Contains(t, output, tool, "tool arguments must pass through unchanged")
			if !composite {
				require.Equal(t, start+tool+delta+stop, output)
				return
			}
			require.Contains(t, output, "id: start-1\n")
			var parts []string
			for _, line := range strings.Split(strings.SplitN(output, "\n\n", 2)[0], "\n") {
				if data, ok := extractAnthropicSSEDataLine(line); ok {
					parts = append(parts, data)
				}
			}
			payload := strings.Join(parts, "\n")
			require.True(t, gjson.Valid(payload))
			require.Equal(t, "public-model", gjson.Get(payload, "message.model").String())
			require.Equal(t, int64(13), gjson.Get(payload, "message.usage.input_tokens").Int())
		})
	}
}

func TestAnthropicPassthroughMultilineUsageSurvivesReadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, multiline := range []bool{false, true} {
		name := "single-data-field"
		payload := `data: {"type":"message_start","message":{"model":"actual-model","usage":{"input_tokens":13}}}` + "\n"
		if multiline {
			name = "multiple-data-fields"
			payload = "data: {\ndata: \"type\":\"message_start\",\ndata: \"message\":{\"model\":\"actual-model\",\"usage\":{\"input_tokens\":13}}}\n"
		}
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			observer := beginUpstreamResponseModelObservation(c)
			resp := &http.Response{Header: make(http.Header), Body: &streamReadCloser{payload: []byte(payload), err: io.ErrUnexpectedEOF}}
			svc := &GatewayService{}
			result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "sent-model")
			require.ErrorIs(t, err, io.ErrUnexpectedEOF)
			require.Equal(t, 13, result.usage.InputTokens)
			require.Equal(t, "actual-model", observer.Model())
			require.Empty(t, rec.Body.String(), "an unterminated event must not be forwarded after a read error")
		})
	}
}

func TestAnthropicPassthroughMultilineBillingErrorKeepsFailoverBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, started := range []bool{false, true} {
		name := "before-output"
		stream := ""
		if started {
			name = "after-output"
			stream = `event: message_start` + "\n" + `data: {"type":"message_start","message":{"usage":{"input_tokens":13}}}` + "\n\n"
		}
		t.Run(name, func(t *testing.T) {
			stream += "event: error\ndata: {\ndata: \"type\":\"error\",\ndata: \"error\":{\"code\":\"insufficient_balance\",\"message\":\"Upstream account balance exhausted\"}}\n\n"
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(stream))}
			svc := &GatewayService{}
			result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "sent-model")
			require.Error(t, err)
			var failover *UpstreamFailoverError
			if started {
				require.NotErrorAs(t, err, &failover)
				require.Equal(t, 13, result.usage.InputTokens)
				require.Contains(t, rec.Body.String(), "message_start")
			} else {
				require.ErrorAs(t, err, &failover)
				require.True(t, failover.IsUpstreamBillingExhausted())
				require.False(t, c.Writer.Written())
			}
			require.NotContains(t, rec.Body.String(), "insufficient_balance")
			require.NotContains(t, rec.Body.String(), "Upstream account balance exhausted")
		})
	}
}
