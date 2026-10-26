//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestImageCountBillingContextTiersAcrossGateways(t *testing.T) {
	for _, source := range []string{"group", "channel"} {
		for _, mode := range []BillingMode{BillingModePerRequest, BillingModeImage} {
			for _, enabled := range []bool{false, true} {
				for _, totalInput := range []int{800, 5000} {
					t.Run(fmt.Sprintf("%s/%s/enabled=%v/input=%d", source, mode, enabled, totalInput), func(t *testing.T) {
						groupID := int64(908)
						model := "gpt-image-2"
						resolver := newOpenAITokenImageChannelPricingResolverForTest(t, groupID, model)
						maxTokens := 1000
						pricing := ChannelModelPricing{
							Models: []string{model}, BillingMode: mode,
							Intervals: []PricingInterval{
								{MinTokens: 0, MaxTokens: &maxTokens, PerRequestPrice: f64p(0.1)},
								{MinTokens: 1000, PerRequestPrice: f64p(0.3)},
							},
						}
						group := &Group{ID: groupID, Platform: PlatformOpenAI, Hydrated: true, Status: StatusActive,
							AllowImageGeneration: true, ImageCountBilling: enabled}
						if source == "group" {
							group.ModelPricing = []ChannelModelPricing{pricing}
						} else {
							cached, ok := resolver.channelService.cache.Load().(*channelCache)
							require.True(t, ok)
							cached.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}] = &pricing
						}
						apiKey := &APIKey{ID: 5, GroupID: &groupID, Group: group}
						want := 0.2
						if totalInput >= 1000 {
							want = 0.6
						}
						// Claude input excludes cached input. Keep cache tokens when selecting tiers.
						gateway := &GatewayService{billingService: resolver.billingService, resolver: resolver}
						cost := gateway.calculateRecordUsageCost(context.Background(), &ForwardResult{
							Model: model, ImageCount: 2, ImageSize: "1K", ImageEndpoint: true,
							Usage: ClaudeUsage{InputTokens: 100, CacheCreationInputTokens: 100, CacheReadInputTokens: totalInput - 200, OutputTokens: 100},
						}, apiKey, model, 1, 1.5, time.Time{})
						require.InDelta(t, want, cost.TotalCost, 1e-12, "gateway")
						require.InDelta(t, want*1.5, cost.ActualCost, 1e-12, "gateway multiplier")

						// OpenAI input already includes cached input. Count it exactly once.
						openai := &OpenAIGatewayService{billingService: resolver.billingService, resolver: resolver}
						cost, err := openai.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{
							Model: model, ImageCount: 2, ImageSize: "1K", ImageEndpoint: true,
							Usage: OpenAIUsage{InputTokens: totalInput, CacheCreationInputTokens: 100, CacheReadInputTokens: totalInput - 200, OutputTokens: 100},
						}, apiKey, []string{model}, 1, 1.5, 1, 1,
							UsageTokens{InputTokens: 100, CacheCreationTokens: 100, CacheReadTokens: totalInput - 200, OutputTokens: 100},
							"", nil, false, time.Time{})
						require.NoError(t, err)
						require.InDelta(t, want, cost.TotalCost, 1e-12, "openai")
						require.InDelta(t, want*1.5, cost.ActualCost, 1e-12, "openai multiplier")
					})
				}
			}
		}
	}
}
