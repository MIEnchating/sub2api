package service

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type codexModelsRequestProxyRepo struct {
	ProxyRepository
	proxies map[int64]*Proxy
}

func (r *codexModelsRequestProxyRepo) GetByID(_ context.Context, id int64) (*Proxy, error) {
	return r.proxies[id], nil
}

func codexModelsProxyRecord(t *testing.T, id int64, raw string) *Proxy {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	host, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	number, err := strconv.Atoi(port)
	require.NoError(t, err)
	return &Proxy{ID: id, Protocol: u.Scheme, Host: host, Port: number, Status: StatusActive}
}
