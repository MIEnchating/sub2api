//go:build unit

package service

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAntigravityConnectionTestModel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "claude-sonnet-4-6", antigravityConnectionTestModel(""))
	require.Equal(t, "gemini-3.1-pro-preview", antigravityConnectionTestModel("gemini-3.1-pro-preview"))
}

func TestExtractAntigravityTestContent_ExtractsCompletedTextAndInlineImages(t *testing.T) {
	t.Parallel()

	sse := "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Here is your image\"},{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"iVBORw0KGgo=\"}}]},\"finishReason\":\"STOP\"}]}}\n\n"
	text, images, err := extractAntigravityTestContent([]byte(sse))
	require.NoError(t, err)
	require.Equal(t, "Here is your image", text)
	require.Len(t, images, 1)
	require.Equal(t, "image/png", images[0].MimeType)
	require.Equal(t, "data:image/png;base64,iVBORw0KGgo=", images[0].ImageURL)
}

func TestAntigravityImageProbeRequiresCompleteResponse(t *testing.T) {
	for _, terminal := range []struct {
		name   string
		ending string
		valid  bool
	}{
		{"complete", `,"finishReason":"STOP"`, true},
		{"truncated", `,"finishReason":"MAX_TOKENS"`, false},
		{"unterminated", "", false},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			body := []byte(`data: {"response":{"candidates":[{"content":{"parts":[{"text":"image result"},{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}` + terminal.ending + `}]}}`)
			text, images, err := extractAntigravityTestContent(body)
			if !terminal.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "image result", text)
			require.Len(t, images, 1)
			require.Equal(t, "data:image/png;base64,aW1hZ2U=", images[0].ImageURL)
		})
	}
}

// Every other platform's connection test sends the admin's prompt (default "hi") and lets the
// model answer. The Antigravity text test sent "." with maxOutputTokens 1, ignoring the prompt,
// so thinking models (gemini-4-argon, gemini-3.8-flash) spent that token thinking and the test
// showed "sent: hi" with an empty response.
func TestAntigravityBuildGeminiTestRequest_UsesPromptAndRoomToAnswer(t *testing.T) {
	t.Parallel()

	svc := &AntigravityGatewayService{}
	for _, tc := range []struct{ prompt, want string }{{"hello there", "hello there"}, {"", "hi"}, {"   ", "hi"}} {
		raw, err := svc.buildGeminiTestRequestWithPrompt("proj-123", "gemini-4-argon-high", tc.prompt)
		require.NoError(t, err)
		var v1Req map[string]any
		require.NoError(t, json.Unmarshal(raw, &v1Req))
		inner := v1Req["request"].(map[string]any)
		parts := inner["contents"].([]any)[0].(map[string]any)["parts"].([]any)
		require.Equal(t, tc.want, parts[0].(map[string]any)["text"], "prompt %q", tc.prompt)
		genCfg := inner["generationConfig"].(map[string]any)
		require.EqualValues(t, accountTestMaxOutputTokens(tc.prompt, antigravityTestMaxOutputTokens), genCfg["maxOutputTokens"])
		require.GreaterOrEqual(t, antigravityTestMaxOutputTokens, 128, "a thinking model needs room to answer")
	}
}

func TestAntigravityBuildClaudeTestRequest_UsesPromptAndRoomToAnswer(t *testing.T) {
	t.Parallel()

	svc := &AntigravityGatewayService{}
	raw, err := svc.buildClaudeTestRequest("proj-123", "claude-sonnet-4-6", "hello there")
	require.NoError(t, err)
	require.Contains(t, string(raw), "hello there")
	require.NotContains(t, string(raw), `"text":"."`)

	raw, err = svc.buildClaudeTestRequest("proj-123", "claude-sonnet-4-6", "")
	require.NoError(t, err)
	require.Contains(t, string(raw), `"hi"`)
}
