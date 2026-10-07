package service

import (
	"context"
	"io"
	"testing"
	"time"

	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	"github.com/stretchr/testify/require"
)

// Model an upstream that stops reading control frames while generating output.
// Canceling its read is independent of acknowledging a close handshake.
type stalledClosePassthroughConn struct {
	*stagedPassthroughConn
	handshakeAck chan struct{}
}

func (c *stalledClosePassthroughConn) Close() error {
	<-c.handshakeAck
	return c.stagedPassthroughConn.Close()
}

func (c *stalledClosePassthroughConn) CloseNow() error {
	return c.stagedPassthroughConn.Close()
}

func TestPassthroughShutdownDoesNotWaitForUpstreamCloseHandshake(t *testing.T) {
	for _, shutdown := range []bool{true, false} {
		name := "parent cancellation"
		if !shutdown {
			name = "client disconnect exhausts usage drain"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			upstream := &stalledClosePassthroughConn{
				stagedPassthroughConn: newStagedPassthroughConn(),
				handshakeAck:          make(chan struct{}),
			}
			defer close(upstream.handshakeAck)
			upstream.Send(`{"type":"response.output_text.delta","delta":"partial"}`)
			client := newStagedPassthroughConn()
			done := make(chan openaiwsv2.RelayResult, 1)
			go func() {
				result, _ := openaiwsv2.RunEntry(openaiwsv2.EntryInput{
					Ctx:          ctx,
					ClientConn:   client,
					UpstreamConn: &openAIWSPassthroughFirstOutputFrameConn{inner: upstream},
					Options: openaiwsv2.RelayOptions{
						FirstMessageSent:     true,
						UpstreamDrainTimeout: 25 * time.Millisecond,
					},
				})
				done <- result
			}()
			select {
			case <-client.writes:
			case <-time.After(time.Second):
				t.Fatal("turn did not emit its first output")
			}
			if shutdown {
				cancel()
			} else {
				client.Fail(io.EOF)
			}
			select {
			case result := <-done:
				require.Empty(t, result.TerminalEventType)
				require.Zero(t, result.Usage, "missing terminal evidence must not fabricate usage")
			case <-time.After(time.Second):
				t.Fatal("relay waited for an upstream close handshake after cancellation or usage drain")
			}
			select {
			case <-upstream.closed:
			default:
				t.Fatal("relay returned without closing its upstream transport")
			}
		})
	}
}
