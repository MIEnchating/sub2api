package service

import (
	"errors"
	"net/http"
	"time"
)

func (s *OpenAIGatewayService) SetPluginManager(manager *PluginManager) {
	s.pluginManager = manager
}

// doOpenAIUpstream 只在 OpenAI OAuth 能力绑定已启用时把真实请求交给插件。
// 插件返回标准 http.Response，响应解析、错误映射、SSE 和计费仍由现有核心链处理。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	if err := validateRegisteredAntiDegrade(account); err != nil {
		return nil, err
	}
	profile, err := resolveAccountTLSProfileForOpenAI(account, s.cfg)
	if err != nil {
		return nil, err
	}
	vmRouted := false
	return doAccount429Retry(request, account, func(attemptRequest *http.Request) (*http.Response, error) {
		if s.openaiVM != nil {
			if response, handled, err := s.openaiVM.RoundTrip(attemptRequest, account); handled {
				vmRouted = true
				return response, err
			}
		}
		if vmRouted {
			return nil, errors.New("OpenAI VM binding changed during retry; direct fallback is disabled")
		}
		if account != nil {
			if err := validateOpenAIVMProxy(account, account.Proxy); err != nil {
				return nil, err
			}
		}
		if profile != nil && (s.pluginManager == nil || !s.pluginManager.ShouldRouteOpenAIOAuth(account)) {
			return s.httpUpstream.DoWithTLS(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency(), profile)
		}
		if s.pluginManager != nil {
			response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(attemptRequest.Context(), attemptRequest, proxyURL, account)
			if handled {
				return response, err
			}
		}
		if profile != nil {
			return s.httpUpstream.DoWithTLS(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency(), profile)
		}
		return s.httpUpstream.Do(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency())
	})
}

// doOpenAIAccountTestUpstream 让 OpenAI OAuth 账号测试与真实转发使用同一插件路径。
// API Key 和未命中插件的账号保持各自原有的 HTTPUpstream 行为。
func (s *AccountTestService) doOpenAIAccountTestUpstream(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
) (*http.Response, error) {
	// Validate persisted protection before invoking a plugin. Plugins are an
	// alternate transport, but must never bypass a malformed account strategy.
	if err := validateRegisteredAntiDegrade(account); err != nil {
		return nil, err
	}
	profile, err := resolveAccountTLSProfileForOpenAI(account, s.cfg)
	if err != nil {
		return nil, err
	}
	vmRouted := false
	perform := func(attemptRequest *http.Request) (*http.Response, error) {
		if s.openaiVM != nil {
			started := time.Now().UTC()
			if response, handled, err := s.openaiVM.RoundTrip(attemptRequest, account); handled {
				vmRouted = true
				return observeAccountTestResponse(attemptRequest.Context(), started, response, err)
			}
		}
		if vmRouted {
			return nil, errors.New("OpenAI VM binding changed during test retry; direct fallback is disabled")
		}
		if account != nil {
			if err := validateOpenAIVMProxy(account, account.Proxy); err != nil {
				return nil, err
			}
		}
		if profile != nil && (s.pluginManager == nil || !s.pluginManager.ShouldRouteOpenAIOAuth(account)) {
			return s.doAccountTestHTTPWithTLS(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency(), profile)
		}
		if s.pluginManager != nil {
			started := time.Now().UTC()
			response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(attemptRequest.Context(), attemptRequest, proxyURL, account)
			if handled {
				return observeAccountTestResponse(attemptRequest.Context(), started, response, err)
			}
		}
		if profile != nil {
			return s.doAccountTestHTTPWithTLS(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency(), profile)
		}
		if useTLSFallback && (account == nil || !account.IdentityProtectionEnabled()) {
			return s.doAccountTestHTTPWithTLS(
				attemptRequest,
				proxyURL,
				account.ID,
				account.Mode1EffectiveConcurrency(),
				s.tlsFPProfileService.ResolveTLSProfile(account),
			)
		}
		return s.doAccountTestHTTP(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency())
	}
	if request.Context().Value(generationPreviewContextKey{}) == true || request.Context().Value(accountTestUsageKey{}) != nil {
		return perform(request)
	}
	return doAccount429Retry(request, account, perform)
}
