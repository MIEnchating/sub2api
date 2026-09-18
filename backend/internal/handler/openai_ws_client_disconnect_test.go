package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSClientDisconnectBeforeOutputCancelsPendingUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{service.OpenAIWSIngressModeHTTPBridge, service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			upstreamCanceled := make(chan struct{})
			cleanupUpstream := make(chan struct{})
			var startedOnce, canceledOnce sync.Once
			var attempts atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
					conn, err := coderws.Accept(w, r, nil)
					if err != nil {
						return
					}
					defer conn.CloseNow()
					readCtx, cancelRead := context.WithCancel(r.Context())
					defer cancelRead()
					go func() {
						select {
						case <-cleanupUpstream:
							_ = conn.CloseNow()
						case <-readCtx.Done():
						}
					}()
					if _, _, err := conn.Read(readCtx); err != nil {
						return
					}
					startedOnce.Do(func() { close(started) })
					if _, _, err := conn.Read(readCtx); err != nil {
						canceledOnce.Do(func() { close(upstreamCanceled) })
					}
					return
				}
				_, _ = io.Copy(io.Discard, r.Body)
				startedOnce.Do(func() { close(started) })
				select {
				case <-r.Context().Done():
					canceledOnce.Do(func() { close(upstreamCanceled) })
				case <-cleanupUpstream:
				}
			}))
			t.Cleanup(upstream.Close)
			defer close(cleanupUpstream)
			router, done := newCapacityRecoveryHandler(t, upstream.URL, mode, false, 4)
			server := httptest.NewServer(router)
			t.Cleanup(server.Close)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.CloseNow() })
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5","input":"hello"}`)))
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("upstream request did not start")
			}
			require.NoError(t, client.CloseNow())
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("disconnected websocket left its upstream attempt running")
			}
			select {
			case <-upstreamCanceled:
			case <-ctx.Done():
				t.Fatal("upstream did not observe client cancellation")
			}
			require.Equal(t, int32(1), attempts.Load(), "client disconnect must not dispatch retries")
		})
	}
}
