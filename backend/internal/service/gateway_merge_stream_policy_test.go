//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicCompatBillingErrorRetainsPolicyAndUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, forward := range map[string]func(*GatewayService, context.Context, *gin.Context, *Account, []byte, *ParsedRequest) (*ForwardResult, error){
		"chat":      (*GatewayService).ForwardAsChatCompletions,
		"responses": (*GatewayService).ForwardAsResponses,
	} {
		for _, started := range []bool{false, true} {
			t.Run(name+"/started="+strconv.FormatBool(started), func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+name, nil)
				stream := ""
				if started {
					stream = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-6\",\"content\":[],\"usage\":{\"input_tokens\":11}}}\n\n"
				}
				stream += "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"insufficient_balance\",\"message\":\"Provider account balance exhausted\"}}\n\n"
				upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}}}
				repo := &openAIStream403AccountRepo{}
				svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream, rateLimitService: &RateLimitService{accountRepo: repo}}
				result, err := forward(svc, context.Background(), c, newAnthropicAPIKeyAccountForTest(), []byte(`{"model":"claude-sonnet-4-6","stream":true,"messages":[{"role":"user","content":"hi"}],"input":"hi"}`), nil)
				require.Error(t, err)
				require.Equal(t, 1, upstream.callCount)
				require.Equal(t, 1, repo.setErrorCalls)
				var failover *UpstreamFailoverError
				if started {
					require.NotErrorAs(t, err, &failover)
					require.NotNil(t, result)
					require.Equal(t, 11, result.Usage.InputTokens)
					require.Contains(t, rec.Body.String(), UpstreamBillingExhaustedClientMessage)
				} else {
					require.ErrorAs(t, err, &failover)
					require.True(t, failover.IsUpstreamBillingExhausted())
					require.False(t, failover.RetryableOnSameAccount)
					require.Nil(t, result)
					require.False(t, c.Writer.Written())
					require.Empty(t, c.Writer.Header().Get("Content-Type"))
				}
				require.NotContains(t, rec.Body.String(), "insufficient_balance")
				require.NotContains(t, rec.Body.String(), "Provider account balance exhausted")
			})
		}
	}
}

func TestAnthropicStreamPumpsHonorRuntimePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, configured := range []int{0, 90} {
		for _, live := range []int{0, 45} {
			t.Run(strconv.Itoa(configured)+"/"+strconv.Itoa(live), func(t *testing.T) {
				cfg := &config.Config{Gateway: config.GatewayConfig{StreamDataIntervalTimeout: configured}}
				settings := NewSettingService(&gatewayPolicySettingRepo{values: map[string]string{SettingKeyGatewayStreamDataIntervalTimeoutSeconds: strconv.Itoa(live)}}, cfg)
				require.NoError(t, settings.WarmGatewayRuntimePolicy(context.Background()))
				openai := &OpenAIGatewayService{cfg: cfg, settingService: settings}
				require.Equal(t, time.Duration(live)*time.Second, openai.anthropicNativeStreamInterval())
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
				svc := &GatewayService{cfg: cfg, settingService: settings}
				stream := svc.newAnthropicCompatStream(c, &http.Response{Body: io.NopCloser(strings.NewReader(""))}, "test", "test")
				defer stream.stop()
				require.Equal(t, time.Duration(live)*time.Second, stream.pump.interval)
				require.Equal(t, live > 0, stream.pump.timer != nil)
				stream.markClientDisconnected()
				require.NotNil(t, stream.pump.timer, "disabled live timeout must still bound disconnect draining")
			})
		}
	}
}
