package service

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

// Existing account-specific policies retain their dispatcher and diagnostics.
// Configured directed harvesting and 780 tickets use the new harvest engine.
func (s *OpenAIGatewayService) legacyCodexTicketHarvest(ctx context.Context, account *Account) bool {
	if s == nil || s.openAICodexTicketConfig().TargetLength == 780 {
		return false
	}
	if s.codexHarvest != nil {
		_, saved, _ := s.codexHarvest.Controls(ctx)
		if saved {
			return false
		}
	}
	// Explicit controller settings opt existing accounts into the same round
	// budget and directed engine. Account switches still gate fresh admission;
	// their mere presence must not bypass the administrator's controls.
	if account != nil {
		if _, exists := account.Extra[OpenAICodexTicketEnabledExtraKey]; exists {
			return true
		}
	}
	return strings.TrimSpace(s.openAICodexTicketHarvestProxySetting(ctx)) == ""
}
func (s *OpenAIGatewayService) fireLegacyOpenAICodexTicketProbe(ctx context.Context, account *Account, token, model, proxyURL string, attemptTimeout time.Duration, routes ...codexTicketProxyRoute) (state string, status int, err error) {
	attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()

	body := []byte(`{"model":` + jsonString(model) + `,"store":false,"stream":true,"instructions":"Reply with exactly: pong","input":[{"role":"user","content":[{"type":"input_text","text":"ping"}]}]}`)
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, chatgptCodexURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, newOpenAICodexTicketProbeError("invalid_response", 0)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAIHarvest))
	req.Close = true
	req.Host = "chatgpt.com"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("session_id", uuid.NewString())
	if err := resolveAndSetOpenAIChatGPTAccountHeaders(attemptCtx, s.accountRepo, req.Header, account); err != nil {
		return "", 0, newOpenAICodexTicketProbeError("auth", 0)
	}
	applyOpenAICodexTicketHarvestIdentity(req.Header, model)

	// Synthetic probes must use the dedicated no-reuse transport even when the
	// production account is bound to a plugin. This also avoids reading pluginManager
	// while handlers are still wiring it during gateway construction.
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return "", 0, classifyOpenAICodexTicketProbeError(err, 0)
	}
	if resp == nil {
		return "", 0, newOpenAICodexTicketProbeError("invalid_response", 0)
	}
	// Successful probes only need headers. Error bodies are bounded diagnostics;
	// neither path reuses a connection or drains a successful generation stream.
	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, openAICodexTicketProbeResponseError(resp)
	}
	if attemptCtx.Err() == nil {
		s.captureOpenAICodexTicketCookies(attemptCtx, account, resp.Header, time.Now(), routes...)
	}
	return extractOpenAICodexTurnState(resp.Header), resp.StatusCode, nil
}

func (s *OpenAIGatewayService) hasLegacyCodexCookies(account *Account) bool {
	if account == nil {
		return false
	}
	if _, ok := account.Extra[openAICodexTicketCookiesExtraKey]; ok {
		return true
	}
	_, ok := s.openaiCodexTicketCookies.Load(account.ID)
	return ok
}

func (s *OpenAIGatewayService) requiresLegacyCodexCookies(account *Account, ticket *openAICodexTicket) bool {
	if s.hasLegacyCodexCookies(account) {
		return true
	}
	return ticket != nil && ticket.IssuedAt.IsZero() && ticket.HarvestSessionID == "" && ticket.Transport == "" && s.legacyCodexTicketHarvest(context.Background(), account)
}
