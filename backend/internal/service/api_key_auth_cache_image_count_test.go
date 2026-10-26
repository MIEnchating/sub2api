package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotImageCountBillingRoundtrip(t *testing.T) {
	groupID := int64(50)
	apiKey := &APIKey{
		ID: 82, UserID: 40, GroupID: &groupID, Key: "sk-image-roundtrip", Status: StatusActive,
		User: &User{ID: 40, Status: StatusActive},
		Group: &Group{
			ID: groupID, Platform: PlatformOpenAI, Status: StatusActive,
			AllowImageGeneration: true, ImageCountBilling: true,
			ImageRateIndependent: true, ImageRateMultiplier: 1.5,
			UserConcurrencyLimit: 4, ProfitControlEnabled: true,
		},
	}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.True(t, materialized.Group.ImageCountBilling)
	require.True(t, materialized.Group.ImageRateIndependent)
	require.Equal(t, 1.5, materialized.Group.ImageRateMultiplier)
	require.Equal(t, 4, materialized.Group.UserConcurrencyLimit)
	require.True(t, materialized.Group.ProfitControlEnabled)

	// A previously shipped snapshot has no image-count switch; it must be reloaded.
	cached.Snapshot.Version = 27
	materialized, used, err = svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.False(t, used)
	require.Nil(t, materialized)
}
