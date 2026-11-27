//go:build unit

package service

import (
	"context"
)

type requestProxyUpstreamMergeRepo struct {
	*upstreamBillingProbeAccountRepo
	atomicCreates   int
	ordinaryCreates int
	bindCalls       int
	initialGroupIDs []int64
	initialPolicy   *OAuthAPIKeyDispositionConfig
	atomicGroups    []AccountGroup
}

func (r *requestProxyUpstreamMergeRepo) Create(ctx context.Context, account *Account) error {
	r.ordinaryCreates++
	return r.upstreamBillingProbeAccountRepo.Create(ctx, account)
}

func (r *requestProxyUpstreamMergeRepo) BindGroups(_ context.Context, _ int64, _ []int64) error {
	r.bindCalls++
	return nil
}

func (r *requestProxyUpstreamMergeRepo) CreateWithAccountGroups(ctx context.Context, account *Account, groups []AccountGroup) error {
	r.atomicCreates++
	r.initialGroupIDs = append([]int64(nil), account.GroupIDs...)
	r.initialPolicy = account.InitialAPIKeyDisposition
	r.atomicGroups = append([]AccountGroup(nil), groups...)
	return r.upstreamBillingProbeAccountRepo.Create(ctx, account)
}
