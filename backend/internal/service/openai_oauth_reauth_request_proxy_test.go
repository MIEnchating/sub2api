package service

import (
	"context"
)

type reauthRequestProxyRepo struct {
	ProxyRepository
	proxies map[int64]*Proxy
}

func (r *reauthRequestProxyRepo) GetByID(_ context.Context, id int64) (*Proxy, error) {
	return r.proxies[id], nil
}
