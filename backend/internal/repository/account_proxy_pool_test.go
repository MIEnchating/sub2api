package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountRepositoryRejectsRetiredProxyPoolBeforeWriting(t *testing.T) {
	ctx := context.Background()
	r := &accountRepository{}
	ids := []int64{11, 12}
	ar := &service.Account{ProxyIDs: ids}
	require.ErrorIs(t, r.Create(ctx, ar), service.ErrAccountMultiProxyRetired)
	require.ErrorIs(t, r.CreateWithAccountGroups(ctx, ar, nil), service.ErrAccountMultiProxyRetired)
	require.ErrorIs(t, r.Update(ctx, ar), service.ErrAccountMultiProxyRetired)
	_, err := r.BulkUpdate(ctx, []int64{1}, service.AccountBulkUpdate{ProxyIDs: &ids})
	require.ErrorIs(t, err, service.ErrAccountMultiProxyRetired)
}
