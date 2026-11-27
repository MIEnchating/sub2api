package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexExecNotificationsPreserveNativeHistory(t *testing.T) {
	body := []byte(`{"input":[
		{"type":"custom_tool_call","id":"ctc_exec","call_id":"call_exec","name":"exec","input":"run checks"},
		{"type":"custom_tool_call_output","id":"cto_first","call_id":"call_exec","name":"exec","output":"started"},
		{"type":"custom_tool_call_output","id":"cto_result","call_id":"call_exec","output":"done"},
		{"type":"custom_tool_call_output","id":"cto_last","call_id":"call_exec","name":"exec","output":"report ready"}
	]}`)
	require.True(t, ValidateCodexExecNotificationContext(body))
	var request struct {
		Input []any `json:"input"`
	}
	require.NoError(t, json.Unmarshal(body, &request))
	require.Equal(t, request.Input, filterCodexInputWithOptions(request.Input, codexInputFilterOptions{}))
	sanitized, changed, err := sanitizeOpenAIResponsesInputItemIDs(body)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, body, sanitized)
	_, _, err = adaptOpenAIResponsesClientTools(body)
	require.ErrorContains(t, err, codexExecNotificationConversionMessage)

	orphan := []byte(`{"input":[{"type":"custom_tool_call_output","call_id":"call_exec","name":"exec","output":"started"}]}`)
	require.False(t, ValidateCodexExecNotificationContext(orphan))
	duplicate := []byte(`{"input":[{"type":"custom_tool_call","call_id":"call_exec","name":"exec","input":"run checks"},{"type":"custom_tool_call_output","call_id":"call_exec","name":"exec","output":"started"},{"type":"custom_tool_call_output","call_id":"call_exec","output":"done"},{"type":"custom_tool_call_output","call_id":"call_exec","output":"done again"}]}`)
	require.False(t, ValidateCodexExecNotificationContext(duplicate))
}

func TestCodexExecWebSocketCompletionBindsHTTPResponseOwner(t *testing.T) {
	ctx := context.Background()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	groupID := int64(15)
	c.Set("api_key", &APIKey{GroupID: &groupID})
	SetOpenAIWSResponseOwner(c, 30, 40)
	gateway := &OpenAIGatewayService{}
	account := &Account{ID: 50, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	gateway.bindWebSocketResponseForHTTP(ctx, c, account, []byte(`{"type":"response.completed","response":{"id":"resp_ws","status":"completed"}}`))
	ok, err := gateway.ValidateOpenAIHTTPResponseOwner(ctx, groupID, "resp_ws", 30, 40)
	require.NoError(t, err)
	require.True(t, ok)
	id, err := gateway.getOpenAIWSStateStore().GetResponseAccount(ctx, groupID, "resp_ws")
	require.NoError(t, err)
	require.Equal(t, int64(50), id)

	SetOpenAIWSResponseOwner(c, 31, 41)
	gateway.bindWebSocketResponseForHTTP(ctx, c, &Account{ID: 51, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, []byte(`{"type":"response.completed","response":{"id":"resp_ws","status":"completed"}}`))
	ok, err = gateway.ValidateOpenAIHTTPResponseOwner(ctx, groupID, "resp_ws", 31, 41)
	require.NoError(t, err)
	require.False(t, ok)
	id, err = gateway.getOpenAIWSStateStore().GetResponseAccount(ctx, groupID, "resp_ws")
	require.NoError(t, err)
	require.Equal(t, int64(50), id)

	gateway.bindWebSocketResponseForHTTP(ctx, c, account, []byte(`{"type":"response.failed","response":{"id":"resp_failed","status":"failed"}}`))
	ok, err = gateway.ValidateOpenAIHTTPResponseOwner(ctx, groupID, "resp_failed", 31, 41)
	require.NoError(t, err)
	require.False(t, ok)
}
