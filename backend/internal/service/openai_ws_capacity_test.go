package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestOpenAIWSCapacityBufferDiscardsMetadataBeforeFailover(t *testing.T) {
	upstream := newStagedPassthroughConn()
	defer upstream.Close()
	upstream.Send(`{"type":"response.created","response":{"id":"failed"}}`)
	upstream.Send(openAIServiceBusyFailedEvent)
	failure := errors.New("capacity failover")
	conn := &openAIWSCapacityFrameConn{FrameConn: upstream, failover: func([]byte) error { return failure }}
	_, payload, err := conn.ReadFrame(context.Background())
	require.ErrorIs(t, err, failure)
	require.Empty(t, payload)
}

func TestOpenAIWSCapacityBufferPreservesMetadataBeforeCommittedFrames(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind coderws.MessageType
		body string
	}{
		{name: "text", kind: coderws.MessageText, body: `{"type":"response.output_text.delta","delta":"hello"}`},
		{name: "function call start", kind: coderws.MessageText, body: `{"type":"response.output_item.added","item":{"type":"function_call","arguments":""}}`},
		{name: "custom tool start", kind: coderws.MessageText, body: `{"type":"response.output_item.added","item":{"type":"custom_tool_call","input":""}}`},
		{name: "opaque binary", kind: coderws.MessageBinary, body: "opaque response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := newStagedPassthroughConn()
			defer upstream.Close()
			metadata := `{"type":"response.created","response":{"id":"committed"}}`
			upstream.Send(metadata)
			upstream.frames <- stagedPassthroughFrame{messageType: tc.kind, payload: []byte(tc.body)}
			upstream.Send(openAIServiceBusyFailedEvent)
			conn := &openAIWSCapacityFrameConn{FrameConn: upstream, failover: func([]byte) error { return errors.New("must not retry") }}
			_, payload, err := conn.ReadFrame(context.Background())
			require.NoError(t, err)
			require.JSONEq(t, metadata, string(payload))
			kind, payload, err := conn.ReadFrame(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.kind, kind)
			require.Equal(t, tc.body, string(payload))
			_, payload, err = conn.ReadFrame(context.Background())
			require.NoError(t, err)
			require.JSONEq(t, openAIServiceBusyFailedEvent, string(payload))
		})
	}
}

func TestOpenAIWSCapacityBufferStopsAtMetadataLimit(t *testing.T) {
	upstream := newStagedPassthroughConn()
	defer upstream.Close()
	upstream.Send(`{"type":"response.created","response":{"metadata":"` + strings.Repeat("x", openAIFirstOutputStageMaxBytes) + `"}}`)
	conn := &openAIWSCapacityFrameConn{FrameConn: upstream}
	_, payload, err := conn.ReadFrame(context.Background())
	require.ErrorContains(t, err, "staging limit exceeded")
	require.Empty(t, payload)
}
