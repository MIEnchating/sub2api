//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLegacyCodexTicketHonorsLiveRefreshStrategy(t *testing.T) {
	for _, tc := range []struct {
		name, strategy string
		remaining      time.Duration
		cookies, probe bool
	}{
		{"fixed keeps valid ticket", "fixed", 10 * time.Second, true, false},
		{"standby prepares replacement", "standby", 10 * time.Second, true, true},
		{"fixed refreshes expired ticket", "fixed", -time.Second, true, true},
		{"fixed refreshes missing cookies", "fixed", 10 * time.Second, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}, RefreshBeforeSeconds: 30}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: http.NoBody}, nil
			}})
			strategy := "standby"
			svc.settingService = NewSettingService(&codexTicketLifecycleSettings{get: func(_ context.Context, key string) (string, error) {
				if key == SettingKeyOpenAICodexTicketStrategy {
					return strategy, nil
				}
				return "", ErrSettingNotFound
			}}, svc.cfg)
			account := activeTicketAccounts(1)[0]
			account.Extra = map[string]any{OpenAICodexTicketEnabledExtraKey: true}
			if tc.cookies {
				seedCodexTicketCookies(svc, &account)
			}
			now := time.Now()
			require.NoError(t, svc.storeOpenAICodexTicket(context.Background(), &account, &openAICodexTicket{
				Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292,
				CapturedAt: now.Add(-time.Minute), ExpiresAt: now.Add(tc.remaining),
			}))
			cfg := svc.openAICodexTicketHarvestConfig(context.Background())
			strategy = tc.strategy // Change after the dispatcher captured its config.
			started := svc.startOpenAICodexTicketProbe(context.Background(), &account, "gpt-6-astra", cfg, now, false)
			require.Equal(t, tc.probe, started)
			if tc.probe {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls, "fixed strategy must not spend an upstream request before expiry")
			}
		})
	}
}

func TestConfiguredHarvestBudgetAppliesToLegacyAccountPolicy(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, exhausted := range []bool{true, false} {
			t.Run(fmt.Sprintf("enabled=%t/exhausted=%t", enabled, exhausted), func(t *testing.T) {
				calls := 0
				svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}, HarvestProxyURL: "http://proxy.example:8080"}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: http.NoBody}, nil
				}})
				repo := &harvestControlSettingsRepo{}
				svc.settingService = NewSettingService(repo, svc.cfg)
				svc.codexHarvest = NewCodexHarvestService(nil, repo, svc.cfg)
				controls, _, err := svc.codexHarvest.Controls(context.Background())
				require.NoError(t, err)
				controls.Speed = CodexHarvestSpeedPresets()["burst"]
				require.NoError(t, svc.codexHarvest.SaveControls(context.Background(), controls))
				account := activeTicketAccounts(1)[0]
				account.Extra = map[string]any{OpenAICodexTicketEnabledExtraKey: enabled, OpenAICodexTicketFailClosedExtraKey: true}
				svc.accountRepo = &harvestFreshRepo{account: account}
				round := &codexHarvestRound{limit: 1}
				if exhausted {
					require.True(t, round.Take(1))
				}
				ctx := context.WithValue(context.Background(), codexHarvestRoundKey{}, round)
				svc.probeOnceOpenAICodexTicket(ctx, &account, "gpt-6-astra")
				if enabled && !exhausted {
					require.Equal(t, 1, calls)
					require.Equal(t, 1, round.Used(), "legacy account switches must not bypass the saved harvest budget")
				} else {
					require.Zero(t, calls)
				}
				require.True(t, svc.openAICodexTicketAccountConfig(ctx, &account).FailClosed)
			})
		}
	}
}

func TestConfiguredHarvestPreservesAccountExit(t *testing.T) {
	for _, route := range []string{"direct", "account", "invalid"} {
		t.Run(route, func(t *testing.T) {
			calls := 0
			wantProxy := ""
			account := activeTicketAccounts(1)[0]
			account.Extra = map[string]any{OpenAICodexTicketEnabledExtraKey: true}
			if route != "direct" {
				id := int64(10)
				account.ProxyID = &id
				if route == "account" {
					account.Proxy = &Proxy{ID: id, Protocol: "http", Host: "account.example", Port: 8080, Status: StatusActive}
					wantProxy = account.Proxy.URL()
				}
			}
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}}, &harvestProxyUpstream{do: func(_ *http.Request, proxy string) (*http.Response, error) {
				calls++
				require.Equal(t, wantProxy, proxy)
				return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: http.NoBody}, nil
			}})
			repo := &harvestControlSettingsRepo{}
			svc.settingService = NewSettingService(repo, svc.cfg)
			svc.codexHarvest = NewCodexHarvestService(nil, repo, svc.cfg)
			controls, _, err := svc.codexHarvest.Controls(context.Background())
			require.NoError(t, err)
			controls.Speed = CodexHarvestSpeedPresets()["burst"]
			require.NoError(t, svc.codexHarvest.SaveControls(context.Background(), controls))
			svc.accountRepo = &harvestFreshRepo{account: account}
			svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
			if route == "invalid" {
				require.Zero(t, calls, "an invalid assigned proxy must never fall back to direct")
			} else {
				require.Equal(t, 1, calls)
			}
		})
	}
}
