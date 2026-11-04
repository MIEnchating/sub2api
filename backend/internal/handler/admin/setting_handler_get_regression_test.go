package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetSettingsPreservesSecurityGatewayAndProxyConfiguration(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeySiteName:                         "Configured gateway",
		service.SettingKeySessionBindingEnabled:            "true",
		service.SettingKeyStepUpEnabled:                    "true",
		service.SettingKeySMTPHost:                         "smtp.example.invalid",
		service.SettingKeySMTPPassword:                     "smtp-test-secret",
		service.SettingKeyDefaultBalance:                   "12.5",
		service.SettingKeyOpenAIFirstOutputTimeoutSeconds:  "90",
		service.SettingKeyOpenAICodexTicketEnabled:         "true",
		service.SettingKeyOpenAICodexTicketHarvestProxyURL: "http://user:proxy-test-secret@proxy.example.invalid:8080",
		service.SettingKeyOpenAICodexTicketStaticProxyURL:  "http://user:static-test-secret@static.example.invalid:8080",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data dto.SystemSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	got := envelope.Data
	require.Equal(t, "Configured gateway", got.SiteName)
	require.True(t, got.SessionBindingEnabled)
	require.True(t, got.StepUpEnabled)
	require.Equal(t, "smtp.example.invalid", got.SMTPHost)
	require.True(t, got.SMTPPasswordConfigured)
	require.Equal(t, 12.5, got.DefaultBalance)
	require.Equal(t, 90, got.OpenAIFirstOutputTimeoutSeconds)
	require.True(t, got.OpenAICodexTicketEnabled)
	require.True(t, got.OpenAICodexTicketHarvestProxyConfigured)
	require.Contains(t, got.OpenAICodexTicketHarvestProxyURL, "proxy.example.invalid")
	require.Contains(t, got.OpenAICodexTicketStaticProxyURL, "static.example.invalid")
	for _, secret := range []string{"smtp-test-secret", "proxy-test-secret", "static-test-secret"} {
		require.NotContains(t, rec.Body.String(), secret)
	}
}
