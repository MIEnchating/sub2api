package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stagedUsageFailingListener struct {
	net.Listener
	failWrites *atomic.Bool
}

func (l *stagedUsageFailingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &stagedUsageFailingConn{Conn: conn, failWrites: l.failWrites}, nil
}

type stagedUsageFailingConn struct {
	net.Conn
	failWrites *atomic.Bool
}

func (c *stagedUsageFailingConn) Write(payload []byte) (int, error) {
	if c.failWrites.Load() {
		_ = c.Conn.Close()
		return 0, net.ErrClosed
	}
	return c.Conn.Write(payload)
}

func TestOpenAIWSStagedMetadataWriteFailureKeepsReceivedTerminalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := newOpenAIWSV2TestConfig()
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		for _, event := range []string{
			`{"type":"response.created","response":{"id":"resp_staged"}}`,
			`{"type":"response.completed","response":{"id":"resp_staged","model":"gpt-5.1","status":"completed","usage":{"input_tokens":2,"output_tokens":3}}}`,
		} {
			if err := conn.Write(r.Context(), coderws.MessageText, []byte(event)); err != nil {
				return
			}
		}
	}))
	t.Cleanup(upstream.Close)
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: newOpenAIWSConnPool(cfg)}
	t.Cleanup(svc.CloseOpenAIWSPool)
	account := &Account{ID: 9235, Name: "staged-usage", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test", "base_url": upstream.URL}, Extra: map[string]any{"responses_websockets_v2_enabled": true}}
	var failWrites atomic.Bool
	done := make(chan struct{})
	results := make(chan *OpenAIForwardResult, 2)
	downstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, stop := BeginOpenAIWSClientSession(r.Context(), conn, 1024)
		defer stop()
		_, first, err := ReadOpenAIWSClientMessage(ctx, conn, time.Second, coderws.StatusPolicyViolation, "first message")
		if err != nil {
			t.Error(err)
			return
		}
		failWrites.Store(true)
		ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ginCtx.Request = r.Clone(ctx)
		_ = svc.ProxyResponsesWebSocketFromClient(ctx, ginCtx, conn, account, "sk-test", first, &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) {
			results <- result
		}})
	}))
	downstream.Listener = &stagedUsageFailingListener{Listener: downstream.Listener, failWrites: &failWrites}
	downstream.Start()
	t.Cleanup(downstream.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(downstream.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"hello"}`)))
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("staged write failure did not stop the session")
	}
	require.Equal(t, int32(1), attempts.Load())
	select {
	case result := <-results:
		require.NotNil(t, result, "metadata write failure discarded the already received terminal")
		require.Equal(t, "resp_staged", result.RequestID)
		require.Equal(t, 2, result.Usage.InputTokens)
		require.Equal(t, 3, result.Usage.OutputTokens)
	default:
		t.Fatal("received terminal usage was not settled")
	}
}
