package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountEgressCacheStub struct {
	mu      sync.Mutex
	values  map[string]int64
	lastTTL time.Duration
}

func (c *accountEgressCacheStub) key(groupID int64, sessionHash string) string {
	return fmt.Sprintf("%d:%s", groupID, sessionHash)
}

func (c *accountEgressCacheStub) GetSessionAccountID(_ context.Context, groupID int64, sessionHash string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[c.key(groupID, sessionHash)]
	if !ok {
		return 0, ErrStickySessionNotFound
	}
	return value, nil
}

func (c *accountEgressCacheStub) SetSessionAccountID(_ context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[string]int64)
	}
	c.values[c.key(groupID, sessionHash)] = accountID
	c.lastTTL = ttl
	return nil
}

func (c *accountEgressCacheStub) RefreshSessionTTL(_ context.Context, _ int64, _ string, ttl time.Duration) error {
	c.mu.Lock()
	c.lastTTL = ttl
	c.mu.Unlock()
	return nil
}

func (c *accountEgressCacheStub) DeleteSessionAccountID(_ context.Context, groupID int64, sessionHash string) error {
	c.mu.Lock()
	delete(c.values, c.key(groupID, sessionHash))
	c.mu.Unlock()
	return nil
}

func TestParseAccountProxyPoolIDsNormalizesValues(t *testing.T) {
	extra := map[string]any{
		AccountProxyPoolExtraKey: []any{float64(2), float64(2), int64(3), float64(-1), "4", float64(5.5)},
	}
	require.Equal(t, []int64{2, 3}, ParseAccountProxyPoolIDs(extra))
}

func TestValidateAccountProxyPoolRequiresPrimaryAndLimitsSize(t *testing.T) {
	_, err := validateAccountProxyPool(nil, []int64{2})
	require.Error(t, err)

	primaryID := int64(1)
	ids := make([]int64, MaxAccountProxyPoolSize+1)
	for i := range ids {
		ids[i] = int64(i + 2)
	}
	_, err = validateAccountProxyPool(&primaryID, ids)
	require.Error(t, err)

	normalized, err := validateAccountProxyPool(&primaryID, []int64{1, 2, 2, 3})
	require.NoError(t, err)
	require.Equal(t, []int64{2, 3}, normalized)
}

func TestSelectAccountEgressProxyRoundRobinAndRequestStability(t *testing.T) {
	accountEgressSequences.Delete(int64(10))
	primary := &Proxy{ID: 1, Status: StatusActive}
	second := &Proxy{ID: 2, Status: StatusActive}
	third := &Proxy{ID: 3, Status: StatusActive}
	account := &Account{
		ID:        10,
		ProxyID:   &primary.ID,
		Proxy:     primary,
		ProxyIDs:  []int64{2, 3},
		ProxyPool: []*Proxy{second, third},
	}

	first := selectAccountEgressProxy(context.Background(), nil, account)
	secondRequest := selectAccountEgressProxy(context.Background(), nil, account)
	thirdRequest := selectAccountEgressProxy(context.Background(), nil, account)
	require.Equal(t, int64(1), first.Proxy.ID)
	require.Equal(t, int64(2), secondRequest.Proxy.ID)
	require.Equal(t, int64(3), thirdRequest.Proxy.ID)
	require.Same(t, secondRequest, selectAccountEgressProxy(context.Background(), nil, secondRequest))
	require.Equal(t, int64(1), *secondRequest.ConfiguredProxyID())
	require.Equal(t, int64(1), account.Proxy.ID, "scheduler account must not be mutated")
}

func TestSelectAccountEgressProxySkipsUnavailableAdditionalProxies(t *testing.T) {
	accountEgressSequences.Delete(int64(11))
	primary := &Proxy{ID: 1, Status: StatusActive}
	inactive := &Proxy{ID: 2, Status: StatusDisabled}
	expiredAt := time.Now().Add(-time.Minute)
	expired := &Proxy{ID: 3, Status: StatusActive, ExpiresAt: &expiredAt}
	active := &Proxy{ID: 4, Status: StatusActive}
	account := &Account{ID: 11, Proxy: primary, ProxyPool: []*Proxy{inactive, expired, active}}

	require.Equal(t, int64(1), selectAccountEgressProxy(context.Background(), nil, account).Proxy.ID)
	require.Equal(t, int64(4), selectAccountEgressProxy(context.Background(), nil, account).Proxy.ID)
}

func TestSelectAccountEgressProxyKeepsIndependentAccountSequences(t *testing.T) {
	accountEgressSequences.Delete(int64(21))
	accountEgressSequences.Delete(int64(22))
	primaryA := &Proxy{ID: 1, Status: StatusActive}
	secondaryA := &Proxy{ID: 2, Status: StatusActive}
	primaryB := &Proxy{ID: 3, Status: StatusActive}
	secondaryB := &Proxy{ID: 4, Status: StatusActive}
	accountA := &Account{ID: 21, Proxy: primaryA, ProxyPool: []*Proxy{secondaryA}}
	accountB := &Account{ID: 22, Proxy: primaryB, ProxyPool: []*Proxy{secondaryB}}

	require.Equal(t, int64(1), selectAccountEgressProxy(context.Background(), nil, accountA).Proxy.ID)
	require.Equal(t, int64(3), selectAccountEgressProxy(context.Background(), nil, accountB).Proxy.ID)
	require.Equal(t, int64(2), selectAccountEgressProxy(context.Background(), nil, accountA).Proxy.ID)
	require.Equal(t, int64(4), selectAccountEgressProxy(context.Background(), nil, accountB).Proxy.ID)
}

func TestSelectAccountEgressProxyKeepsSessionSticky(t *testing.T) {
	primary := &Proxy{ID: 1, Status: StatusActive}
	secondary := &Proxy{ID: 2, Status: StatusActive}
	account := &Account{
		ID: 31, Proxy: primary, ProxyPool: []*Proxy{secondary},
		Extra: map[string]any{AccountProxyStickyTTLSecondsExtraKey: float64(600)},
	}
	cache := &accountEgressCacheStub{}
	ctx := withAccountEgressSessionHash(context.Background(), "conversation-a")

	first := selectAccountEgressProxy(ctx, cache, account)
	second := selectAccountEgressProxy(ctx, cache, account)

	require.Equal(t, first.Proxy.ID, second.Proxy.ID)
	require.Equal(t, 10*time.Minute, cache.lastTTL)
}

func TestSelectAccountEgressProxyRebindsUnavailableStickyProxy(t *testing.T) {
	primary := &Proxy{ID: 1, Status: StatusActive}
	secondary := &Proxy{ID: 2, Status: StatusActive}
	account := &Account{ID: 32, Proxy: primary, ProxyPool: []*Proxy{secondary}}
	ctx := withAccountEgressSessionHash(context.Background(), "conversation-b")
	cacheKey := accountProxyStickyCacheKey("conversation-b")
	cache := &accountEgressCacheStub{values: map[string]int64{fmt.Sprintf("%d:%s", account.ID, cacheKey): 999}}

	selected := selectAccountEgressProxy(ctx, cache, account)
	bound, err := cache.GetSessionAccountID(context.Background(), account.ID, cacheKey)

	require.NoError(t, err)
	require.Equal(t, selected.Proxy.ID, bound)
}

func TestSelectAccountEgressProxyModes(t *testing.T) {
	accountEgressSequences.Delete(int64(33))
	primary := &Proxy{ID: 1, Status: StatusActive}
	secondary := &Proxy{ID: 2, Status: StatusActive}
	account := &Account{ID: 33, Proxy: primary, ProxyPool: []*Proxy{secondary}, Extra: map[string]any{}}

	account.Extra[AccountProxyEgressModeExtraKey] = AccountProxyEgressModePrimary
	require.Equal(t, primary.ID, selectAccountEgressProxy(context.Background(), nil, account).Proxy.ID)
	require.Equal(t, primary.ID, selectAccountEgressProxy(context.Background(), nil, account).Proxy.ID)

	account.Extra[AccountProxyEgressModeExtraKey] = AccountProxyEgressModeRoundRobin
	require.Equal(t, primary.ID, selectAccountEgressProxy(context.Background(), nil, account).Proxy.ID)
	require.Equal(t, secondary.ID, selectAccountEgressProxy(context.Background(), nil, account).Proxy.ID)
}
