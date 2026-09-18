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

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCapacityRecoveryWSRetriesOnlyCurrentTextTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{service.OpenAIWSIngressModeHTTPBridge, service.OpenAIWSIngressModeCtxPool} {
		t.Run(mode, func(t *testing.T) {
			var attempts atomic.Int32
			requests := make(chan string, 4)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				eventsFor := func(payload []byte) []string {
					requests <- string(payload)
					attempt := attempts.Add(1)
					if attempt == 1 || attempt == 3 {
						return []string{`{"type":"response.created","response":{"id":"resp_busy"}}`, capacityRecoveryFailure}
					}
					return []string{capacityRecoverySuccess}
				}
				if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
					conn, err := coderws.Accept(w, r, nil)
					if err != nil {
						return
					}
					defer func() { _ = conn.CloseNow() }()
					ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
					defer cancel()
					for {
						_, payload, err := conn.Read(ctx)
						if err != nil {
							return
						}
						for _, event := range eventsFor(payload) {
							if err := conn.Write(ctx, coderws.MessageText, []byte(event)); err != nil {
								return
							}
						}
					}
				}
				payload, _ := io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				for _, event := range eventsFor(payload) {
					_, _ = io.WriteString(w, "event: "+gjson.Get(event, "type").String()+"\ndata: "+event+"\n\n")
				}
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
			for _, request := range []string{
				`{"type":"response.create","model":"gpt-5","input":[{"role":"user","content":"first question"}]}`,
				`{"type":"response.create","model":"gpt-5","previous_response_id":"resp_ok","input":[{"role":"user","content":"second question"}]}`,
			} {
				require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(request)))
				_, payload, err := client.Read(ctx)
				require.NoError(t, err)
				require.Equal(t, "response.completed", gjson.GetBytes(payload, "type").String(), string(payload))
			}
			require.Equal(t, int32(4), attempts.Load())
			<-requests
			<-requests
			<-requests
			retry := <-requests
			require.False(t, gjson.Get(retry, "previous_response_id").Exists())
			require.Contains(t, gjson.Get(retry, "input").Raw, "first question")
			require.Contains(t, gjson.Get(retry, "input").Raw, "second question")
			_ = client.CloseNow()
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("websocket handler did not stop")
			}
		})
	}
}

func TestCapacityRecoveryWSDoesNotReplayConsumedTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{service.OpenAIWSIngressModeHTTPBridge, service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough} {
		for _, tc := range []struct {
			name    string
			events  []string
			request string
		}{
			{name: "text output", events: []string{`{"type":"response.output_text.delta","delta":"partial"}`, capacityRecoveryFailure}},
			{name: "tool output", events: []string{`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"write_file","arguments":""}}`, capacityRecoveryFailure}},
			{name: "terminal usage", events: []string{`{"type":"response.failed","response":{"usage":{"input_tokens":10,"output_tokens":0},"error":{"message":"The service is busy. Please retry later."}}}`}},
			{name: "cached usage without total", events: []string{`{"type":"response.failed","response":{"usage":{"input_tokens_details":{"cached_tokens":2}},"error":{"message":"The service is busy. Please retry later."}}}`}},
			{name: "reasoning usage without total", events: []string{`{"type":"response.failed","response":{"usage":{"output_tokens_details":{"reasoning_tokens":3}},"error":{"message":"The service is busy. Please retry later."}}}`}},
			{name: "image usage without total", events: []string{`{"type":"response.failed","response":{"usage":{"output_tokens_details":{"image_tokens":4}},"error":{"message":"The service is busy. Please retry later."}}}`}},
			{name: "tool result input", events: []string{capacityRecoveryFailure}, request: `{"type":"response.create","model":"gpt-5","input":[{"type":"function_call","call_id":"call_1","name":"write_file","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"done"}]}`},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				var attempts atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					serveCapacityRecoveryEvents(w, r, tc.events)
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
				request := tc.request
				if request == "" {
					request = `{"type":"response.create","model":"gpt-5","input":"hello"}`
				}
				require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(request)))
				for _, event := range tc.events {
					_, payload, err := client.Read(ctx)
					require.NoError(t, err)
					require.Equal(t, gjson.Get(event, "type").String(), gjson.GetBytes(payload, "type").String())
				}
				require.Equal(t, int32(1), attempts.Load())
				_ = client.CloseNow()
				select {
				case <-done:
				case <-ctx.Done():
					t.Fatal("websocket handler did not stop")
				}
			})
		}
	}
}

func TestCapacityRecoveryWSPassthroughDoesNotReplayAnEarlierTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
			event := capacityRecoveryFailure
			if attempts.Add(1) == 1 {
				event = capacityRecoverySuccess
			}
			if err := conn.Write(r.Context(), coderws.MessageText, []byte(event)); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	router, done := newCapacityRecoveryHandler(t, upstream.URL, service.OpenAIWSIngressModePassthrough, false, 1)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	for _, eventType := range []string{"response.completed", "response.failed"} {
		require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5","input":"hello"}`)))
		_, payload, err := client.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, eventType, gjson.GetBytes(payload, "type").String())
	}
	_ = client.CloseNow()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("passthrough handler did not stop")
	}
	require.Equal(t, int32(2), attempts.Load(), "later passthrough failure must not replay the initial request")
}
