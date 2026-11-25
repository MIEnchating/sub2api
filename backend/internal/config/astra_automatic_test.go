package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAstraAutomaticDependenciesKeepsSelectionsDetached(t *testing.T) {
	value := AstraRoutingSettings{
		CookiePool: CodexGatewayPinConfig{
			Enabled: true, SourceAccountIDs: []int64{299}, TargetAccountIDs: []int64{300},
		},
	}
	resolved, err := ResolveAstraDependencies(value)
	require.NoError(t, err)
	resolved.CookiePool.SourceAccountIDs[0] = 1
	resolved.CookiePool.TargetAccountIDs[0] = 2
	require.Equal(t, []int64{299}, value.CookiePool.SourceAccountIDs)
	require.Equal(t, []int64{300}, value.CookiePool.TargetAccountIDs)
	value.CookiePool.SourceAccountIDs = nil
	_, err = ResolveAstraDependencies(value)
	require.ErrorContains(t, err, "astra_source_required")
}

func TestAstraLegacyRotationIsRejected(t *testing.T) {
	value := AstraRoutingSettings{CookiePool: CodexGatewayPinConfig{RotateNodes: true}}
	_, err := ResolveAstraDependencies(value)
	require.ErrorContains(t, err, "astra_legacy_rotation_unsupported")
}

func TestAstraNodeCooldownBounds(t *testing.T) {
	for _, seconds := range []int{-1, 1, 59, 86401} {
		require.Error(t, (CodexGatewayPinConfig{NodeCooldownSeconds: seconds}).Validate())
	}
	for _, seconds := range []int{0, 60, 3600, 86400} {
		require.NoError(t, (CodexGatewayPinConfig{NodeCooldownSeconds: seconds}).Validate())
	}
}
