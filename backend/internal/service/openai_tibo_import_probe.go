package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const openAITiboProbePrompt = "Based on your knowledge, do you know who Tibo is in OpenAI? Do not search online. Reply only True or False."
const openAITiboResultKey = "openai_import_tibo"

func (s *OpenAIGatewayService) probeNewOpenAIOAuthAccount(ctx context.Context, account *Account) {
	now := time.Now()
	if s.cfg == nil || !s.cfg.Gateway.OpenAITiboProbeEnabled || !shouldRefreshOpenAIQuotaSnapshots(account, now) || !account.IsSchedulable() ||
		account.IsCredentialShadow() || account.IsOpenAIAgentIdentity() || account.IsOpenAIPersonalAccessToken() ||
		account.isModelRateLimitedWithContext(ctx, openAICodexTicketDefaultModel) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, openAIQuotaRefreshTimeout)
	defer cancel()
	model := openAICodexTicketDefaultModel
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil || token == "" {
		return
	}
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+token)
	if err := resolveAndSetOpenAIChatGPTAccountHeaders(ctx, s.accountRepo, headers, account); err != nil {
		return
	}
	applyOpenAICodexTicketHarvestIdentity(headers, model)
	proxy, err := resolveOpenAIMaintenanceProxy(ctx, account, s.proxyRepo)
	if err != nil {
		return
	}
	shot, err := fireOpenAICodexPromptShotRequest(ctx, headers, model, "", "", openAITiboProbePrompt, openAITiboProbePrompt, func(req *http.Request) (*http.Response, error) {
		return s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	}, "xhigh")
	verdict := "unknown"
	if err == nil && shot.status == http.StatusOK && shot.streamErr == nil {
		verdict = "degraded"
		if upstreamModelsMatchForAudit(model, shot.model) && classifyTiboAnswer(shot.answer) == tiboAnswerTrue {
			verdict = "healthy"
		}
	}
	if ctx.Err() != nil {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{
		openAITiboResultKey: map[string]any{"verdict": verdict, "model": model, "checked_at": time.Now().UTC().Format(time.RFC3339Nano)},
	}); err != nil {
		slog.Warn("openai_import_tibo_save_failed", "account_id", account.ID)
	}
}
