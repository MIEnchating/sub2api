package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSClientSessionCommittedTurnCancellation(t *testing.T) {
	for _, mode := range []string{OpenAIWSIngressModeCtxPool, OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			for _, test := range []struct {
				name               string
				shutdown, terminal bool
			}{
				{name: "client disconnect drains same turn terminal usage", terminal: true},
				{name: "client disconnect bounds terminal drain"},
				{name: "parent shutdown does not enter client drain", shutdown: true},
			} {
				t.Run(test.name, func(t *testing.T) {
					gin.SetMode(gin.TestMode)
					cfg := newOpenAIWSV2TestConfig()
					cfg.Security.URLAllowlist.Enabled = false
					cfg.Security.URLAllowlist.AllowInsecureHTTP = true
					cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 10
					cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 2
					cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
					terminalGate := make(chan struct{})
					var releaseOnce sync.Once
					releaseTerminal := func() { releaseOnce.Do(func() { close(terminalGate) }) }
					defer releaseTerminal()
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
						if err := conn.Write(r.Context(), coderws.MessageText, []byte(`{"type":"response.output_text.delta","delta":"partial"}`)); err != nil {
							return
						}
						<-terminalGate
						_ = conn.Write(r.Context(), coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"drained","model":"gpt-5.1","status":"completed","usage":{"input_tokens":2,"output_tokens":3}}}`))
						_, _, _ = conn.Read(r.Context())
					}))
					t.Cleanup(upstream.Close)
					pool := newOpenAIWSConnPool(cfg)
					svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool}
					t.Cleanup(svc.CloseOpenAIWSPool)
					account := &Account{ID: 9234, Name: "session-drain", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test", "base_url": upstream.URL}, Extra: map[string]any{"responses_websockets_v2_enabled": true}}
					account.Extra["openai_apikey_responses_websockets_v2_mode"] = mode
					parent, cancelParent := context.WithCancel(context.Background())
					defer cancelParent()
					sessionCanceled := make(chan struct{})
					done := make(chan struct{})
					turnResults := make(chan *OpenAIForwardResult, 2)
					downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						defer close(done)
						conn, err := coderws.Accept(w, r, nil)
						if err != nil {
							t.Error(err)
							return
						}
						ctx, stop := BeginOpenAIWSClientSession(parent, conn, 1024)
						defer stop()
						stopNotice := context.AfterFunc(ctx, func() { close(sessionCanceled) })
						defer stopNotice()
						ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
						ginCtx.Request = r.Clone(ctx)
						_, first, err := ReadOpenAIWSClientMessage(ctx, conn, time.Second, coderws.StatusPolicyViolation, "first message")
						if err != nil {
							t.Error(err)
							return
						}
						_ = svc.ProxyResponsesWebSocketFromClient(ctx, ginCtx, conn, account, "sk-test", first, &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) {
							if result != nil {
								turnResults <- result
							}
						}})
					}))
					t.Cleanup(downstream.Close)
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(downstream.URL, "http"), nil)
					require.NoError(t, err)
					defer client.CloseNow()
					require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"hello"}`)))
					_, _, err = client.Read(ctx)
					require.NoError(t, err)
					var closedRead <-chan error
					if test.shutdown {
						readDone := make(chan error, 1)
						closedRead = readDone
						go func() {
							_, _, err := client.Read(ctx)
							readDone <- err
						}()
						cancelParent()
					} else {
						require.NoError(t, client.CloseNow())
					}
					select {
					case <-sessionCanceled:
					case <-ctx.Done():
						t.Fatal("client session was not canceled")
					}
					if test.terminal {
						releaseTerminal()
					}
					select {
					case <-done:
					case <-ctx.Done():
						t.Fatal("canceled turn exceeded the bounded usage drain")
					}
					if closedRead != nil {
						select {
						case <-closedRead:
						case <-ctx.Done():
							t.Fatal("client did not finish the shutdown close handshake")
						}
					}
					require.Equal(t, int32(1), attempts.Load(), "drain must not start another attempt")
					if test.terminal {
						select {
						case result := <-turnResults:
							require.Equal(t, 2, result.Usage.InputTokens)
							require.Equal(t, 3, result.Usage.OutputTokens)
						default:
							t.Fatal("committed turn lost terminal usage after client disconnected")
						}
					} else {
						select {
						case result := <-turnResults:
							t.Fatalf("no terminal evidence should produce usage: %+v", result)
						default:
						}
					}
				})
			}
		})
	}
}
