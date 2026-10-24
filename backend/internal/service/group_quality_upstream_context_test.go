package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupQualityFallbackUpstreamContextPreservesCancellation(t *testing.T) {
	for _, helper := range []struct {
		name   string
		detach func(context.Context) (context.Context, context.CancelFunc)
	}{
		{"all upstream", detachUpstreamContext},
		{"stream upstream", func(ctx context.Context) (context.Context, context.CancelFunc) {
			return detachStreamUpstreamContext(ctx, true)
		}},
	} {
		t.Run(helper.name, func(t *testing.T) {
			for _, active := range []bool{false, true} {
				name := "ordinary budget request"
				if active {
					name = "reserved emergency fallback"
				}
				t.Run(name, func(t *testing.T) {
					type metadataKey struct{}
					base := context.WithValue(context.Background(), metadataKey{}, "request-value")
					ctx := GroupQualityTrafficContext(base, GroupQualityTrafficRequest{Key: "request", GroupID: 8})
					bindGroupQualityTrafficLease(ctx, nil, 7, "public", "lease")
					if active {
						bindGroupQualityFallbackStop(ctx, "lease", func() {})
					}
					upstream, release := helper.detach(ctx)
					release()
					require.NoError(t, upstream.Err(), "request-builder cleanup must not terminate the upstream")
					require.Equal(t, "request-value", upstream.Value(metadataKey{}))
					cancelGroupQualityTraffic(ctx, ErrGroupQualityFallbackLeaseLost)
					if active {
						require.ErrorIs(t, upstream.Err(), context.Canceled)
						require.ErrorIs(t, context.Cause(upstream), ErrGroupQualityFallbackLeaseLost)
					} else {
						require.NoError(t, upstream.Err(), "ordinary traffic still drains for usage billing")
						require.NoError(t, context.Cause(upstream))
					}
				})
			}
		})
	}
}

func TestGroupQualityFallbackWSClientContextPreservesCancellation(t *testing.T) {
	for _, helper := range []struct {
		name string
		make func(context.Context) (context.Context, context.CancelFunc)
	}{
		{"passthrough", newOpenAIWSClientUpstreamContext},
		{"committed turn", newOpenAIWSCommittedTurnContext},
	} {
		for _, cause := range []error{ErrGroupQualityFallbackLeaseLost, errOpenAIWSClientSessionDisconnected} {
			for _, timing := range []string{"first turn", "later turn"} {
				t.Run(helper.name+"/"+cause.Error()+"/"+timing, func(t *testing.T) {
					parent := context.Background()
					base := context.WithValue(parent, openAIWSClientSessionKey{}, &openAIWSClientSession{parent: parent})
					ctx := GroupQualityTrafficContext(base, GroupQualityTrafficRequest{Key: "request", GroupID: 8})
					bindGroupQualityTrafficLease(ctx, nil, 7, "public", "lease")
					if timing == "first turn" {
						bindGroupQualityFallbackStop(ctx, "lease", func() {})
					}
					upstream, release := helper.make(ctx)
					defer release()
					if timing == "later turn" {
						bindGroupQualityFallbackStop(ctx, "lease", func() {})
					}
					cancelGroupQualityTraffic(ctx, cause)
					require.Eventually(t, func() bool { return upstream.Err() != nil }, 200*time.Millisecond, time.Millisecond,
						"fallback traffic must stop before its released lease can admit another request")
					require.ErrorIs(t, context.Cause(upstream), cause)
					require.NoError(t, parent.Err(), "the session parent outlives the canceled fallback attempt")
				})
			}
		}
	}
}

func TestOrdinaryWSClientContextStillDrainsAfterDisconnect(t *testing.T) {
	parent := context.Background()
	base := context.WithValue(parent, openAIWSClientSessionKey{}, &openAIWSClientSession{parent: parent})
	ctx := GroupQualityTrafficContext(base, GroupQualityTrafficRequest{Key: "request", GroupID: 8})
	upstream, release := newOpenAIWSClientUpstreamContext(ctx)
	defer release()
	cancelGroupQualityTraffic(ctx, errOpenAIWSClientSessionDisconnected)
	require.Never(t, func() bool { return upstream.Err() != nil }, 20*time.Millisecond, time.Millisecond,
		"ordinary traffic retains the existing bounded usage-drain behavior")
}

func TestGroupQualityFallbackHTTPLeaseLossIsNotUpstreamFailure(t *testing.T) {
	ctx := GroupQualityTrafficContext(context.Background(), GroupQualityTrafficRequest{Key: "request", GroupID: 8})
	defer cancelGroupQualityTraffic(ctx, context.Canceled)
	bindGroupQualityTrafficLease(ctx, nil, 7, "public", "lease")
	bindGroupQualityFallbackStop(ctx, "lease", func() {})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancelGroupQualityTraffic(ctx, ErrGroupQualityFallbackLeaseLost)
		<-r.Context().Done()
	}))
	defer upstream.Close()
	upstreamCtx, release := detachUpstreamContext(ctx)
	release()
	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodGet, upstream.URL, nil)
	require.NoError(t, err)
	client := upstream.Client()
	client.Timeout = time.Second
	resp, transportErr := client.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.ErrorIs(t, transportErr, ErrGroupQualityFallbackLeaseLost, "real HTTP transport exposes the local cancellation cause")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	err = (&OpenAIGatewayService{}).handleOpenAIUpstreamTransportError(ctx, c, &Account{ID: 7, Platform: PlatformOpenAI}, transportErr, false)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, ErrGroupQualityFallbackLeaseLost)
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover))
	require.Empty(t, c.GetString(OpsUpstreamErrorMessageKey))
	_, hasEvents := c.Get(OpsUpstreamErrorsKey)
	require.False(t, hasEvents)
}
