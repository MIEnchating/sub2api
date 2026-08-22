package service

import "time"

func (s *GatewayService) gatewayStreamDataIntervalTimeout() time.Duration {
	if s == nil {
		return 0
	}
	if s.settingService != nil {
		return s.settingService.GatewayStreamDataIntervalTimeout()
	}
	if s.cfg == nil || s.cfg.Gateway.StreamDataIntervalTimeout <= 0 {
		return 0
	}
	return time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
}

func (s *OpenAIGatewayService) gatewayStreamDataIntervalTimeout() time.Duration {
	if s == nil {
		return 0
	}
	if s.settingService != nil {
		return s.settingService.GatewayStreamDataIntervalTimeout()
	}
	if s.cfg == nil || s.cfg.Gateway.StreamDataIntervalTimeout <= 0 {
		return 0
	}
	return time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
}

func (s *AntigravityGatewayService) gatewayStreamDataIntervalTimeout() time.Duration {
	if s == nil || s.settingService == nil {
		return 0
	}
	return s.settingService.GatewayStreamDataIntervalTimeout()
}
