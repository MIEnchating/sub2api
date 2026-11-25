package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAstraAutomaticSetupShutdownCancelsAndJoins(t *testing.T) {
	gateway, repo, _ := unifiedAstraFixture(t)
	started := make(chan struct{})
	gateway.httpUpstream = &harvestProxyUpstream{do: func(req *http.Request, _ string) (*http.Response, error) {
		close(started)
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	service := &AccountTestService{cfg: gateway.cfg, accountRepo: repo, openaiGatewayService: gateway}
	settings := gateway.cfg.AstraRouting(context.Background())
	service.StartAstraAutomaticSetup(settings)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("setup did not start")
	}
	stopped := make(chan struct{})
	go func() { service.StopAstraAutomaticSetup(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not join canceled setup")
	}
	require.True(t, service.astraSetupStopped)
	require.Equal(t, "failed", service.astraSetupStatus.State)
	service.StartAstraAutomaticSetup(settings)
	require.Equal(t, "failed", service.astraSetupStatus.State, "shutdown prevents new background requests")
	service.StopAstraAutomaticSetup()
}

type astraSetupUpstream struct {
	HTTPUpstream
	AstraTicketLifecycle
	prepare  func(context.Context) error
	snapshot AstraGatewayRuntime
}

func (s *astraSetupUpstream) PrepareAstraGateway(ctx context.Context) error              { return s.prepare(ctx) }
func (s *astraSetupUpstream) SetAstraGatewayPreparer(func(context.Context, int64) error) {}
func (s *astraSetupUpstream) AstraGatewaySnapshot(context.Context) AstraGatewayRuntime {
	return s.snapshot
}
func (s *astraSetupUpstream) PrepareStoredAstraSources(ctx context.Context) error {
	return s.prepare(ctx)
}

func (s *astraSetupUpstream) StoredAstraSnapshot(context.Context) AstraGatewayRuntime {
	return s.snapshot
}

func TestAstraSetupRequiresAllTargetsOnCurrentRoutes(t *testing.T) {
	provider := &astraSetupUpstream{snapshot: AstraGatewayRuntime{Targets: []AstraRouteStatus{{AccountID: 300, State: "waiting"}, {AccountID: 301, State: "ready"}}}}
	id, ready := astraTargetsReady(t.Context(), provider, []int64{300, 301})
	require.False(t, ready)
	require.Equal(t, int64(300), id)
	provider.snapshot.Targets[0].State = "ready"
	_, ready = astraTargetsReady(t.Context(), provider, []int64{300, 301})
	require.True(t, ready)
}

func TestAstraSetupWaitsForConcurrentPreparation(t *testing.T) {
	calls := 0
	provider := &astraSetupUpstream{prepare: func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("preparation_in_progress")
		}
		return nil
	}}
	require.NoError(t, prepareAstraForSetup(t.Context(), provider))
	require.Equal(t, 2, calls)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, prepareAstraForSetup(ctx, provider), context.Canceled)
	require.Equal(t, 2, calls)
}
