package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSClientSessionPreservesQueuedFrameOrderAndType(t *testing.T) {
	type frame struct {
		kind    coderws.MessageType
		payload string
	}
	want := []frame{{coderws.MessageText, "first"}, {coderws.MessageBinary, "second"}, {coderws.MessageText, ""}}
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFrames := func() { releaseOnce.Do(func() { close(release) }) }
	result := make(chan []frame, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		ctx, stop := BeginOpenAIWSClientSession(r.Context(), conn, 1024)
		defer stop()
		<-release
		got := make([]frame, 0, len(want))
		for range want {
			kind, payload, err := ReadOpenAIWSClientMessage(ctx, conn, time.Second, coderws.StatusNormalClosure, "idle")
			if err != nil {
				t.Error(err)
				break
			}
			got = append(got, frame{kind, string(payload)})
		}
		result <- got
	}))
	defer server.Close()
	defer releaseFrames()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	for _, item := range want {
		require.NoError(t, client.Write(ctx, item.kind, []byte(item.payload)))
	}
	releaseFrames()
	select {
	case got := <-result:
		require.Equal(t, want, got)
	case <-ctx.Done():
		t.Fatal("queued frames were not delivered")
	}
}

func TestOpenAIWSClientSessionRejectsPendingPayloadBeyondBound(t *testing.T) {
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			result <- err
			return
		}
		ctx, stop := BeginOpenAIWSClientSession(r.Context(), conn, 8)
		defer stop()
		<-ctx.Done()
		result <- context.Cause(ctx)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	_ = client.Write(ctx, coderws.MessageText, []byte("123456789"))
	select {
	case err := <-result:
		var closeErr *OpenAIWSClientCloseError
		require.ErrorAs(t, err, &closeErr)
		require.Equal(t, coderws.StatusPolicyViolation, closeErr.StatusCode())
	case <-ctx.Done():
		t.Fatal("pending input exceeded its memory bound without closing")
	}
}

func TestOpenAIWSClientSessionDoesNotDispatchQueuedFramesAfterDisconnect(t *testing.T) {
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			result <- err
			return
		}
		ctx, stop := BeginOpenAIWSClientSession(r.Context(), conn, 1024)
		defer stop()
		<-ctx.Done()
		_, payload, err := ReadOpenAIWSClientMessage(ctx, conn, 0, 0, "")
		if len(payload) != 0 {
			t.Error("disconnected client dispatched a queued request")
		}
		result <- err
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create"}`)))
	require.NoError(t, client.CloseNow())
	select {
	case err := <-result:
		require.Error(t, err)
	case <-ctx.Done():
		t.Fatal("disconnected reader did not stop")
	}
}

func TestOpenAIWSClientAttemptCancellationKeepsSessionForNextReader(t *testing.T) {
	cancelAttempt := make(chan context.CancelFunc, 1)
	attemptResult := make(chan error, 1)
	nextResult := make(chan openAIWSClientReadResult, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			attemptResult <- err
			return
		}
		ctx, stop := BeginOpenAIWSClientSession(r.Context(), conn, 1024)
		defer stop()
		attempt, cancel := context.WithCancel(ctx)
		defer cancel()
		cancelAttempt <- cancel
		_, _, err = readOpenAIWSClientMessageWithTimeoutStart(ctx, conn, 0, 0, "", nil, nil, attempt)
		attemptResult <- err
		kind, payload, err := ReadOpenAIWSClientMessage(ctx, conn, time.Second, coderws.StatusPolicyViolation, "next message")
		nextResult <- openAIWSClientReadResult{messageType: kind, payload: payload, err: err}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer client.CloseNow()
	select {
	case stopAttempt := <-cancelAttempt:
		stopAttempt()
	case <-ctx.Done():
		t.Fatal("attempt reader did not start")
	}
	select {
	case err := <-attemptResult:
		require.ErrorIs(t, err, context.Canceled)
	case <-ctx.Done():
		t.Fatal("attempt reader ignored cancellation")
	}
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.cancel"}`)))
	select {
	case got := <-nextResult:
		require.NoError(t, got.err)
		require.Equal(t, coderws.MessageText, got.messageType)
		require.JSONEq(t, `{"type":"response.cancel"}`, string(got.payload))
	case <-ctx.Done():
		t.Fatal("next attempt lost the live client session")
	}
}

func TestOpenAIWSClientUpstreamContextStillObservesShutdownAfterClientDisconnect(t *testing.T) {
	parent, cancelParent := context.WithCancelCause(context.Background())
	defer cancelParent(context.Canceled)
	clientCtx, cancelClient := context.WithCancelCause(parent)
	clientCtx = context.WithValue(clientCtx, openAIWSClientSessionKey{}, &openAIWSClientSession{parent: parent})
	cancelClient(errors.Join(context.Canceled, errOpenAIWSClientSessionDisconnected))
	upstreamCtx, stop := newOpenAIWSClientUpstreamContext(clientCtx)
	defer stop()
	require.NoError(t, upstreamCtx.Err(), "committed usage drain can outlive the client socket")
	shutdown := errors.New("server is shutting down")
	cancelParent(shutdown)
	select {
	case <-upstreamCtx.Done():
		require.ErrorIs(t, context.Cause(upstreamCtx), shutdown)
	case <-time.After(time.Second):
		t.Fatal("client usage drain ignored server shutdown")
	}
}

func TestOpenAIWSCommittedTurnContextCleanupCanBeRepeated(t *testing.T) {
	upstreamCtx, stop := newOpenAIWSCommittedTurnContext(context.Background())
	stop()
	require.ErrorIs(t, upstreamCtx.Err(), context.Canceled)
	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("repeated committed turn cleanup did not return")
	}
}

func TestOpenAIWSClientSessionPreservesControlCloseFrames(t *testing.T) {
	for _, test := range []struct {
		name      string
		leaseLost bool
		status    coderws.StatusCode
		reason    string
	}{
		{name: "first message timeout", status: coderws.StatusPolicyViolation, reason: "missing first response.create message"},
		{name: "ingress lease lost", leaseLost: true, status: coderws.StatusTryAgainLater, reason: "websocket ingress capacity lease lost; please reconnect"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, cancelParent := context.WithCancelCause(context.Background())
			defer cancelParent(context.Canceled)
			started := make(chan struct{})
			result := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					result <- err
					return
				}
				ctx, stop := BeginOpenAIWSClientSession(parent, conn, 1024)
				defer stop()
				close(started)
				_, _, err = ReadOpenAIWSClientMessage(ctx, conn, 50*time.Millisecond, test.status, test.reason)
				result <- err
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			require.NoError(t, err)
			defer client.CloseNow()
			<-started
			if test.leaseLost {
				cancelParent(ErrOpenAIWSIngressLeaseLost)
			}
			_, _, err = client.Read(ctx)
			var clientClose coderws.CloseError
			require.ErrorAs(t, err, &clientClose)
			require.Equal(t, test.status, clientClose.Code)
			require.Equal(t, test.reason, clientClose.Reason)
			select {
			case err := <-result:
				var closeErr *OpenAIWSClientCloseError
				require.ErrorAs(t, err, &closeErr)
				require.Equal(t, test.status, closeErr.StatusCode())
				if test.leaseLost {
					require.True(t, errors.Is(err, ErrOpenAIWSIngressLeaseLost))
				}
			case <-ctx.Done():
				t.Fatal("session reader did not stop after control close")
			}
		})
	}
}
