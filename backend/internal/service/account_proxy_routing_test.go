package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func proxyRoutingAccount() *Account {
	first := int64(11)
	return &Account{ID: 7, ProxyID: &first, Concurrency: 2, ProxyIDs: []int64{11, 22}, Proxies: []*Proxy{{ID: 11, Status: StatusActive}, {ID: 22, Status: StatusActive}}}
}
func TestProxyPoolUnavailableNeverFallsBackToDirect(t *testing.T) {
	cache := &proxyLegacyCache{acquireResult: true}
	account := proxyRoutingAccount()
	original := account
	_, err := NewConcurrencyService(cache).AcquireAccountRoute(context.Background(), &account, 2)
	require.ErrorIs(t, err, ErrAccountMultiProxyRetired)
	require.Same(t, original, account)
	account.Proxies = nil
	_, err = NewConcurrencyService(cache).AcquireAccountRoute(context.Background(), &account, 2)
	require.ErrorIs(t, err, ErrAccountMultiProxyRetired)
}
func TestFixedProxyKeepsAccountSlotAcquisition(t *testing.T) {
	cache := &proxyLegacyCache{acquireResult: true}
	account := proxyRoutingAccount()
	account.ProxyIDs = []int64{11}
	original := account
	result, err := NewConcurrencyService(cache).AcquireAccountRoute(context.Background(), &account, 2)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Same(t, original, account)
	result.ReleaseFunc()
	require.Equal(t, []int64{7}, cache.releasedAccountIDs)
}
func TestRetiredProxyConfigurationCannotScheduleWithoutReview(t *testing.T) {
	account := &Account{ID: 7, Status: StatusActive, Schedulable: true, Extra: map[string]any{RetiredMultiProxyNeedsReviewKey: true}}
	require.False(t, account.IsSchedulable())
	_, err := NewConcurrencyService(&proxyLegacyCache{acquireResult: true}).AcquireAccountRoute(context.Background(), &account, 1)
	require.ErrorIs(t, err, ErrAccountMultiProxyRetired)
}

func TestRetiredRequestProxyConfigurationCannotBeSaved(t *testing.T) {
	ctx := context.Background()
	svc := &adminServiceImpl{}
	for _, extra := range []map[string]any{
		{OpenAIProxyModeKey: OpenAIProxyModeRequest, OpenAIProxyIDsKey: []int64{11, 12}},
		{ExcelBPSProxyModeKey: ExcelBPSProxyModeRequest, ExcelBPSProxyIDsKey: []int64{11}},
	} {
		_, err := svc.CreateAccount(ctx, &CreateAccountInput{Extra: extra})
		require.ErrorIs(t, err, ErrAccountMultiProxyRetired)
		_, err = svc.UpdateAccount(ctx, 7, &UpdateAccountInput{Extra: extra})
		require.ErrorIs(t, err, ErrAccountMultiProxyRetired)
		_, err = svc.BulkUpdateAccounts(ctx, &BulkUpdateAccountsInput{Extra: extra})
		require.ErrorIs(t, err, ErrAccountMultiProxyRetired)
		require.ErrorIs(t, svc.UpdateAccountExtra(ctx, 7, extra), ErrAccountMultiProxyRetired)
	}
}

type proxyLegacyCache struct {
	ConcurrencyCache
	acquireResult      bool
	releasedAccountIDs []int64
}

func (c *proxyLegacyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return c.acquireResult, nil
}
func (c *proxyLegacyCache) ReleaseAccountSlot(_ context.Context, id int64, _ string) error {
	c.releasedAccountIDs = append(c.releasedAccountIDs, id)
	return nil
}
