package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountAdmissionGateFunc func(context.Context, int64) (bool, error)

func (f accountAdmissionGateFunc) CheckAccountAdmission(ctx context.Context, accountID int64) (bool, error) {
	return f(ctx, accountID)
}

func TestActiveTestAdmissionChecksUnlimitedAndReleasesRejectedSlots(t *testing.T) {
	for _, limit := range []int{0, -1, 3} {
		for _, gateErr := range []error{nil, errors.New("admission database unavailable")} {
			cache := &proxyLegacyCache{acquireResult: true}
			svc := NewConcurrencyService(cache)
			calls := 0
			svc.SetAccountAdmissionGate(accountAdmissionGateFunc(func(_ context.Context, accountID int64) (bool, error) {
				calls++
				require.Equal(t, int64(7), accountID)
				return false, gateErr
			}))
			result, err := svc.AcquireAccountSlot(context.Background(), 7, limit)
			if gateErr != nil {
				require.ErrorIs(t, err, gateErr)
			} else {
				require.ErrorIs(t, err, ErrAccountAdmissionDenied)
			}
			require.False(t, result.Acquired)
			require.Nil(t, result.ReleaseFunc)
			require.Equal(t, 1, calls)
			if limit > 0 {
				require.Equal(t, []int64{7}, cache.releasedAccountIDs)
			} else {
				require.Empty(t, cache.releasedAccountIDs)
			}
		}
	}
}

func TestActiveTestAdmissionDoesNotQueryWhileSaturated(t *testing.T) {
	cache := &proxyLegacyCache{}
	svc := NewConcurrencyService(cache)
	svc.SetAccountAdmissionGate(accountAdmissionGateFunc(func(context.Context, int64) (bool, error) {
		t.Fatal("saturated concurrency must not query account admission")
		return false, nil
	}))
	for range 3 {
		result, err := svc.AcquireAccountSlot(context.Background(), 7, 1)
		require.NoError(t, err)
		require.False(t, result.Acquired)
	}
	require.Empty(t, cache.releasedAccountIDs)
}
