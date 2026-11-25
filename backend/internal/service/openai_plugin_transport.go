package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (s *OpenAIGatewayService) SetPluginManager(manager *PluginManager) {
	s.pluginManager = manager
}

// doOpenAIUpstream 只在 OpenAI OAuth 能力绑定已启用时把真实请求交给插件。
// 插件返回标准 http.Response，响应解析、错误映射、SSE 和计费仍由现有核心链处理。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (result *http.Response, resultErr error) {
	if request == nil || account == nil {
		return nil, errors.New("OpenAI request or account is missing")
	}
	if err := rejectRetiredVMAccount(request.Context(), s.accountRepo, account); err != nil {
		return nil, err
	}
	if attempt, _ := request.Context().Value(groupSmartWSAttemptKey{}).(*groupSmartWSAttempt); attempt != nil && attempt.sent.Load() {
		return nil, finishGroupSmartWSAttempt(attempt, errSmartWSAlreadyDispatched)
	}
	// Resolve ticket policy at the final native send boundary, after request identity
	// normalization and before opening a network connection. All HTTP aliases share it.
	if account.IsOpenAIOAuthLike() && s.openAICodexTicketEnabledContext(request.Context()) {
		raw, err := snapshotHTTPRequestBody(request)
		if err != nil {
			return nil, err
		}
		replaceHTTPRequestJSONBody(request, raw)
		model := extractOpenAICodexTicketModel(raw)
		account, err = s.refreshCodexTicketAccountForSend(request.Context(), account, model, request.Header.Get("Authorization"))
		if err != nil {
			return nil, err
		}
		if err := s.applyOpenAICodexTicket(request.Context(), account, model, request.Header); err != nil {
			return nil, err
		}
	}
	route, scoped := openAIRequestProxyFromContext(request.Context(), account.ID)
	// A fresh admission snapshot may disable request routing after acquisition.
	// Reject before send rather than bypassing the scope's cancellation/body fence.
	if route != nil && !account.OpenAIRequestProxyEnabled() {
		return nil, denyOpenAITurn("account_binding_changed")
	}
	if account.OpenAIRequestProxyEnabled() {
		if !scoped {
			return nil, errOpenAIRequestProxyUnavailable
		}
		var bindErr error
		request, bindErr = route.bind(request)
		if bindErr != nil {
			return nil, bindErr
		}
		proxyURL = route.proxy
		defer func() { route.track(result) }()
		if s.codexTicketPinsEgress(request, account) || account.AuthConnectionID != "" {
			return nil, errOpenAIRequestProxyUnavailable
		}
	}
	if err := validateOpenAIConnectionKind(account); err != nil {
		return nil, err
	}
	var phaseGuard *openAIHTTPPhaseGuard
	request, phaseGuard = s.guardOpenAIHTTPRequest(request, account)
	request, finishHeaderTimeout := guardOpenAIResponseHeaderTimeout(request, account)
	defer func() {
		if result != nil && result.Request == nil {
			result.Request = request
		}
		if result != nil && result.Request != nil {
			result.Request = result.Request.WithContext(context.WithValue(result.Request.Context(), openAI429WeeklyObservedAtKey{}, time.Now()))
		}
		result, resultErr = phaseGuard.finish(result, resultErr)
		resultErr = finishHeaderTimeout(result, resultErr)
	}()
	if phaseGuard != nil && phaseGuard.ctx.Err() != nil {
		return nil, context.Cause(phaseGuard.ctx)
	}
	request, safetyInfo := s.applySafetyPromptToRequest(request, account)
	defer func() {
		if result != nil {
			result = s.wrapSafetyPromptResponse(result, safetyInfo)
		}
	}()
	if account.IsSIWCEnabled() {
		if request.URL == nil || request.URL.Scheme != "https" || request.URL.Host != "api.openai.com" || request.URL.Path != "/v1/responses" {
			return nil, errors.New("SIWC request must use the official Responses endpoint")
		}
		// Compatibility adapters can add internal headers after building a request.
		// Enforce SIWC headers at the final send boundary as well.
		authorization := request.Header.Get("Authorization")
		applySIWCRequestHeaders(request)
		request.Host = ""
		resp, err := s.doUpstreamWithProxyFallback(request.Context(), request, account, proxyURL)
		if err == nil && resp != nil && resp.StatusCode == http.StatusUnauthorized && s.excelOAuthReauth != nil {
			var cache OpenAITokenCache
			if s.openAITokenProvider != nil {
				cache = s.openAITokenProvider.tokenCache
			}
			s.excelOAuthReauth.rejectSIWCAccessToken(request.Context(), account, strings.TrimPrefix(authorization, "Bearer "), cache)
		}
		return resp, err
	}
	request = s.stickBoundCodexTicketRequest(request, account)
	boundTicket := s.codexTicketRequestBound(request, account)
	observationTicket := s.boundCodexTicket(request, account)
	observationHeaders := request.Header.Clone()
	releaseChat := func() {}
	if boundTicket {
		releaseChat = s.holdCodexTicketChat(account)
	}
	proxyURL, release, err := s.pinCodexTicketEgress(request, account, proxyURL)
	if err != nil {
		releaseChat()
		return nil, err
	}
	defer func() {
		observe := resultErr == nil && result != nil && result.StatusCode == http.StatusOK
		result, resultErr = attachCodexTicketEgressRelease(result, resultErr, func() {
			release()
			releaseChat()
			if observe {
				s.observeScheduledAstraCandy(request.Context(), observationHeaders, account, observationTicket)
			}
		})
	}()
	defer func() {
		if resultErr == nil {
			// Fast admission may have installed the ticket inside the upstream
			// adapter. Capture the binding before observation can revoke it.
			observedRequest := request
			if result != nil && result.Request != nil && s.codexTicketRequestBound(result.Request, account) {
				// Timing/fallback layers clone request headers before the routing
				// adapter installs a newly acquired bundle. Observe the sent copy.
				observedRequest = result.Request
			}
			if ticket := s.boundCodexTicket(observedRequest, account); ticket != nil {
				observationTicket = ticket
				observationHeaders = observedRequest.Header.Clone()
			}
			responseBound := boundTicket || s.codexTicketRequestBound(observedRequest, account)
			s.observeCodexTicketResponse(observedRequest, result, account)
			if responseBound && s.settingService != nil && s.settingService.CodexTicketStrictResponse(request.Context()) && codexResponseMismatches(observedRequest, result, account) {
				if result != nil && result.Body != nil {
					_ = result.Body.Close()
				}
				result = nil
				resultErr = ErrCodexTicketResponseRejected
			}
		}
	}()
	// Keep ticket observation/strict validation around the final response,
	// while every egress attempt retains its own plugin routing and trace.
	if _, independent := request.Context().Value(oauth429ExecutionKey{}).(*oauth429Execution); independent {
		return s.doUpstreamWithProxyFallback(request.Context(), request, account, proxyURL)
	}
	return doAccount429Retry(request, account, func(attemptRequest *http.Request) (*http.Response, error) {
		if attempt, _ := attemptRequest.Context().Value(groupSmartWSAttemptKey{}).(*groupSmartWSAttempt); attempt != nil && attempt.sent.Load() {
			return nil, finishGroupSmartWSAttempt(attempt, errSmartWSAlreadyDispatched)
		}
		return s.doUpstreamWithProxyFallback(attemptRequest.Context(), attemptRequest, account, proxyURL)
	})
}

// doOpenAIAccountTestUpstream applies the same stored ticket and transport lifecycle
// as production forwarding. Use the final payload model, after mapping/normalization;
// a self-harvested ticket must not depend on membership in the gateway-reuse target list.
func (s *AccountTestService) doOpenAIAccountTestUpstream(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
	payload ...[]byte,
) (result *http.Response, resultErr error) {
	if request == nil || account == nil {
		return nil, errors.New("OpenAI request or account is missing")
	}
	if err := rejectRetiredVMAccount(request.Context(), s.accountRepo, account); err != nil {
		return nil, err
	}
	if err := validateRegisteredAntiDegrade(account); err != nil {
		return nil, err
	}
	gateway := s.openaiGatewayService
	if account.OpenAIRequestProxyEnabled() {
		if account.AuthConnectionID != "" || account.IsManagedIPv6Bound() {
			return nil, errOpenAIRequestProxyUnavailable
		}
		// Scope acquisition is independent of optional plugin/ticket services.
		scopeService := gateway
		if scopeService == nil {
			scopeService = &OpenAIGatewayService{}
		}
		ctx, release, err := scopeService.beginOpenAIRequestProxy(request.Context(), nil, account)
		if err != nil {
			return nil, err
		}
		request = request.WithContext(ctx)
		route, _ := openAIRequestProxyFromContext(ctx, account.ID)
		request, err = route.bind(request)
		if err != nil {
			release()
			return nil, err
		}
		proxyURL = route.proxy
		defer func() {
			if resultErr != nil || result == nil || result.Body == nil {
				release()
				return
			}
			route.track(result)
			result.Body = &openAIRequestProxyOwnedBody{ReadCloser: result.Body, finish: release}
		}()
	}
	if gateway != nil && !IsAstraSourceAcquisition(request.Context()) && request.URL != nil &&
		request.URL.String() == chatgptCodexURL && gateway.openAICodexTicketEnabledContext(request.Context()) {
		var body []byte
		if len(payload) > 0 {
			body = payload[0]
		} else {
			var err error
			body, err = snapshotHTTPRequestBody(request)
			if err != nil {
				return nil, err
			}
			replaceHTTPRequestJSONBody(request, body)
		}
		model := extractOpenAICodexTicketModel(body)
		if gateway.isOpenAICodexTicketAccount(request.Context(), account, model) && gateway.openAICodexTicketGatedModel(model) {
			if err := gateway.applyOpenAICodexTicket(request.Context(), account, model, request.Header, "sse"); err != nil {
				return nil, err
			}
			if expected, independent := request.Context().Value(oauth429ExecutionKey{}).(*oauth429Execution); independent {
				ticket := gateway.boundCodexTicket(request, account)
				if ticket == nil || ticket.AccountID != expected.AccountID || (ticket.SourceAccountID != 0 && ticket.SourceAccountID != expected.AccountID) {
					return nil, errors.New("independent test cannot use another account's ticket")
				}
			}
			request = request.WithContext(context.WithValue(request.Context(), astraCandySkipObservationKey{}, true))
			started := time.Now().UTC()
			response, sendErr := gateway.doOpenAIUpstream(request, proxyURL, account)
			recordOAuth429TestStatus(request, account.ID, response, sendErr)
			return observeAccountTestResponse(request.Context(), started, response, sendErr)
		}
	}
	profile, err := resolveAccountTLSProfileForOpenAI(account, s.cfg)
	if err != nil {
		return nil, err
	}
	perform := func(attemptRequest *http.Request) (*http.Response, error) {
		if err := validateManagedAccountProxy(account, account.Proxy); err != nil {
			return nil, err
		}
		if gateway != nil && gateway.openaiIPv6 != nil {
			if err := gateway.openaiIPv6.CheckRoute(attemptRequest.Context(), account, proxyURL); err != nil {
				return nil, err
			}
		}
		if err := validateOpenAIIPv6Route(account, proxyURL); err != nil {
			return nil, err
		}

		_, independent429 := attemptRequest.Context().Value(oauth429ExecutionKey{}).(*oauth429Execution)
		if s.pluginManager != nil && !account.IsManagedIPv6Bound() && !account.OpenAIRequestProxyEnabled() && !independent429 {
			started := time.Now().UTC()
			response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(attemptRequest.Context(), attemptRequest, proxyURL, account)
			if handled {
				if options, ok := pelicanTestOptionsFromContext(attemptRequest.Context()); ok && options.nativeOAuthTransport != nil && !IsAstraSourceAcquisition(attemptRequest.Context()) {
					*options.nativeOAuthTransport = false
				}
				return observeAccountTestResponse(attemptRequest.Context(), started, response, err)
			}
		}
		if gateway != nil && !account.OpenAIRequestProxyEnabled() && !independent429 {
			started := time.Now().UTC()
			if response, handled, err := gateway.tryGroupSmartWS(attemptRequest, proxyURL, account); handled {
				return observeAccountTestResponse(attemptRequest.Context(), started, response, err)
			}
		}
		if options, ok := pelicanTestOptionsFromContext(attemptRequest.Context()); ok && options.nativeOAuthTransport != nil && !IsAstraSourceAcquisition(attemptRequest.Context()) {
			*options.nativeOAuthTransport = true
		}
		if profile != nil {
			return s.doAccountTestHTTPWithTLS(attemptRequest, proxyURL, account.ID, account.Mode1EffectiveConcurrency(), profile)
		}
		if useTLSFallback && s.tlsFPProfileService != nil && !account.IdentityProtectionEnabled() {
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
	if _, independent := request.Context().Value(oauth429ExecutionKey{}).(*oauth429Execution); independent || IsGatewayAccountTest(request.Context()) || request.Context().Value(generationPreviewContextKey{}) == true || request.Context().Value(accountTestUsageKey{}) != nil {
		return perform(request)
	}
	return doAccount429Retry(request, account, perform)
}
