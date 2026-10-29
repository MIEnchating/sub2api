package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type riskRoutingAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r riskRoutingAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (r riskRoutingAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]Account, error) {
	return r.list(groupID, []string{platform}), nil
}

func (r riskRoutingAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(_ context.Context, groupID int64, platforms []string) ([]Account, error) {
	return r.list(groupID, platforms), nil
}

func (r riskRoutingAccountRepoStub) list(groupID int64, platforms []string) []Account {
	allowedPlatforms := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		allowedPlatforms[platform] = struct{}{}
	}
	var result []Account
	for _, account := range r.accounts {
		if _, ok := allowedPlatforms[account.Platform]; !ok || !account.IsSchedulable() {
			continue
		}
		for _, accountGroup := range account.AccountGroups {
			if accountGroup.GroupID == groupID {
				result = append(result, account)
				break
			}
		}
	}
	return result
}

type riskRoutingGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (r riskRoutingGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if group := r.groups[id]; group != nil {
		clone := *group
		return &clone, nil
	}
	return nil, ErrGroupNotFound
}

func (r riskRoutingGroupRepoStub) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	return r.GetByID(ctx, id)
}

func TestContentModerationConfigDefaultsToBlockingRiskHits(t *testing.T) {
	cfg, err := parseContentModerationConfig(`{"enabled":true}`)
	require.NoError(t, err)
	require.Equal(t, RiskHitActionBlock, cfg.HitAction)
	require.Nil(t, cfg.RouteGroupID)
	require.Nil(t, cfg.RouteAccountID)
}

func TestOpenAIRiskRoutingOverridesGroupAndAccountPriority(t *testing.T) {
	groupOne, groupTwo := int64(1), int64(2)
	repo := riskRoutingAccountRepoStub{accounts: []Account{
		{ID: 11, Name: "normal", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, AccountGroups: []AccountGroup{{GroupID: groupOne}}},
		{ID: 22, Name: "risk", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 99, AccountGroups: []AccountGroup{{GroupID: groupTwo}}},
	}}
	svc := &OpenAIGatewayService{accountRepo: repo}

	groupCtx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteGroup, GroupID: groupTwo})
	groupSelection, err := svc.SelectAccountWithLoadAwareness(groupCtx, &groupOne, "", "gpt-5.5", nil)
	require.NoError(t, err)
	require.Equal(t, int64(22), groupSelection.Account.ID)

	accountCtx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteAccount, AccountID: 22})
	accountSelection, err := svc.SelectAccountWithLoadAwareness(accountCtx, &groupOne, "", "gpt-5.5", nil)
	require.NoError(t, err)
	require.Equal(t, int64(22), accountSelection.Account.ID)

	_, err = svc.SelectAccountWithLoadAwareness(accountCtx, &groupOne, "", "gpt-5.5", map[int64]struct{}{22: {}})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
}

func TestGatewayRiskRoutingOverridesGroupAndFailsClosedForAccount(t *testing.T) {
	groupOne, groupTwo := int64(1), int64(2)
	repo := riskRoutingAccountRepoStub{accounts: []Account{
		{ID: 31, Name: "normal", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0, AccountGroups: []AccountGroup{{GroupID: groupOne}}},
		{ID: 32, Name: "risk", Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 99, AccountGroups: []AccountGroup{{GroupID: groupTwo}}},
	}}
	groups := riskRoutingGroupRepoStub{groups: map[int64]*Group{
		groupOne: {ID: groupOne, Name: "normal", Platform: PlatformAnthropic},
		groupTwo: {ID: groupTwo, Name: "risk", Platform: PlatformAnthropic},
	}}
	svc := &GatewayService{accountRepo: repo, groupRepo: groups}

	groupCtx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteGroup, GroupID: groupTwo})
	groupSelection, err := svc.SelectAccountWithLoadAwareness(groupCtx, &groupOne, "", "claude-sonnet-4-6", nil, "", 0)
	require.NoError(t, err)
	require.Equal(t, int64(32), groupSelection.Account.ID)

	accountCtx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteAccount, AccountID: 32})
	accountSelection, err := svc.SelectAccountWithLoadAwareness(accountCtx, &groupOne, "", "claude-sonnet-4-6", nil, "", 0)
	require.NoError(t, err)
	require.Equal(t, int64(32), accountSelection.Account.ID)

	_, err = svc.SelectAccountWithLoadAwareness(accountCtx, &groupOne, "", "claude-sonnet-4-6", map[int64]struct{}{32: {}}, "", 0)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
}

func TestOpenAIRiskRoutingSchedulerPreservesTargetAndBilling(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_nonbatch", "advanced"} {
		for _, action := range []string{RiskHitActionRouteAccount, RiskHitActionRouteGroup} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
				primary, target := int64(43), int64(44)
				gate := &activeHoldTestGate{normal: map[int64]bool{1: true, 2: true}}
				svc := activeHoldService(mode, []Account{
					activeRoutingTestAccount(1, primary, -100),
					activeRoutingTestAccount(2, target, 100),
				}, gate)
				key := &APIKey{GroupID: &primary, Group: &Group{ID: primary, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}}
				ctx := WithAPIKeyForUsageContext(context.Background(), key)
				ctx = WithRiskRoutingTarget(ctx, RiskRoutingTarget{Action: action, AccountID: 2, GroupID: target, Platform: PlatformOpenAI})
				selection, decision, err := svc.SelectAccountWithScheduler(ctx, key.GroupID, "", "", "public-model", nil, OpenAIUpstreamTransportHTTPSSE, false)
				require.NoError(t, err)
				require.NotNil(t, selection)
				t.Cleanup(selection.ReleaseFunc)
				require.EqualValues(t, 2, selection.Account.ID, "moderation's target overrides ordinary priority and group selection")
				require.EqualValues(t, 2, decision.SelectedAccountID)
				require.False(t, selection.UsedAPIKeyFallback)
				require.EqualValues(t, 43, *key.GroupID, "moderation routing does not change billing authorization")
			})
		}
	}
}

func TestOpenAIRiskRoutingSchedulerCannotRecoverThroughAnotherAccount(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_nonbatch", "advanced"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
			groupID := int64(43)
			gate := &activeHoldTestGate{normal: map[int64]bool{2: true}}
			svc := activeHoldService(mode, []Account{
				activeRoutingTestAccount(1, groupID, 0),
				activeRoutingTestAccount(2, groupID, -100),
			}, gate)
			ctx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteAccount, AccountID: 1})
			selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "public-model", nil, OpenAIUpstreamTransportHTTPSSE, false)
			require.ErrorIs(t, err, ErrAccountQualityHeld)
			require.Nil(t, selection)
			require.NotContains(t, gate.groups, int64(44), "admission must not switch the authorized group")
		})
	}
}

func TestOpenAIRiskRoutingSchedulerPreservesResponseOwnership(t *testing.T) {
	for _, owner := range []int64{0, 1, 2} {
		t.Run(map[int64]string{0: "unknown", 1: "different account", 2: "same account"}[owner], func(t *testing.T) {
			groupID := int64(43)
			gate := &activeHoldTestGate{normal: map[int64]bool{1: true, 2: true}}
			svc := activeHoldService("legacy", []Account{
				activeRoutingTestAccount(1, groupID, 0),
				activeRoutingTestAccount(2, groupID, 0),
			}, gate)
			svc.openaiWSStateStore = NewOpenAIWSStateStore(nil)
			ctx := WithRiskRoutingTarget(context.Background(), RiskRoutingTarget{Action: RiskHitActionRouteAccount, AccountID: 2})
			if owner != 0 {
				require.NoError(t, svc.openaiWSStateStore.BindResponseAccount(ctx, groupID, "resp_owner", owner, time.Hour))
			}
			selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "resp_owner", "", "public-model", nil, OpenAIUpstreamTransportHTTPSSE, false)
			if owner == 2 {
				require.NoError(t, err)
				require.NotNil(t, selection)
				t.Cleanup(selection.ReleaseFunc)
				require.EqualValues(t, 2, selection.Account.ID)
			} else {
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				require.Nil(t, selection)
				require.Empty(t, gate.groups, "reject foreign or unknown response ownership before admission")
			}
		})
	}
}
