//go:build unit

package service

import (
	"context"
)

type bpsProxyConfigRepo struct {
	longContextBillingRepoStub
}

func (r *bpsProxyConfigRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	for key, value := range updates {
		r.account.Extra[key] = value
	}
	return nil
}

type bpsProxyConfigProxies struct {
	ProxyRepository
	proxies []Proxy
}

func (r *bpsProxyConfigProxies) GetByID(_ context.Context, id int64) (*Proxy, error) {
	for _, proxy := range r.proxies {
		if proxy.ID == id {
			return &proxy, nil
		}
	}
	return nil, ErrProxyNotFound
}

func (r *bpsProxyConfigProxies) ListByIDs(_ context.Context, ids []int64) ([]Proxy, error) {
	var proxies []Proxy
	for _, proxy := range r.proxies {
		for _, id := range ids {
			if proxy.ID == id {
				proxies = append(proxies, proxy)
				break
			}
		}
	}
	return proxies, nil
}

func newBPSProxyConfigService() (*adminServiceImpl, *bpsProxyConfigRepo) {
	repo := &bpsProxyConfigRepo{}
	return &adminServiceImpl{accountRepo: repo, proxyRepo: &bpsProxyConfigProxies{proxies: []Proxy{{ID: 11, Status: StatusActive}, {ID: 22, Status: StatusActive}}}}, repo
}
