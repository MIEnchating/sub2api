//go:build bps_request_integration

package service

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type requestProxyAccountTestRepo struct {
	AccountRepository
	account *Account
}
type requestProxyAccountTestUpstream struct{ HTTPUpstream }

func (u *requestProxyAccountTestUpstream) DoWithTLS(r *http.Request, p string, id int64, n int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.HTTPUpstream.Do(r, p, id, n)
}

func (r *requestProxyAccountTestRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}
func (r *requestProxyAccountTestRepo) UpdateExtra(context.Context, int64, map[string]any) error {
	return nil
}
