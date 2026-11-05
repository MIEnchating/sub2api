package admin

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

func (h *AccountHandler) enrichCodexTicketStatus(account *service.Account, out *dto.Account) {
	if out == nil || account == nil || !account.IsOpenAIOAuthLike() || account.IsShadow() {
		return
	}
	bpsEnabled := true
	cfg := config.OpenAICodexTicketConfig{}
	if h != nil && h.cfg != nil {
		cfg = h.cfg.Gateway.OpenAICodexTicket
	}
	if h != nil && h.codexTicketSettings != nil {
		if account.IsExcelBPSEnabled() {
			bpsEnabled, _ = h.codexTicketSettings.GetProtocolFeatureEnabled(context.Background(), service.SettingKeyExcelBPSEnabled)
		}
		cfg.Enabled = h.codexTicketSettings.GetOpenAICodexTicketEnabled(context.Background(), cfg.Enabled)
		cfg.Models = h.codexTicketSettings.GetOpenAICodexTicketModels(context.Background(), cfg.Models)
		cfg.FailClosed = h.codexTicketSettings.GetOpenAICodexTicketFailClosed(context.Background())
	}
	policy := service.ResolveOpenAICodexTicketAccountConfig(account, cfg, bpsEnabled)
	out.CodexTicketConfig = &policy
	out.CodexTurnTickets = service.OpenAICodexTicketStatuses(account, cfg, time.Now(), bpsEnabled)
	if h != nil && h.codexTicketGateway != nil {
		h.codexTicketGateway.EnrichOpenAICodexTicketDiagnostics(account, out.CodexTurnTickets)
	}
}
