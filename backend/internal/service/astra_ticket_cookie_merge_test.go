package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestTicketPoolSchedulingMatchesLegacyCookieAdmission(t *testing.T) {
	for _, mode := range []string{"account", "model", "groups"} {
		for _, state := range []string{"missing", "ready", "expired", "other_exit", "modern"} {
			t.Run(mode+"/"+state, func(t *testing.T) {
				gateway := ticketTestService(t, config.OpenAICodexTicketConfig{
					Enabled: true, FailClosed: true, TargetLength: 292, Models: []string{"gpt-6-astra"},
				}, nil)
				account := ticketTestAccount(1)
				now := time.Now()
				ticket := &openAICodexTicket{
					AccountID: account.ID, Model: "gpt-6-astra", State: fakeCodexTicketState(292),
					Length: 292, CapturedAt: now, ExpiresAt: now.Add(time.Minute),
				}
				if state == "modern" {
					ticket.IssuedAt = now
					ticket.Transport = "sse"
				}
				gateway.openaiCodexTickets.Store(openAICodexTicketKey(account.ID, ticket.Model), ticket)
				if state != "missing" && state != "modern" {
					cookies := &openAICodexTicketCookies{
						AccountID: account.ID, Scope: openAICodexTicketScope(account), UpdatedAt: now,
						CFLB:  openAICodexTicketCookie{Value: "cflb", CapturedAt: now, ExpiresAt: now.Add(time.Minute)},
						OAILB: openAICodexTicketCookie{Value: "oailb", CapturedAt: now, ExpiresAt: now.Add(time.Minute)},
					}
					if state == "expired" {
						cookies.CFLB.CapturedAt = now.Add(-time.Minute)
						cookies.CFLB.ExpiresAt = now.Add(-time.Second)
					}
					if state == "other_exit" {
						cookies.Scope = "another-proxy"
					}
					gateway.openaiCodexTicketCookies.Store(account.ID, cookies)
				}
				want := state == "ready" || state == "modern"
				applied, err := gateway.applyOpenAICodexTicketWithResult(t.Context(), account, ticket.Model, http.Header{})
				if want {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
				}
				require.Equal(t, want, applied)
				require.Equal(t, want, gateway.ticketPoolSchedulingReady(account, config.AstraRoutingSettings{SchedulingMode: mode}),
					"ticket scheduling must not restore an account whose credential bundle the gateway rejects")
			})
		}
	}
}
