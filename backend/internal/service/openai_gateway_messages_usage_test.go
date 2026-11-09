//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestCopyOpenAIUsageFromResponsesUsageTrustsCanonicalCacheCreationValue(t *testing.T) {
	usage := &apicompat.ResponsesUsage{
		InputTokens:              20,
		OutputTokens:             2,
		CacheCreationInputTokens: 0,
		InputTokensDetails: &apicompat.ResponsesInputTokensDetails{
			CachedTokens:     3,
			CacheWriteTokens: 19,
		},
	}

	got := copyOpenAIUsageFromResponsesUsage(usage)

	require.Equal(t, 20, got.InputTokens)
	require.Equal(t, 3, got.CacheReadInputTokens)
	require.Zero(t, got.CacheCreationInputTokens)
}

func TestCopyOpenAIUsageFromResponsesUsagePreservesMatchingTTL(t *testing.T) {
	observed := OpenAIUsage{CacheCreationInputTokens: 200, CacheCreation30mInputTokens: 150, CacheCreation12hInputTokens: 50}
	for _, aggregate := range []int{0, 100, 200} {
		got := copyOpenAIUsageFromResponsesUsage(&apicompat.ResponsesUsage{CacheCreationInputTokens: aggregate}, observed)
		require.Equal(t, aggregate, got.CacheCreationInputTokens)
		if aggregate == 200 {
			require.Equal(t, 150, got.CacheCreation30mInputTokens)
			require.Equal(t, 50, got.CacheCreation12hInputTokens)
		} else {
			require.Zero(t, got.CacheCreation30mInputTokens)
			require.Zero(t, got.CacheCreation12hInputTokens)
		}
	}
}
