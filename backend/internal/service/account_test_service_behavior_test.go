package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func requirePayloadItems(t *testing.T, value any) []map[string]any {
	t.Helper()
	items, ok := value.([]map[string]any)
	require.True(t, ok)
	return items
}

func TestCreateClaudeBehaviorTestPayloadUsesDetectionPrompt(t *testing.T) {
	payload, err := createClaudeAccountTestPayload("claude-opus-5", "return the probe contract", AccountTestModeBehavior)
	require.NoError(t, err)

	messages := requirePayloadItems(t, payload["messages"])
	content := requirePayloadItems(t, messages[0]["content"])
	system := requirePayloadItems(t, payload["system"])
	require.Equal(t, "return the probe contract", content[0]["text"])
	require.Equal(t, accountTestBehaviorInstructions, system[0]["text"])
	require.Equal(t, 0, payload["temperature"])
}

func TestCreateOpenAIBehaviorTestPayloadUsesDetectionPrompt(t *testing.T) {
	payload := createOpenAIAccountTestPayload("gpt-5.6-sol", false, "return the probe contract", AccountTestModeBehavior)

	input := requirePayloadItems(t, payload["input"])
	content := requirePayloadItems(t, input[0]["content"])
	require.Equal(t, "return the probe contract", content[0]["text"])
	require.Equal(t, accountTestBehaviorInstructions, payload["instructions"])
	require.Equal(t, 0, payload["temperature"])
}

func TestDefaultAccountTestPayloadsRemainUnchanged(t *testing.T) {
	claudePayload, err := createClaudeAccountTestPayload("claude-opus-5", "ignored", AccountTestModeDefault)
	require.NoError(t, err)
	claudeMessages := requirePayloadItems(t, claudePayload["messages"])
	claudeContent := requirePayloadItems(t, claudeMessages[0]["content"])
	require.Equal(t, "hi", claudeContent[0]["text"])
	require.Equal(t, 1, claudePayload["temperature"])

	openAIPayload := createOpenAIAccountTestPayload("gpt-5.6", false, "ignored", AccountTestModeDefault)
	openAIInput := requirePayloadItems(t, openAIPayload["input"])
	openAIContent := requirePayloadItems(t, openAIInput[0]["content"])
	require.Equal(t, "hi", openAIContent[0]["text"])
	require.NotContains(t, openAIPayload, "temperature")
}
