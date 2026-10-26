package service

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type vmMergeTransport func(*http.Request) (*http.Response, error)

func (f vmMergeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise the merged transport boundary without requiring a local listener.
func newVMMergeService(transport vmMergeTransport) (*OpenAIGatewayService, *Account) {
	a := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials:            map[string]any{"access_token": "account-token", "chatgpt_account_id": "owner"},
		RateLimit429RetryCount: retryCountPointer(1)}
	r := &vmTestRepo{account: a, b: &OpenAIVMBinding{AccountID: a.ID, Status: "ready"},
		c: OpenAIVMConfig{Enabled: true, BaseURL: "https://vm.example", ProxyURL: "https://vm.example", EncryptedSecret: "encrypted:" + vmTestSecret}}
	vm := NewOpenAIVMService(r, &vmTestAccounts{a: a}, &vmTestProxies{}, vmTestEncryptor{})
	vm.client.Transport = transport
	return &OpenAIGatewayService{cfg: &config.Config{}, openaiVM: vm, accountRepo: vm.accounts, cache: &stubGatewayCache{}}, a
}

func vmMergeResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"0"}, "Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"id":"resp_vm_merge","model":"gpt-5.4","usage":{"input_tokens":11,"output_tokens":7}}`))}
}

func TestOpenAIVMMergePreservesHTTPRetryAndUsage(t *testing.T) {
	calls := 0
	body := `{"model":"gpt-5.4","input":[],"previous_response_id":"resp_previous"}`
	s, a := newVMMergeService(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "vm.example", r.URL.Host)
		require.Equal(t, "/v1/accounts/7/responses", r.URL.Path)
		require.Equal(t, "Bearer "+vmTestSecret, r.Header.Get("Authorization"))
		require.Equal(t, "account-token", r.Header.Get("X-VM2API-Access-Token"))
		got, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, body, string(got))
		if calls == 1 {
			return vmMergeResponse(http.StatusTooManyRequests), nil
		}
		return vmMergeResponse(http.StatusOK), nil
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	result, err := s.Forward(context.Background(), c, a, []byte(body))
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.EqualValues(t, 11, result.Usage.InputTokens)
	require.EqualValues(t, 7, result.Usage.OutputTokens)
}

func TestOpenAIVMMergeRejectsInvalidProtectionAndRequestIntegrity(t *testing.T) {
	for _, mode := range []string{"protection", "integrity"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s, a := newVMMergeService(func(*http.Request) (*http.Response, error) {
				calls++
				return vmMergeResponse(http.StatusOK), nil
			})
			body := `{"model":"gpt-5.4","input":[]}`
			if mode == "protection" {
				a.Extra = map[string]any{AntiDegradeMarkerExtraKey: map[string]any{"enabled": true, "mode": "invalid"}}
				require.Error(t, validateRegisteredAntiDegrade(a))
			} else {
				a.Extra = map[string]any{requestIntegrityModeKey: "enforce"}
				body += `{}`
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
			_, err := s.Forward(context.Background(), c, a, []byte(body))
			require.Error(t, err)
			require.Zero(t, calls, "VM routing must preserve local protection before egress")
		})
	}
}

func TestOpenAIVMMergeCommonTransportsRetainRetry(t *testing.T) {
	for _, kind := range []string{"gateway", "account-test", "model-sync"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			s, a := newVMMergeService(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return vmMergeResponse(http.StatusTooManyRequests), nil
				}
				return vmMergeResponse(http.StatusOK), nil
			})
			req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", strings.NewReader(`{}`))
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer account-token")
			testSvc := &AccountTestService{openaiVM: s.openaiVM}
			var resp *http.Response
			switch kind {
			case "gateway":
				resp, err = s.doOpenAIUpstream(req, "", a)
			case "account-test":
				resp, err = testSvc.doOpenAIAccountTestUpstream(req, "", a, false)
			case "model-sync":
				resp, err = testSvc.doUpstreamModelsRequest(req, "", a)
			}
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, 2, calls)
		})
	}
}

func TestOpenAIVMMergeRetryBindingChangeNeverFallsBack(t *testing.T) {
	for _, kind := range []string{"gateway", "account-test", "model-sync"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			var s *OpenAIGatewayService
			var a *Account
			s, a = newVMMergeService(func(*http.Request) (*http.Response, error) {
				calls++
				// A scheduler snapshot can predate the binding marker. Removing
				// the authoritative row during retry must still fail closed.
				repo, ok := s.openaiVM.repo.(*vmTestRepo)
				require.True(t, ok)
				repo.b = nil
				return vmMergeResponse(http.StatusTooManyRequests), nil
			})
			req, err := http.NewRequest(http.MethodPost, "https://chatgpt.com/backend-api/codex/responses", strings.NewReader(`{}`))
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer account-token")
			testSvc := &AccountTestService{openaiVM: s.openaiVM}
			var resp *http.Response
			switch kind {
			case "gateway":
				resp, err = s.doOpenAIUpstream(req, "", a)
			case "account-test":
				resp, err = testSvc.doOpenAIAccountTestUpstream(req, "", a, false)
			case "model-sync":
				resp, err = testSvc.doUpstreamModelsRequest(req, "", a)
			}
			require.ErrorContains(t, err, "direct fallback is disabled")
			require.Nil(t, resp)
			require.Equal(t, 1, calls)
		})
	}
}

func TestOpenAIVMMergeQueueRetiresReplayBeforeTurnAndJoinsReader(t *testing.T) {
	for _, payload := range []string{`{"type":"response.create","model":"gpt-5.4"}`, `{"type":"response.interrupt"}`} {
		t.Run(payload, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := newStagedPassthroughConn()
			received := make(chan struct{}, 1)
			queue := newOpenAIVMQueuedClientConn(client, 1024, func(context.Context, coderws.MessageType, []byte) error { return nil })
			queue.onReceive = func() { received <- struct{}{} }
			defer queue.stop()
			done := make(chan error, 1)
			go func() { _, _, err := queue.ReadFrame(ctx); done <- err }()
			client.Send(payload)
			select {
			case <-received:
			case <-time.After(time.Second):
				t.Fatal("consumed frame did not retire replay while waiting for a turn")
			}
			queue.stop()
			select {
			case <-queue.done:
			default:
				t.Fatal("VM reader must exit before replay eligibility is inspected")
			}
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("stopping the queue did not unblock its waiting consumer")
			}
			select {
			case <-client.closed:
				t.Fatal("stopping an attempt must preserve the shared downstream connection")
			default:
			}
		})
	}
}

type vmMergeWSWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w *vmMergeWSWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

// Exercise the real VM adapter and session reader without a listening socket.
func TestOpenAIVMMergeQueuedFramesUseSessionReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	local, peer := net.Pipe()
	defer func() { _ = local.Close(); _ = peer.Close() }()
	upstream := newStagedPassthroughConn()
	svc, account := newVMMergeService(func(*http.Request) (*http.Response, error) {
		t.Error("unexpected HTTP inference request")
		return nil, nil
	})
	svc.cfg = passthroughLifecycleConfig()
	svc.cfg.Gateway.OpenAIWS.OAuthEnabled = true
	svc.openaiWSPassthroughDialer = &vmCapturingWSDialer{conn: upstream, calls: make(chan vmCapturedDial, 1)}
	turns := make(chan int, 2)
	finished := make(chan error, 1)
	transport := vmMergeTransport(func(req *http.Request) (*http.Response, error) {
		w := &vmMergeWSWriter{ResponseRecorder: httptest.NewRecorder(), conn: peer}
		server, err := coderws.Accept(w, req, nil)
		if err != nil {
			return nil, err
		}
		go func() {
			defer func() { _ = server.CloseNow() }()
			_, first, readErr := ReadOpenAIWSClientMessage(ctx, server, time.Second, coderws.StatusPolicyViolation, "missing first request")
			if readErr != nil {
				finished <- readErr
				return
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req.Clone(ctx)
			finished <- svc.ProxyResponsesWebSocketFromClient(ctx, c, server, account, "account-token", first, &OpenAIWSIngressHooks{
				BeforeRequest: func(turn int, _ []byte, _ string) error { turns <- turn; return nil },
			})
		}()
		response := w.Result()
		response.Body = local
		return response, nil
	})
	client, _, err := coderws.Dial(ctx, "ws://vm.test", &coderws.DialOptions{HTTPClient: &http.Client{Transport: transport}})
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	first := `{"type":"response.create","model":"gpt-5.1"}`
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(first)))
	require.JSONEq(t, first, string(requirePassthroughUpstreamWrite(t, upstream, time.Second)))
	second := `{"type":"response.create","model":"gpt-5.1","input":"queued"}`
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(second)))
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.interrupt"}`)))
	require.JSONEq(t, `{"type":"response.interrupt"}`, string(requirePassthroughUpstreamWrite(t, upstream, time.Second)))
	select {
	case payload := <-upstream.writes:
		t.Fatalf("queued turn started before completion: %s", payload)
	default:
	}
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_first","usage":{"input_tokens":1,"output_tokens":1}}}`)
	_, err = readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, err)
	require.JSONEq(t, second, string(requirePassthroughUpstreamWrite(t, upstream, time.Second)))
	select {
	case turn := <-turns:
		require.Equal(t, 2, turn)
	case <-ctx.Done():
		t.Fatal("queued turn bypassed authorization")
	}
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_second","usage":{"input_tokens":2,"output_tokens":1}}}`)
	_, err = readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, err)
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("VM session reader did not stop")
	}
}
