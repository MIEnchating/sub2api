package service

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInspectOpenAIToolPayloadIncludesAdditionalAndDiscoveredTools(t *testing.T) {
	body := []byte(`{"tools":[{"type":"tool_search"},{"type":"function","name":"read_file"}],"input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"codex_app","tools":[{"type":"function","name":"exec"}]}]},{"type":"tool_search_output","tools":[{"type":"function","name":"write_file"}]},{"type":"function_call","name":"read_file"},{"type":"function_call_output","call_id":"call_1"}]}`)
	got := inspectOpenAIToolPayload(body)
	require.Equal(t, []string{"codex_app__exec", "read_file", "tool_search"}, got.InboundToolNames)
	require.Equal(t, []string{"write_file"}, got.DiscoveredToolNames)
	require.True(t, got.ToolSearchDeclared)
	require.Equal(t, 1, got.InboundToolCalls)
	require.Equal(t, 2, got.InboundToolOutputs, "tool_search_output also represents a tool output")
}

func TestInspectOpenAIToolPayloadIgnoresPendingDiscoveries(t *testing.T) {
	got := inspectOpenAIToolPayload([]byte(`{"tools":[{"type":"tool_search"}],"input":[{"type":"tool_search_output","status":"in_progress","tools":[{"type":"function","name":"exec"}]}]}`))
	require.Empty(t, got.DiscoveredToolNames)
	require.Equal(t, 1, got.InboundToolOutputs)
}

func TestDiagnoseOpenAIToolPayloadDistinguishesMissingAndDroppedTools(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state openAIToolDiagnosticsState
		want  string
	}{
		{name: "client missing", state: openAIToolDiagnosticsState{}, want: "no_tools_declared"},
		{name: "gateway dropped", state: openAIToolDiagnosticsState{OpenAIToolDiagnostics: OpenAIToolDiagnostics{InboundToolNames: []string{"exec"}}}, want: "gateway_dropped_tools"},
		{name: "tools present", state: openAIToolDiagnosticsState{OpenAIToolDiagnostics: OpenAIToolDiagnostics{InboundToolNames: []string{"exec"}, OutboundToolNames: []string{"exec"}}}, want: "tools_present"},
		{name: "history without declarations", state: openAIToolDiagnosticsState{OpenAIToolDiagnostics: OpenAIToolDiagnostics{InboundToolCalls: 1}}, want: "tool_history_without_declarations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, diagnoseOpenAIToolPayload(&tc.state))
		})
	}
}

func TestDiagnoseOpenAIToolPayloadDetectsPartialToolDrop(t *testing.T) {
	state := openAIToolDiagnosticsState{OpenAIToolDiagnostics: OpenAIToolDiagnostics{
		InboundToolNames:  []string{"exec", "read_file"},
		OutboundToolNames: []string{"exec"},
	}}
	require.Equal(t, "gateway_dropped_tools", diagnoseOpenAIToolPayload(&state))
	require.Equal(t, []string{"read_file"}, state.MissingToolNames)
}

func TestRecordOpenAIToolEgressDiagnosticsStoresNamesWithoutArguments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	RecordOpenAIToolIngressDiagnostics(c, []byte(`{"tools":[{"type":"custom","name":"exec","parameters":{"secret":"omit"}}]}`), true)
	RecordOpenAIToolEgressDiagnostics(c, []byte(`{"tools":[{"type":"function","name":"exec","parameters":{"secret":"omit"}}]}`))

	got, ok := GetOpenAIToolDiagnostics(c)
	require.True(t, ok)
	require.Equal(t, []string{"exec"}, got.InboundToolNames)
	require.Equal(t, []string{"exec"}, got.OutboundToolNames)
	require.Equal(t, "tools_present", got.Diagnosis)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
}
