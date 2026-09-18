package service

import (
	"context"
	"errors"
	"sync/atomic"

	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

func openAIWSCapacityFailureCanRetry(payload []byte) bool {
	eventType := gjson.GetBytes(payload, "type").String()
	return (eventType == "error" || eventType == "response.failed") &&
		isOpenAIUpstreamCapacityShedEvent(payload) && !upstreamErrorRetryHasUsage(payload)
}

func openAIWSCommitsCapacityAttempt(payload []byte, eventType string) bool {
	if eventType == "response.output_item.added" {
		itemType := gjson.GetBytes(payload, "item.type").String()
		if itemType == "function_call" || itemType == "custom_tool_call" {
			return true
		}
	}
	return upstreamErrorRetryHasUsage(payload) || openAIStreamDataStartsClientOutput(string(payload), eventType)
}

// Passthrough has no complete replay history after its first turn, so only
// that initial attempt can stage metadata for an explicit capacity rejection.
type openAIWSCapacityFrameConn struct {
	openaiwsv2.FrameConn
	failover    func([]byte) error
	pending     []openAIWSStagedFrame
	pendingSize int64
	committed   bool
	observed    atomic.Bool
}

type openAIWSStagedFrame struct {
	messageType coderws.MessageType
	payload     []byte
}

func (c *openAIWSCapacityFrameConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	if err := ctx.Err(); err != nil {
		return coderws.MessageText, nil, err
	}
	if messageType, payload, ok := c.ReadBufferedFrame(); ok {
		return messageType, payload, nil
	}
	for {
		msgType, payload, err := c.FrameConn.ReadFrame(ctx)
		if err == nil {
			c.observed.Store(true)
		}
		if err != nil || c.committed {
			return msgType, payload, err
		}
		if msgType == coderws.MessageText && openAIWSCapacityFailureCanRetry(payload) {
			if failoverErr := c.failover(payload); failoverErr != nil {
				return msgType, nil, failoverErr
			}
		}
		eventType := gjson.GetBytes(payload, "type").String()
		commit := msgType != coderws.MessageText || openAIWSCommitsCapacityAttempt(payload, eventType) || isOpenAIWSTerminalEvent(eventType) || eventType == "error"
		if !commit && c.pendingSize+int64(len(payload)) > openAIFirstOutputStageMaxBytes {
			return msgType, nil, errors.New("OpenAI websocket first-output staging limit exceeded")
		}
		c.pending = append(c.pending, openAIWSStagedFrame{messageType: msgType, payload: append([]byte(nil), payload...)})
		c.pendingSize += int64(len(payload))
		if commit {
			c.committed = true
			return c.ReadFrame(ctx)
		}
	}
}

// ReadBufferedFrame exposes only frames already received for a committed
// attempt, so relay settlement can retain terminal usage after a failed write.
func (c *openAIWSCapacityFrameConn) ReadBufferedFrame() (coderws.MessageType, []byte, bool) {
	if !c.committed || len(c.pending) == 0 {
		return coderws.MessageText, nil, false
	}
	frame := c.pending[0]
	c.pending[0] = openAIWSStagedFrame{}
	c.pending = c.pending[1:]
	c.pendingSize -= int64(len(frame.payload))
	return frame.messageType, frame.payload, true
}
