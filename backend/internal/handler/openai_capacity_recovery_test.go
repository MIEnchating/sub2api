package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/testutil"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const capacityRecoveryFailure = `{"type":"response.failed","response":{"id":"resp_busy","status":"failed","error":{"code":"server_error","type":"server_error","message":"The service is busy. Please retry later."}}}`
const capacityRecoverySuccess = `{"type":"response.completed","response":{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"recovered"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`

type capacityRecoveryHTTPUpstream struct{}

func (*capacityRecoveryHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func (u *capacityRecoveryHTTPUpstream) DoWithTLS(req *http.Request, proxy string, account int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, account, concurrency)
}

func newCapacityRecoveryHandler(t *testing.T, upstreamURL, mode string, passthrough bool, retryCount int) (*gin.Engine, <-chan struct{}) {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 3
	account := service.Account{
		ID: 9915, Name: "capacity-recovery", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": upstreamURL, "pool_mode": true, "pool_mode_retry_count": retryCount},
		Extra: map[string]any{
			"openai_passthrough":                            passthrough,
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    mode,
		},
	}
	accountRepo := &openAIWSUsageHandlerAccountRepoStub{account: account}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 16)}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(
		accountRepo, usageRepo, nil, nil, nil, nil, testutil.NewRedisGatewayCache(t), cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billing, &capacityRecoveryHTTPUpstream{}, &service.DeferredService{},
		nil, nil, nil, nil, nil, nil, nil,
	)
	t.Cleanup(gateway.CloseOpenAIWSPool)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(cache), billing, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
	h.maxAccountSwitches = 0
	groupID := int64(9916)
	apiKey := &service.APIKey{ID: 9917, GroupID: &groupID, User: &service.User{ID: 9918, Status: service.StatusActive}, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	done := make(chan struct{})
	router.GET("/v1/responses", func(c *gin.Context) { defer close(done); h.ResponsesWebSocket(c) })
	router.POST("/v1/responses", h.Responses)
	return router, done
}

func TestCapacityRecoveryWSKeepsClientConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{service.OpenAIWSIngressModeHTTPBridge, service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serveCapacityRecoveryAttempt(w, r, attempts.Add(1) == 1)
			}))
			defer upstream.Close()
			router, done := newCapacityRecoveryHandler(t, upstream.URL, mode, false, 1)
			server := httptest.NewServer(router)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			defer func() { _ = client.CloseNow() }()
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5","input":"hello"}`)))
			_, payload, err := client.Read(ctx)
			require.NoError(t, err)
			require.Equal(t, "response.completed", gjson.GetBytes(payload, "type").String(), string(payload))
			require.Equal(t, "resp_ok", gjson.GetBytes(payload, "response.id").String())
			require.Equal(t, int32(2), attempts.Load())
			_ = client.CloseNow()
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("websocket handler did not stop after client disconnect")
			}
		})
	}
}

func TestCapacityRecoveryHTTPKeepsClientRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serveCapacityRecoveryAttempt(w, r, attempts.Add(1) == 1)
			}))
			defer upstream.Close()
			router, _ := newCapacityRecoveryHandler(t, upstream.URL, service.OpenAIWSIngressModeHTTPBridge, passthrough, 1)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","input":"hello","stream":true}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, 1, strings.Count(rec.Body.String(), "event: response.completed"))
			require.NotContains(t, rec.Body.String(), "resp_busy")
			require.NotContains(t, rec.Body.String(), "response.failed")
			require.Equal(t, int32(2), attempts.Load())
		})
	}
}

func TestCapacityRecoveryWSStopsAfterConfiguredRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{service.OpenAIWSIngressModeHTTPBridge, service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				serveCapacityRecoveryAttempt(w, r, true)
			}))
			defer upstream.Close()
			router, done := newCapacityRecoveryHandler(t, upstream.URL, mode, false, 1)
			server := httptest.NewServer(router)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			defer func() { _ = client.CloseNow() }()
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5","input":"hello"}`)))
			_, _, err = client.Read(ctx)
			require.Equal(t, coderws.StatusTryAgainLater, coderws.CloseStatus(err))
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("capacity retry did not exhaust")
			}
			require.Equal(t, int32(2), attempts.Load())
		})
	}
}

func serveCapacityRecoveryAttempt(w http.ResponseWriter, r *http.Request, busy bool) {
	events := []string{capacityRecoverySuccess}
	if busy {
		events = []string{`{"type":"response.created","response":{"id":"resp_busy"}}`, capacityRecoveryFailure}
	}
	serveCapacityRecoveryEvents(w, r, events)
}

func serveCapacityRecoveryEvents(w http.ResponseWriter, r *http.Request, events []string) {
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
		for _, event := range events {
			if err := conn.Write(ctx, coderws.MessageText, []byte(event)); err != nil {
				return
			}
		}
		_, _, _ = conn.Read(ctx)
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range events {
		_, _ = io.WriteString(w, "event: "+gjson.Get(event, "type").String()+"\ndata: "+event+"\n\n")
	}
}
