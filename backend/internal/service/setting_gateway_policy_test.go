//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type gatewayPolicySettingRepo struct {
	values map[string]string
}

func (r *gatewayPolicySettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *gatewayPolicySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *gatewayPolicySettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *gatewayPolicySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *gatewayPolicySettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *gatewayPolicySettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *gatewayPolicySettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestGatewayRuntimePolicyFallsBackToConfigAndWarmsOverrides(t *testing.T) {
	cfg := &config.Config{Gateway: config.GatewayConfig{
		StreamDataIntervalTimeout:                 90,
		OpenAIFirstOutputTimeoutSeconds:           120,
		OpenAIHighEffortFirstOutputTimeoutSeconds: 240,
		OpenAIAccountUniqueFingerprintEnabled:     true,
		OpenAIScheduler: config.GatewayOpenAISchedulerConfig{
			StickyEscapeEnabled:   true,
			StickyEscapeTTFTMs:    18000,
			StickyEscapeErrorRate: 0.4,
		},
	}}
	repo := &gatewayPolicySettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, cfg)

	require.Equal(t, 90*time.Second, svc.GatewayStreamDataIntervalTimeout())
	require.Equal(t, 120*time.Second, svc.GatewayOpenAIFirstOutputTimeout("medium"))
	require.Equal(t, 240*time.Second, svc.GatewayOpenAIFirstOutputTimeout("high"))
	require.True(t, svc.OpenAIAccountUniqueFingerprintEnabled())

	repo.values[SettingKeyGatewayStreamDataIntervalTimeoutSeconds] = "45"
	repo.values[SettingKeyOpenAIFirstOutputTimeoutSeconds] = "60"
	repo.values[SettingKeyOpenAIHighEffortFirstOutputTimeoutSeconds] = "180"
	repo.values[SettingKeyOpenAIStickyEscapeEnabled] = "false"
	repo.values[SettingKeyOpenAIStickyEscapeTTFTMs] = "9000"
	repo.values[SettingKeyOpenAIStickyEscapeErrorRate] = "0.25"
	repo.values[SettingKeyOpenAIAccountUniqueFingerprintEnabled] = "false"
	repo.values[SettingKeyGatewayPlatformEnabled] = `{"openai":false,"gemini":true}`
	require.NoError(t, svc.WarmGatewayRuntimePolicy(context.Background()))

	require.Equal(t, 45*time.Second, svc.GatewayStreamDataIntervalTimeout())
	require.Equal(t, 60*time.Second, svc.GatewayOpenAIFirstOutputTimeout("medium"))
	require.Equal(t, 180*time.Second, svc.GatewayOpenAIFirstOutputTimeout("xhigh"))
	enabled, ttft, errorRate := svc.GatewayOpenAIStickyEscapeConfig()
	require.False(t, enabled)
	require.Equal(t, float64(9000), ttft)
	require.Equal(t, 0.25, errorRate)
	require.False(t, svc.OpenAIAccountUniqueFingerprintEnabled())
	require.False(t, svc.IsGatewayPlatformEnabled(PlatformOpenAI))
	require.True(t, svc.IsGatewayPlatformEnabled(PlatformGemini))
	require.True(t, svc.IsGatewayPlatformEnabled(PlatformAnthropic), "omitted platforms default to enabled")
}

func TestGatewayRuntimePolicyUpdateIsImmediate(t *testing.T) {
	repo := &gatewayPolicySettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)

	settings.GatewayStreamDataIntervalTimeoutSeconds = 75
	settings.OpenAIFirstOutputTimeoutSeconds = 90
	settings.OpenAIHighEffortFirstOutputTimeoutSeconds = 150
	settings.OpenAIStickyEscapeEnabled = false
	settings.OpenAIStickyEscapeTTFTMs = 12000
	settings.OpenAIStickyEscapeErrorRate = 0.2
	settings.OpenAIAccountUniqueFingerprintEnabled = true
	settings.GatewayPlatformEnabled[PlatformGrok] = false
	require.NoError(t, svc.UpdateSettings(context.Background(), settings))

	require.Equal(t, 75*time.Second, svc.GatewayStreamDataIntervalTimeout())
	require.Equal(t, 150*time.Second, svc.GatewayOpenAIFirstOutputTimeout("max"))
	require.False(t, svc.IsGatewayPlatformEnabled(PlatformGrok))
	require.True(t, svc.OpenAIAccountUniqueFingerprintEnabled())
	require.Equal(t, "true", repo.values[SettingKeyOpenAIAccountUniqueFingerprintEnabled])
}

func TestValidateGatewayRuntimePolicyRanges(t *testing.T) {
	base := func() *SystemSettings {
		return &SystemSettings{
			GatewayStreamDataIntervalTimeoutSeconds: 180,
			OpenAIFirstOutputTimeoutSeconds:         120,
			OpenAIStickyEscapeTTFTMs:                15000,
			OpenAIStickyEscapeErrorRate:             0.5,
			GatewayPlatformEnabled:                  defaultGatewayPlatformEnabled(),
		}
	}

	settings := base()
	settings.GatewayStreamDataIntervalTimeoutSeconds = 29
	require.Error(t, validateGatewayRuntimePolicy(settings))

	settings = base()
	settings.OpenAIFirstOutputTimeoutSeconds = 601
	require.Error(t, validateGatewayRuntimePolicy(settings))

	settings = base()
	settings.OpenAIStickyEscapeErrorRate = 1.01
	require.Error(t, validateGatewayRuntimePolicy(settings))

	settings = base()
	settings.GatewayPlatformEnabled["unknown"] = true
	require.Error(t, validateGatewayRuntimePolicy(settings))
}
