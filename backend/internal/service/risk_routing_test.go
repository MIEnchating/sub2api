package service

import (
	"context"
	"testing"

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
