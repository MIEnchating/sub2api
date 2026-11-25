package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexWSAnchorRejectsLegacyFixedSession(t *testing.T) {
	account := ticketTestAccount(300)
	cfg := &config.Config{}
	wsSession := config.CodexWSAnchorConfig{Enabled: true, AccountIDs: []int64{account.ID}}
	cfg.SetAstraRoutingLoader(func(context.Context) config.AstraRoutingSettings {
		return config.AstraRoutingSettings{WSSession: wsSession}
	})
	gateway := &OpenAIGatewayService{cfg: cfg}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-6-astra"}`)

	_, finish, err := gateway.prepareCodexWSAnchor(t.Context(), c, account, body)
	require.ErrorContains(t, err, "astra_ws_anchor_unsupported")
	require.Nil(t, finish)
	require.Empty(t, gateway.codexWSAnchors.entries)

	wsSession.Enabled = false
	_, finish, err = gateway.prepareCodexWSAnchor(t.Context(), c, account, body)
	require.NoError(t, err)
	require.Nil(t, finish)
	require.Empty(t, gateway.codexWSAnchors.entries)
}

func TestCodexWSAnchorCookiePreservesIdentity(t *testing.T) {
	h := http.Header{"Cookie": []string{"__oailb=old; __cflb=target", "extra=keep; __oailb=duplicate"}, "Authorization": []string{"Bearer target"}, "X-Codex-Turn-State": []string{"target-state"}}
	replaceCodexWSAnchorCookie(h, "qualified")
	require.Equal(t, "__cflb=target; extra=keep; __oailb=qualified", h.Get("Cookie"))
	require.Equal(t, "Bearer target", h.Get("Authorization"))
	require.Equal(t, "target-state", h.Get("X-Codex-Turn-State"))
}
