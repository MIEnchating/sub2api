package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCapacityRecoveryWSDoesNotReplayAcceptedTurnAfterUpstreamDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		_ = conn.Write(r.Context(), coderws.MessageText, []byte(`{"type":"response.created","response":{"id":"resp_accepted"}}`))
	}))
	defer upstream.Close()
	router, done := newCapacityRecoveryHandler(t, upstream.URL, service.OpenAIWSIngressModeCtxPool, false, 1)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5","input":"hello"}`)))
	_, _, err = client.Read(ctx)
	require.Error(t, err)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("uncertain upstream disconnect did not close the client session")
	}
	require.Equal(t, int32(1), attempts.Load(), "accepted progress followed by disconnect is not a capacity rejection")
}
