//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestAllModelBPSDisabledRetainsNativeTicketPolicy(t *testing.T) {
	ctx := context.Background()
	settings := &excelBPSImageSettingsRepo{values: map[string]string{SettingKeyExcelBPSEnabled: "false"}}
	gateway := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true, Models: []string{"gpt-6-astra"}}, nil)
	gateway.settingService = NewSettingService(settings, gateway.cfg)
	account := ticketTestAccount(1)
	account.Extra = map[string]any{"openai_excel_bps": true, OpenAICodexTicketEnabledExtraKey: true, OpenAICodexTicketFailClosedExtraKey: true}
	require.True(t, gateway.openAICodexTicketAccountConfig(ctx, account).Enabled)
	require.True(t, gateway.openAICodexTicketBlocksAccount(account, "gpt-6-astra"))
	_, err := gateway.applyOpenAICodexTicketWithResult(ctx, account, "gpt-6-astra", http.Header{})
	require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
	require.Len(t, OpenAICodexTicketStatuses(account, gateway.cfg.Gateway.OpenAICodexTicket, time.Now(), false), 1)
	flow := BuildCodexHarvestFlow(ctx, gateway.cfg, gateway.settingService, []Account{*account})
	require.Len(t, flow.Accounts, 1)

	account.Extra[OpenAICodexTicketFailClosedExtraKey] = false
	require.False(t, gateway.openAICodexTicketBlocksAccount(account, "gpt-6-astra"))
	require.NoError(t, gateway.applyOpenAICodexTicket(ctx, account, "gpt-6-astra", http.Header{}))
	account.Extra[OpenAICodexTicketEnabledExtraKey] = false
	require.False(t, gateway.astraTicketAccountEligible(ctx, account))
	require.False(t, gateway.ticketPoolSchedulingIncludes(ctx, account))
	require.False(t, gateway.openAICodexTicketAccountConfig(ctx, account).Enabled)
	account.Extra[OpenAICodexTicketEnabledExtraKey] = true
	settings.values[SettingKeyExcelBPSEnabled] = "true"
	require.False(t, gateway.openAICodexTicketAccountConfig(ctx, account).Enabled)
	settings.values[SettingKeyExcelBPSEnabled] = "false"
	account.Credentials["client_id"] = openai.ExcelClientID
	require.False(t, gateway.openAICodexTicketAccountConfig(ctx, account).Enabled, "Excel primary credentials must never enter native Codex")
}

func TestAllModelBPSDisabledAllowsLegacyHarvest(t *testing.T) {
	calls := 0
	gateway := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: http.NoBody}, nil
	}})
	gateway.settingService = NewSettingService(&excelBPSImageSettingsRepo{values: map[string]string{SettingKeyExcelBPSEnabled: "false"}}, gateway.cfg)
	account := activeTicketAccounts(1)[0]
	account.Extra = map[string]any{"openai_excel_bps": true, OpenAICodexTicketEnabledExtraKey: true}
	cfg := gateway.openAICodexTicketHarvestConfig(context.Background())
	require.True(t, gateway.startOpenAICodexTicketProbe(context.Background(), &account, "gpt-6-astra", cfg, time.Now(), false))
	require.Equal(t, 1, calls)
	require.Equal(t, true, account.Extra["openai_excel_bps"], "runtime fallback must preserve account routing settings")
}
