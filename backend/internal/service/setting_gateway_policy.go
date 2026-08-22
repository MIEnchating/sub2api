package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultGatewayStreamDataIntervalTimeoutSeconds = 180
	defaultOpenAIStickyEscapeTTFTMs                = 15000
	defaultOpenAIStickyEscapeErrorRate             = 0.5
)

var gatewayControllablePlatforms = []string{
	PlatformAnthropic,
	PlatformOpenAI,
	PlatformGemini,
	PlatformAntigravity,
	PlatformGrok,
	PlatformKimi,
	PlatformZhipu,
	PlatformDeepseek,
}

// GatewayRuntimePolicy is an immutable snapshot used on gateway hot paths.
// Store a fresh snapshot rather than mutating PlatformEnabled after publication.
type GatewayRuntimePolicy struct {
	StreamDataIntervalTimeoutSeconds          int
	OpenAIFirstOutputTimeoutSeconds           int
	OpenAIHighEffortFirstOutputTimeoutSeconds int
	OpenAIAccountUniqueFingerprintEnabled     bool
	StickyEscapeEnabled                       bool
	StickyEscapeTTFTMs                        int
	StickyEscapeErrorRate                     float64
	PlatformEnabled                           map[string]bool
}

func defaultGatewayPlatformEnabled() map[string]bool {
	result := make(map[string]bool, len(gatewayControllablePlatforms))
	for _, platform := range gatewayControllablePlatforms {
		result[platform] = true
	}
	return result
}

func normalizeGatewayPlatformEnabled(input map[string]bool) (map[string]bool, error) {
	result := defaultGatewayPlatformEnabled()
	for platform, enabled := range input {
		platform = strings.ToLower(strings.TrimSpace(platform))
		if !IsAllowedQuotaPlatform(platform) {
			return nil, infraerrors.BadRequest("INVALID_GATEWAY_PLATFORM", fmt.Sprintf("unsupported gateway platform: %s", platform))
		}
		result[platform] = enabled
	}
	return result, nil
}

func gatewayRuntimePolicyFromConfig(cfg *config.Config) *GatewayRuntimePolicy {
	policy := &GatewayRuntimePolicy{
		StreamDataIntervalTimeoutSeconds:      defaultGatewayStreamDataIntervalTimeoutSeconds,
		OpenAIAccountUniqueFingerprintEnabled: true,
		StickyEscapeEnabled:                   true,
		StickyEscapeTTFTMs:                    defaultOpenAIStickyEscapeTTFTMs,
		StickyEscapeErrorRate:                 defaultOpenAIStickyEscapeErrorRate,
		PlatformEnabled:                       defaultGatewayPlatformEnabled(),
	}
	if cfg == nil {
		return policy
	}
	policy.StreamDataIntervalTimeoutSeconds = cfg.Gateway.StreamDataIntervalTimeout
	policy.OpenAIFirstOutputTimeoutSeconds = cfg.Gateway.OpenAIFirstOutputTimeoutSeconds
	policy.OpenAIHighEffortFirstOutputTimeoutSeconds = cfg.Gateway.OpenAIHighEffortFirstOutputTimeoutSeconds
	policy.OpenAIAccountUniqueFingerprintEnabled = cfg.Gateway.OpenAIAccountUniqueFingerprintEnabled
	scheduler := cfg.Gateway.OpenAIScheduler
	policy.StickyEscapeEnabled = scheduler.StickyEscapeEnabled
	if !policy.StickyEscapeEnabled && scheduler.StickyEscapeTTFTMs == 0 && scheduler.StickyEscapeErrorRate == 0 {
		policy.StickyEscapeEnabled = true
	}
	if scheduler.StickyEscapeTTFTMs > 0 {
		policy.StickyEscapeTTFTMs = scheduler.StickyEscapeTTFTMs
	}
	if scheduler.StickyEscapeErrorRate >= 0 && scheduler.StickyEscapeErrorRate <= 1 {
		policy.StickyEscapeErrorRate = scheduler.StickyEscapeErrorRate
	}
	if scheduler.StickyEscapeErrorRate == 0 && scheduler.StickyEscapeTTFTMs == 0 {
		policy.StickyEscapeErrorRate = defaultOpenAIStickyEscapeErrorRate
	}
	return policy
}

func (s *SettingService) gatewayRuntimePolicySnapshot() *GatewayRuntimePolicy {
	if s != nil {
		if policy, ok := s.gatewayRuntimePolicy.Load().(*GatewayRuntimePolicy); ok && policy != nil {
			return policy
		}
	}
	if s == nil {
		return gatewayRuntimePolicyFromConfig(nil)
	}
	return gatewayRuntimePolicyFromConfig(s.cfg)
}

func (s *SettingService) storeGatewayRuntimePolicy(settings *SystemSettings) {
	if s == nil || settings == nil {
		return
	}
	platforms, err := normalizeGatewayPlatformEnabled(settings.GatewayPlatformEnabled)
	if err != nil {
		platforms = defaultGatewayPlatformEnabled()
	}
	s.gatewayRuntimePolicy.Store(&GatewayRuntimePolicy{
		StreamDataIntervalTimeoutSeconds:          settings.GatewayStreamDataIntervalTimeoutSeconds,
		OpenAIFirstOutputTimeoutSeconds:           settings.OpenAIFirstOutputTimeoutSeconds,
		OpenAIHighEffortFirstOutputTimeoutSeconds: settings.OpenAIHighEffortFirstOutputTimeoutSeconds,
		OpenAIAccountUniqueFingerprintEnabled:     settings.OpenAIAccountUniqueFingerprintEnabled,
		StickyEscapeEnabled:                       settings.OpenAIStickyEscapeEnabled,
		StickyEscapeTTFTMs:                        settings.OpenAIStickyEscapeTTFTMs,
		StickyEscapeErrorRate:                     settings.OpenAIStickyEscapeErrorRate,
		PlatformEnabled:                           platforms,
	})
}

// WarmGatewayRuntimePolicy loads persisted overrides synchronously during startup.
func (s *SettingService) WarmGatewayRuntimePolicy(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	values, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	policy, err := parseGatewayRuntimePolicySettings(values, gatewayRuntimePolicyFromConfig(s.cfg))
	if err != nil {
		return err
	}
	s.gatewayRuntimePolicy.Store(policy)
	return nil
}

func (s *SettingService) GatewayStreamDataIntervalTimeout() time.Duration {
	seconds := s.gatewayRuntimePolicySnapshot().StreamDataIntervalTimeoutSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (s *SettingService) GatewayOpenAIFirstOutputTimeout(reasoningEffort string) time.Duration {
	policy := s.gatewayRuntimePolicySnapshot()
	seconds := policy.OpenAIFirstOutputTimeoutSeconds
	switch strings.ToLower(strings.TrimSpace(reasoningEffort)) {
	case "high", "xhigh", "max":
		if policy.OpenAIHighEffortFirstOutputTimeoutSeconds > 0 {
			seconds = policy.OpenAIHighEffortFirstOutputTimeoutSeconds
		}
	}
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (s *SettingService) OpenAIAccountUniqueFingerprintEnabled() bool {
	return s.gatewayRuntimePolicySnapshot().OpenAIAccountUniqueFingerprintEnabled
}

func (s *SettingService) GatewayOpenAIStickyEscapeConfig() (bool, float64, float64) {
	policy := s.gatewayRuntimePolicySnapshot()
	return policy.StickyEscapeEnabled, float64(policy.StickyEscapeTTFTMs), policy.StickyEscapeErrorRate
}

func (s *SettingService) IsGatewayPlatformEnabled(platform string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" || platform == PlatformComposite {
		return true
	}
	policy := s.gatewayRuntimePolicySnapshot()
	enabled, known := policy.PlatformEnabled[platform]
	return !known || enabled
}

func parseGatewayRuntimePolicySettings(settings map[string]string, fallback *GatewayRuntimePolicy) (*GatewayRuntimePolicy, error) {
	if fallback == nil {
		fallback = gatewayRuntimePolicyFromConfig(nil)
	}
	policy := *fallback
	policy.PlatformEnabled = defaultGatewayPlatformEnabled()
	if raw, ok := settings[SettingKeyGatewayStreamDataIntervalTimeoutSeconds]; ok {
		if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			policy.StreamDataIntervalTimeoutSeconds = value
		}
	}
	if raw, ok := settings[SettingKeyOpenAIFirstOutputTimeoutSeconds]; ok {
		if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			policy.OpenAIFirstOutputTimeoutSeconds = value
		}
	}
	if raw, ok := settings[SettingKeyOpenAIHighEffortFirstOutputTimeoutSeconds]; ok {
		if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			policy.OpenAIHighEffortFirstOutputTimeoutSeconds = value
		}
	}
	if raw, ok := settings[SettingKeyOpenAIAccountUniqueFingerprintEnabled]; ok {
		if value, err := strconv.ParseBool(strings.TrimSpace(raw)); err == nil {
			policy.OpenAIAccountUniqueFingerprintEnabled = value
		}
	}
	if raw, ok := settings[SettingKeyOpenAIStickyEscapeEnabled]; ok {
		if value, err := strconv.ParseBool(strings.TrimSpace(raw)); err == nil {
			policy.StickyEscapeEnabled = value
		}
	}
	if raw, ok := settings[SettingKeyOpenAIStickyEscapeTTFTMs]; ok {
		if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			policy.StickyEscapeTTFTMs = value
		}
	}
	if raw, ok := settings[SettingKeyOpenAIStickyEscapeErrorRate]; ok {
		if value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			policy.StickyEscapeErrorRate = value
		}
	}
	if raw := strings.TrimSpace(settings[SettingKeyGatewayPlatformEnabled]); raw != "" {
		var configured map[string]bool
		if err := json.Unmarshal([]byte(raw), &configured); err != nil {
			return nil, fmt.Errorf("parse %s: %w", SettingKeyGatewayPlatformEnabled, err)
		}
		normalized, err := normalizeGatewayPlatformEnabled(configured)
		if err != nil {
			return nil, err
		}
		policy.PlatformEnabled = normalized
	}
	if policy.StreamDataIntervalTimeoutSeconds != 0 && (policy.StreamDataIntervalTimeoutSeconds < 30 || policy.StreamDataIntervalTimeoutSeconds > 300) {
		policy.StreamDataIntervalTimeoutSeconds = fallback.StreamDataIntervalTimeoutSeconds
	}
	if policy.OpenAIFirstOutputTimeoutSeconds != 0 && (policy.OpenAIFirstOutputTimeoutSeconds < 30 || policy.OpenAIFirstOutputTimeoutSeconds > 600) {
		policy.OpenAIFirstOutputTimeoutSeconds = fallback.OpenAIFirstOutputTimeoutSeconds
	}
	if policy.OpenAIHighEffortFirstOutputTimeoutSeconds != 0 && (policy.OpenAIHighEffortFirstOutputTimeoutSeconds < 30 || policy.OpenAIHighEffortFirstOutputTimeoutSeconds > 600) {
		policy.OpenAIHighEffortFirstOutputTimeoutSeconds = fallback.OpenAIHighEffortFirstOutputTimeoutSeconds
	}
	if policy.StickyEscapeTTFTMs <= 0 {
		policy.StickyEscapeTTFTMs = fallback.StickyEscapeTTFTMs
	}
	if policy.StickyEscapeErrorRate < 0 || policy.StickyEscapeErrorRate > 1 {
		policy.StickyEscapeErrorRate = fallback.StickyEscapeErrorRate
	}
	return &policy, nil
}

func validateGatewayRuntimePolicy(settings *SystemSettings) error {
	if settings.GatewayStreamDataIntervalTimeoutSeconds != 0 && (settings.GatewayStreamDataIntervalTimeoutSeconds < 30 || settings.GatewayStreamDataIntervalTimeoutSeconds > 300) {
		return infraerrors.BadRequest("INVALID_STREAM_DATA_INTERVAL_TIMEOUT", "gateway_stream_data_interval_timeout_seconds must be 0 or between 30 and 300")
	}
	for name, value := range map[string]int{
		"openai_first_output_timeout_seconds":             settings.OpenAIFirstOutputTimeoutSeconds,
		"openai_high_effort_first_output_timeout_seconds": settings.OpenAIHighEffortFirstOutputTimeoutSeconds,
	} {
		if value != 0 && (value < 30 || value > 600) {
			return infraerrors.BadRequest("INVALID_OPENAI_FIRST_OUTPUT_TIMEOUT", name+" must be 0 or between 30 and 600")
		}
	}
	stickyThresholdsUnset := settings.OpenAIStickyEscapeTTFTMs == 0 && settings.OpenAIStickyEscapeErrorRate == 0
	if settings.OpenAIStickyEscapeTTFTMs < 0 {
		return infraerrors.BadRequest("INVALID_OPENAI_STICKY_ESCAPE_TTFT", "openai_sticky_escape_ttft_ms must be greater than 0")
	}
	if settings.OpenAIStickyEscapeTTFTMs == 0 {
		settings.OpenAIStickyEscapeTTFTMs = defaultOpenAIStickyEscapeTTFTMs
	}
	if settings.OpenAIStickyEscapeErrorRate < 0 || settings.OpenAIStickyEscapeErrorRate > 1 {
		return infraerrors.BadRequest("INVALID_OPENAI_STICKY_ESCAPE_ERROR_RATE", "openai_sticky_escape_error_rate must be between 0 and 1")
	}
	if stickyThresholdsUnset {
		settings.OpenAIStickyEscapeErrorRate = defaultOpenAIStickyEscapeErrorRate
	}
	platforms, err := normalizeGatewayPlatformEnabled(settings.GatewayPlatformEnabled)
	if err != nil {
		return err
	}
	settings.GatewayPlatformEnabled = platforms
	return nil
}
