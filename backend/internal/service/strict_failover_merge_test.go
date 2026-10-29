package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStrictFailoverPreservesRiskPinAndBilling(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_nonbatch", "advanced"} {
		for _, target := range []int64{1, 3} {
			t.Run(mode+string(rune('0'+target)), func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
				g, repo := strictFixture()
				svc := &OpenAIGatewayService{accountRepo: repo, cache: &schedulerTestGatewayCache{}, cfg: &config.Config{}}
				svc.cfg.Gateway.Scheduling.LoadBatchEnabled = mode != "legacy_nonbatch"
				if mode == "advanced" {
					svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true", "true")
				}
				key := &APIKey{GroupID: &g.ID, Group: g}
				ctx := WithAPIKeyForUsageContext(strictContext(g), key)
				ctx = WithRiskRoutingTarget(ctx, RiskRoutingTarget{Action: RiskHitActionRouteAccount, AccountID: target})
				selection, _, err := svc.SelectAccountWithScheduler(ctx, &g.ID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportHTTPSSE, false)
				if target == 1 {
					require.NoError(t, err)
					require.NotNil(t, selection)
					require.Equal(t, target, selection.Account.ID)
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
				} else {
					require.Error(t, err)
					require.Nil(t, selection, "a risk pin outside the active tier must fail closed, never select a peer")
				}
				require.Equal(t, int64(75), *key.GroupID)
			})
		}
	}
}

type mergeAdmissionErrorGate struct{ err error }

func (g mergeAdmissionErrorGate) CheckAccountAdmission(context.Context, int64) (bool, error) {
	return false, g.err
}

func TestRiskPinAdmissionFailureNeverBecomesWaitPlan(t *testing.T) {
	for _, denied := range []error{ErrAccountQualityHeld, ErrAccountAdmissionDenied, errors.New("admission database unavailable")} {
		svc := activeRoutingTestService("legacy", []Account{activeRoutingTestAccount(1, 43, 0)}, &schedulerTestGatewayCache{}, schedulerTestConcurrencyCache{})
		svc.concurrencyService.SetAccountAdmissionGate(mergeAdmissionErrorGate{denied})
		group := int64(43)
		selection, err := svc.selectRiskRoutedOpenAIAccountWithSlot(context.Background(), 1, &group, PlatformOpenAI, "", nil, false, "", "", OpenAIUpstreamTransportAny)
		require.ErrorIs(t, err, denied)
		require.Nil(t, selection)
	}
}

func TestGatewayRiskPinAdmissionFailureNeverBecomesWaitPlan(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, denied := range []error{ErrAccountQualityHeld, ErrAccountAdmissionDenied, errors.New("admission database unavailable")} {
			groupID := int64(43)
			account := activeRoutingTestAccount(1, groupID, 0)
			account.Platform = PlatformAnthropic
			account.AccountGroups = []AccountGroup{{GroupID: groupID}}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = batch
			svc := &GatewayService{
				accountRepo: riskRoutingAccountRepoStub{accounts: []Account{account}},
				groupRepo: riskRoutingGroupRepoStub{groups: map[int64]*Group{
					groupID: {ID: groupID, Platform: PlatformAnthropic},
				}},
				cfg: cfg, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			svc.concurrencyService.SetAccountAdmissionGate(mergeAdmissionErrorGate{denied})
			ctx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteAccount, AccountID: account.ID})
			selection, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "", nil, "", 0)
			require.ErrorIs(t, err, denied, "batch=%v", batch)
			require.Nil(t, selection, "admission failure must not become a concurrency wait")
		}
	}
}
