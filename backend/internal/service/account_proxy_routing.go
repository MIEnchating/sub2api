package service

import (
	"context"
	"fmt"
	"time"
)

// WithProxyRoute copies the account snapshot. Shared scheduler snapshots must
// never be mutated by a request selecting a different outbound proxy.
func (a *Account) WithProxyRoute(proxyID int64) (*Account, error) {
	if proxyID == 0 {
		return a, nil
	}
	for _, p := range a.Proxies {
		if p != nil && p.ID == proxyID && p.IsActive() && !p.IsExpired(time.Now()) {
			copy := *a
			copy.ProxyID = &p.ID
			copy.Proxy = p
			copy.SelectedProxyID = proxyID
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("selected proxy %d is unavailable for account %d", proxyID, a.ID)
}

func (r *AcquireResult) RouteAccount(a *Account) (*Account, error) {
	if r == nil || !r.Acquired || r.ProxyID == 0 {
		return a, nil
	}
	account, err := a.WithProxyRoute(r.ProxyID)
	if err != nil && r.ReleaseFunc != nil {
		r.ReleaseFunc()
	}
	return account, err
}

// AcquireAccountRoute updates only the caller's local pointer after Redis has
// reserved the selected proxy. The release closure owns that exact reservation.
func (s *ConcurrencyService) AcquireAccountRoute(ctx context.Context, account **Account, limit int) (*AcquireResult, error) {
	if account == nil || *account == nil {
		return nil, fmt.Errorf("missing account for proxy routing")
	}
	a := *account
	// Adaptive protection is resolved here, before either the fixed-account or
	// proxy-pool admission path. This keeps one request on one slot source and
	// preserves the administrator's saved concurrency as the hard ceiling.
	limit = EffectiveAccountConcurrency(a, limit)
	if a.RequiresProxyRetirementReview() || len(a.ProxyIDs) > 1 {
		return nil, ErrAccountMultiProxyRetired
	}
	result, err := s.AcquireAccountSlot(ctx, a.ID, limit)
	if err != nil || result == nil || !result.Acquired {
		return result, err
	}
	routed, err := result.RouteAccount(a)
	if err != nil {
		return nil, err
	}
	*account = routed
	return result, nil
}

// TotalConcurrency is shared by scheduling and wait capacity calculations.
func (a *Account) TotalConcurrency() int {
	if a == nil {
		return 0
	}
	return a.Concurrency
}

func (a *Account) TotalLoadFactor() int {
	if a == nil {
		return 1
	}
	return a.EffectiveLoadFactor()
}

// Reject pre-retirement snapshots until the account is rehydrated.
func validateProxyReservation(selected, hydrated *Account) error {
	if len(selected.ProxyIDs) > 1 || len(hydrated.ProxyIDs) > 1 || hydrated.RequiresProxyRetirementReview() {
		return ErrAccountMultiProxyRetired
	}
	return nil
}
