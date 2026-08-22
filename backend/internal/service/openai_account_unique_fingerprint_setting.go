package service

import "github.com/Wei-Shaw/sub2api/internal/config"

func resolveOpenAIAccountUniqueFingerprintEnabled(settingService *SettingService, cfg *config.Config) bool {
	if settingService != nil {
		return settingService.OpenAIAccountUniqueFingerprintEnabled()
	}
	return cfg != nil && cfg.Gateway.OpenAIAccountUniqueFingerprintEnabled
}
