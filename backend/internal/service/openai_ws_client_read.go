package service

import (
	"context"
	"errors"
	"time"

	coderws "github.com/coder/websocket"
)

type openAIWSClientReadResult struct {
	messageType coderws.MessageType
	payload     []byte
	err         error
}

// ReadOpenAIWSClientMessage keeps one reader alive while control events send
// their close frame, then closes the transport and joins that reader.
func ReadOpenAIWSClientMessage(
	controlCtx context.Context,
	conn *coderws.Conn,
	timeout time.Duration,
	timeoutStatus coderws.StatusCode,
	timeoutReason string,
) (coderws.MessageType, []byte, error) {
	return readOpenAIWSClientMessageWithTimeoutStart(
		controlCtx,
		conn,
		timeout,
		timeoutStatus,
		timeoutReason,
		nil,
		nil,
		nil,
	)
}

// readOpenAIWSClientMessageWithTimeoutStart supports readers whose timeout
// starts after a state transition, such as a completed passthrough turn. When
// timeoutActive is nil, a positive timeout starts immediately.
func readOpenAIWSClientMessageWithTimeoutStart(
	controlCtx context.Context,
	conn *coderws.Conn,
	timeout time.Duration,
	timeoutStatus coderws.StatusCode,
	timeoutReason string,
	timeoutStart <-chan struct{},
	timeoutActive func() bool,
	attemptCtx context.Context,
) (coderws.MessageType, []byte, error) {
	if conn == nil {
		return 0, nil, errors.New("openai websocket client connection is nil")
	}
	if controlCtx == nil {
		controlCtx = context.Background()
	}

	session := openAIWSClientSessionFromContext(controlCtx, conn)
	readDone := make(chan openAIWSClientReadResult, 1)
	var sessionDone <-chan struct{}
	var attemptDone <-chan struct{}
	if session != nil {
		readDone = session.frames
		sessionDone = session.done
		if attemptCtx != nil {
			attemptDone = attemptCtx.Done()
		}
	} else {
		go func() {
			messageType, payload, err := conn.Read(context.Background())
			readDone <- openAIWSClientReadResult{messageType: messageType, payload: payload, err: err}
		}()
	}

	var timer *time.Timer
	var timeoutCh <-chan time.Time
	startTimeout := func() {
		if timeout <= 0 || (timeoutActive != nil && !timeoutActive()) {
			return
		}
		if timer == nil {
			timer = time.NewTimer(timeout)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(timeout)
		}
		timeoutCh = timer.C
	}
	if timeoutActive == nil || timeoutActive() {
		startTimeout()
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	closeAndJoin := func(status coderws.StatusCode, reason string, cause error) (coderws.MessageType, []byte, error) {
		_ = conn.Close(status, reason)
		_ = conn.CloseNow()
		if session != nil {
			<-session.done
		} else {
			<-readDone
		}
		return 0, nil, NewOpenAIWSClientCloseError(status, reason, cause)
	}
	closeForControlCancellation := func() (coderws.MessageType, []byte, error) {
		if session != nil {
			select {
			case <-session.done:
				return 0, nil, session.readErr
			default:
			}
		}
		cause := context.Cause(controlCtx)
		if errors.Is(cause, ErrOpenAIWSIngressLeaseLost) {
			return closeAndJoin(coderws.StatusTryAgainLater, "websocket ingress capacity lease lost; please reconnect", cause)
		}
		return closeAndJoin(coderws.StatusGoingAway, "websocket request canceled", cause)
	}

	for {
		if attemptDone != nil && attemptCtx.Err() != nil && controlCtx.Err() == nil {
			return 0, nil, attemptCtx.Err()
		}
		if session != nil {
			select {
			case <-session.done:
				return 0, nil, session.readErr
			default:
			}
		}
		select {
		case result := <-readDone:
			if session != nil {
				session.pendingBytes.Add(-int64(len(result.payload)))
				select {
				case <-session.done:
					return 0, nil, session.readErr
				default:
				}
				if err := controlCtx.Err(); err != nil {
					return 0, nil, err
				}
			}
			return result.messageType, result.payload, result.err
		case <-sessionDone:
			return 0, nil, session.readErr
		case <-timeoutStart:
			startTimeout()
		case <-timeoutCh:
			return closeAndJoin(timeoutStatus, timeoutReason, context.DeadlineExceeded)
		case <-attemptDone:
			if controlCtx.Err() != nil {
				return closeForControlCancellation()
			}
			return 0, nil, attemptCtx.Err()
		case <-controlCtx.Done():
			return closeForControlCancellation()
		}
	}
}
