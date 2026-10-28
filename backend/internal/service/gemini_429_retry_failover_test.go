package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiAlways429Upstream struct {
	calls int
}

func (s *geminiAlways429Upstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	return &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"0"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
		Request:    req,
	}, nil
}

func (s *geminiAlways429Upstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestGeminiForwardPreservesExhaustedAccount429MarkerOnFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forwarders := []struct {
		name    string
		forward func(*GeminiMessagesCompatService, context.Context, *gin.Context, *Account) (*ForwardResult, error)
	}{
		{"claude", func(s *GeminiMessagesCompatService, ctx context.Context, c *gin.Context, account *Account) (*ForwardResult, error) {
			return s.Forward(ctx, c, account, []byte(`{"model":"gemini-2.5-flash","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`))
		}},
		{"native", func(s *GeminiMessagesCompatService, ctx context.Context, c *gin.Context, account *Account) (*ForwardResult, error) {
			return s.ForwardNative(ctx, c, account, "gemini-2.5-flash", "generateContent", false,
				[]byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
		}},
		{"chat_completions", func(s *GeminiMessagesCompatService, ctx context.Context, c *gin.Context, account *Account) (*ForwardResult, error) {
			return s.ForwardAsChatCompletions(ctx, c, account, []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hi"}]}`))
		}},
	}
	for _, retries := range []int{0, 1, 4} {
		for _, fw := range forwarders {
			t.Run(fmt.Sprintf("%s/retries=%d", fw.name, retries), func(t *testing.T) {
				upstream := &geminiAlways429Upstream{}
				var backoffs []int
				svc := &GeminiMessagesCompatService{
					httpUpstream: upstream,
					cfg:          &config.Config{Gateway: config.GatewayConfig{MaxSameAccountRetriesGemini: retries}},
					retryBackoff: func(attempt int) { backoffs = append(backoffs, attempt) },
				}
				retryCount := 1
				account := &Account{
					ID: 901, Platform: PlatformGemini, Type: AccountTypeAPIKey,
					Credentials:            map[string]any{"api_key": "test-key", "pool_mode": true},
					RateLimit429RetryCount: &retryCount,
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				requestCtx := WithAccount429RetryScope(context.Background())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(nil)).WithContext(requestCtx)

				result, err := fw.forward(svc, requestCtx, c, account)

				require.Nil(t, result)
				var failoverErr *UpstreamFailoverError
				require.ErrorAs(t, err, &failoverErr)
				require.True(t, failoverErr.Account429RetryExhausted)
				require.Equal(t, 1+retries+retryCount, upstream.calls, "透明预算只增加一次，随后仍执行配置的 Gemini 重试")
				require.Len(t, backoffs, retries)
				for i, attempt := range backoffs {
					require.Equal(t, i+1, attempt)
				}
			})
		}
	}
}
