package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	coderws "github.com/coder/websocket"
)

type openAIWSClientSessionKey struct{}

var errOpenAIWSClientSessionDisconnected = errors.New("websocket client disconnected")

type openAIWSClientSession struct {
	conn         *coderws.Conn
	parent       context.Context
	frames       chan openAIWSClientReadResult
	done         chan struct{}
	pendingBytes atomic.Int64
	readErr      error
}

// BeginOpenAIWSClientSession keeps the sole socket reader alive during upstream
// waits and retry backoff. HTTP request cancellation cannot observe a closed
// websocket after the connection has been hijacked.
func BeginOpenAIWSClientSession(parent context.Context, conn *coderws.Conn, maxPendingBytes int64) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}
	if conn == nil {
		return parent, func() {}
	}
	if existing := openAIWSClientSessionFromContext(parent, conn); existing != nil {
		return parent, func() {}
	}
	if maxPendingBytes <= 0 {
		maxPendingBytes = openAIWSClientReadLimitBytesDefault
	}
	sessionCtx, cancel := context.WithCancelCause(parent)
	session := &openAIWSClientSession{conn: conn, parent: parent, frames: make(chan openAIWSClientReadResult, 64), done: make(chan struct{})}
	go func() {
		defer func() {
			close(session.done)
			cancel(errors.Join(context.Canceled, errOpenAIWSClientSessionDisconnected, session.readErr))
		}()
		for {
			messageType, payload, err := conn.Read(context.Background())
			if err != nil {
				session.readErr = err
				return
			}
			pending := session.pendingBytes.Add(int64(len(payload)))
			if pending <= maxPendingBytes {
				select {
				case session.frames <- openAIWSClientReadResult{messageType: messageType, payload: payload}:
					continue
				default:
				}
			}
			session.readErr = NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "too many pending websocket messages", nil)
			cancel(errors.Join(context.Canceled, session.readErr))
			_ = conn.Close(coderws.StatusPolicyViolation, "too many pending websocket messages")
			_ = conn.CloseNow()
			return
		}
	}()
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			cancel(context.Canceled)
			_ = conn.CloseNow()
			<-session.done
		})
	}
	return context.WithValue(sessionCtx, openAIWSClientSessionKey{}, session), cleanup
}

func isOpenAIWSClientSessionDisconnected(ctx context.Context) bool {
	return ctx != nil && errors.Is(context.Cause(ctx), errOpenAIWSClientSessionDisconnected)
}

// Upstream draining may outlive the socket, but never the original request,
// ingress lease or server shutdown. Callers must bound their drain lifetime.
func newOpenAIWSClientUpstreamContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	session, _ := ctx.Value(openAIWSClientSessionKey{}).(*openAIWSClientSession)
	if session == nil {
		return context.WithCancel(ctx)
	}
	upstreamCtx, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	parentDone := make(chan struct{})
	stopParent := context.AfterFunc(session.parent, func() {
		defer close(parentDone)
		cancel(context.Cause(session.parent))
	})
	if session.parent.Err() != nil {
		cancel(context.Cause(session.parent))
	}
	var cleanupOnce sync.Once
	return upstreamCtx, func() {
		cleanupOnce.Do(func() {
			if !stopParent() {
				<-parentDone
			}
			cancel(context.Canceled)
		})
	}
}

func newOpenAIWSCommittedTurnContext(ctx context.Context) (context.Context, context.CancelFunc) {
	upstreamCtx, cancelUpstream := newOpenAIWSClientUpstreamContext(ctx)
	cancelDone := make(chan struct{})
	stopCancellation := context.AfterFunc(ctx, func() {
		defer close(cancelDone)
		if !isOpenAIWSClientSessionDisconnected(ctx) {
			cancelUpstream()
			return
		}
		timer := time.NewTimer(1200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			cancelUpstream()
		case <-upstreamCtx.Done():
		}
	})
	var cleanupOnce sync.Once
	return upstreamCtx, func() {
		cleanupOnce.Do(func() {
			cancelUpstream()
			if !stopCancellation() {
				<-cancelDone
			}
		})
	}
}

func openAIWSClientSessionFromContext(ctx context.Context, conn *coderws.Conn) *openAIWSClientSession {
	if ctx == nil {
		return nil
	}
	session, _ := ctx.Value(openAIWSClientSessionKey{}).(*openAIWSClientSession)
	if session == nil || session.conn != conn {
		return nil
	}
	return session
}
