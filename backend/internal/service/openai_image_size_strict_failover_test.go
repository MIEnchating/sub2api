package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIImageSizeSpilloverProfitAdmissionBinding(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_session", true: "existing_owner"}[bound], func(t *testing.T) {
			groupID := int64(75)
			cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{}}
			initial := int64(0)
			if bound {
				initial = 1
				cache.sessionBindings["openai:session"] = initial
			}
			svc := &OpenAIGatewayService{cache: cache}
			selection := &AccountSelectionResult{
				Account: &Account{ID: 3}, Acquired: true,
				PreserveStickyBinding: true, profitGate: &openAIProfitControlGate{},
			}
			svc.bindOpenAIImageSizeSpilloverAccount(context.Background(), &groupID, "session", initial, selection)
			require.Equal(t, bound, selection.PreserveStickyBinding)
			require.Equal(t, initial, cache.sessionBindings["openai:session"], "profit admission must precede a new binding")
		})
	}
}

func TestOpenAIImageSizeRoutingStrictFailover(t *testing.T) {
	for _, mode := range openAIImageSizeSchedulerModes {
		for _, tc := range []struct {
			name string
			size string
			busy bool
			want int64
		}{
			{name: "explicit_size_advances_incompatible_tier", size: "3840x2160", want: 3},
			{name: "unspecified_size_keeps_primary_tier", want: 1},
			{name: "concurrency_does_not_advance_strict_tier", busy: true, want: 1},
		} {
			t.Run(mode.name+"/"+tc.name, func(t *testing.T) {
				g, repo := strictFixture()
				g.StrictFailover.Tiers = [][]int64{{1}, {3}}
				repo.accounts = []Account{
					newOpenAIImageSizeTestAccount(1, 0, "1K"),
					newOpenAIImageSizeTestAccount(3, 0, "4K"),
				}
				for i := range repo.accounts {
					repo.accounts[i].GroupIDs = []int64{g.ID}
				}
				concurrency := schedulerTestConcurrencyCache{}
				if tc.busy {
					concurrency.acquireResults = map[int64]bool{1: false}
				}
				svc := newOpenAIImageSizeSchedulerTestService(mode, repo.accounts, nil, concurrency)
				svc.accountRepo = repo
				ctx := WithOpenAIImageSizeRouting(strictContext(g), tc.size)
				selection, _, err := svc.SelectAccountWithSchedulerForImages(ctx, &g.ID, "", "gpt-image-2", nil, OpenAIImagesCapabilityNative)
				require.NoError(t, err)
				require.NotNil(t, selection)
				require.Equal(t, tc.want, selection.Account.ID)
				if tc.busy {
					require.False(t, selection.Acquired)
					require.NotNil(t, selection.WaitPlan)
				}
			})
		}
	}
}
