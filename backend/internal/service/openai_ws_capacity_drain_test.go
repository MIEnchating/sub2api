package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type capacityMetadataWriteFailureConn struct {
	failAt int
	writes int
}

func (c *capacityMetadataWriteFailureConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	<-ctx.Done()
	return coderws.MessageText, nil, ctx.Err()
}

func (c *capacityMetadataWriteFailureConn) WriteFrame(context.Context, coderws.MessageType, []byte) error {
	c.writes++
	if c.writes == c.failAt {
		return net.ErrClosed
	}
	return nil
}

func (*capacityMetadataWriteFailureConn) Close() error { return nil }

func TestOpenAIWSCapacityBufferedTerminalSettlesAfterMetadataWriteFails(t *testing.T) {
	for _, test := range []struct {
		name         string
		failAt       int
		terminalType string
	}{
		{name: "first metadata write fails", failAt: 1, terminalType: "response.completed"},
		{name: "later metadata write fails", failAt: 2, terminalType: "response.completed"},
		{name: "failed terminal retains usage", failAt: 1, terminalType: "response.failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := newStagedPassthroughConn()
			defer func() { _ = upstream.Close() }()
			upstream.Send(`{"type":"response.created","response":{"id":"buffered-terminal"}}`)
			upstream.Send(`{"type":"response.in_progress","response":{"id":"buffered-terminal"}}`)
			upstream.Send(fmt.Sprintf(`{"type":%q,"response":{"id":"buffered-terminal","model":"gpt-5.1","usage":{"input_tokens":7,"output_tokens":3}}}`, test.terminalType))
			upstream.Send(`{"type":"response.created","response":{"id":"must-not-read"}}`)
			conn := &openAIWSCapacityFrameConn{FrameConn: upstream, failover: func([]byte) error { return errors.New("unexpected replay") }}
			var turns []openaiwsv2.RelayTurnResult
			terminalAttributed := false
			attributedAtSettlement := false
			result, exit := openaiwsv2.Relay(context.Background(), &capacityMetadataWriteFailureConn{failAt: test.failAt}, conn,
				[]byte(`{"type":"response.create","model":"gpt-5.1","input":"hello"}`), openaiwsv2.RelayOptions{
					StartClientAfterFirstDownstream: true,
					BeforeWriteClient: func(_ coderws.MessageType, payload []byte, _ bool) error {
						if gjson.GetBytes(payload, "type").String() == test.terminalType {
							terminalAttributed = true
						}
						return nil
					},
					OnTurnComplete: func(turn openaiwsv2.RelayTurnResult) {
						attributedAtSettlement = terminalAttributed
						turns = append(turns, turn)
					},
				})
			require.NotNil(t, exit)
			require.ErrorIs(t, exit.Err, net.ErrClosed)
			require.Len(t, turns, 1, "already read terminal usage must settle exactly once")
			require.True(t, attributedAtSettlement, "failure attribution must precede settlement")
			require.Equal(t, "buffered-terminal", turns[0].RequestID)
			require.Equal(t, test.terminalType, turns[0].TerminalEventType)
			require.Equal(t, 7, result.Usage.InputTokens)
			require.Equal(t, 3, result.Usage.OutputTokens)
			require.Equal(t, result.Usage, turns[0].Usage)
			require.Len(t, upstream.writes, 1, "settling buffered frames must not submit another request")
			require.Len(t, upstream.frames, 1, "settling buffered frames must not read more upstream frames")
		})
	}
}
