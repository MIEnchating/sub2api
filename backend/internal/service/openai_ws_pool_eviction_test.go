package service

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A peer that stops reading cannot complete a graceful close handshake.
type openAIWSEvictionCloseProbe struct {
	openAIWSFakeConn
	gracefulStarted chan struct{}
	releaseGraceful chan struct{}
	forced          chan struct{}
	gracefulOnce    sync.Once
	forceOnce       sync.Once
}

func (c *openAIWSEvictionCloseProbe) Close() error {
	c.gracefulOnce.Do(func() { close(c.gracefulStarted) })
	<-c.releaseGraceful
	return c.openAIWSFakeConn.Close()
}

func (c *openAIWSEvictionCloseProbe) CloseNow() error {
	c.forceOnce.Do(func() { close(c.forced) })
	return c.openAIWSFakeConn.Close()
}

func TestOpenAIWSConnLeaseMarkBrokenDoesNotWaitForCloseHandshake(t *testing.T) {
	ws := &openAIWSEvictionCloseProbe{
		gracefulStarted: make(chan struct{}),
		releaseGraceful: make(chan struct{}),
		forced:          make(chan struct{}),
	}
	const accountID int64 = 5001
	conn := newOpenAIWSConn("broken_unresponsive_peer", accountID, ws, nil)
	require.True(t, conn.tryAcquire())
	ap := &openAIWSAccountPool{
		conns:       map[string]*openAIWSConn{conn.id: conn},
		pinnedConns: map[string]int{conn.id: 1},
	}
	pool := &openAIWSConnPool{}
	pool.accounts.Store(accountID, ap)
	changed := ap.changeChannelLocked()
	lease := &openAIWSConnLease{pool: pool, accountID: accountID, conn: conn, accountPool: ap}
	done := make(chan struct{})
	defer func() {
		close(ws.releaseGraceful)
		<-done
	}()
	go func() {
		lease.MarkBroken()
		close(done)
	}()
	select {
	case <-done:
	case <-ws.gracefulStarted:
		t.Fatal("broken transport entered a close handshake with an unresponsive peer")
	case <-time.After(time.Second):
		t.Fatal("broken transport eviction did not finish")
	}
	select {
	case <-ws.forced:
	default:
		t.Fatal("broken transport was not forcibly closed")
	}
	select {
	case <-changed:
	default:
		t.Fatal("eviction did not wake capacity waiters")
	}
	ap.mu.Lock()
	_, exists := ap.conns[conn.id]
	_, pinned := ap.pinnedConns[conn.id]
	ap.mu.Unlock()
	require.False(t, exists)
	require.False(t, pinned)
	require.False(t, conn.tryAcquire(), "evicted transport must not be reused")
	lease.Release()
	lease.MarkBroken()
}
