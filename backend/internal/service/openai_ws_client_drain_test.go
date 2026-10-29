package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOrdinaryWSClientContextStillDrainsAfterDisconnect(t *testing.T) {
	parent := context.Background()
	base := context.WithValue(parent, openAIWSClientSessionKey{}, &openAIWSClientSession{parent: parent})
	ctx, cancel := context.WithCancelCause(base)
	upstream, release := newOpenAIWSClientUpstreamContext(ctx)
	defer release()
	cancel(errOpenAIWSClientSessionDisconnected)
	require.Never(t, func() bool { return upstream.Err() != nil }, 20*time.Millisecond, time.Millisecond,
		"ordinary traffic retains the existing bounded usage-drain behavior")
}
