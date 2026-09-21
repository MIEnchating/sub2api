package service

import (
	"context"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// OpenAIToolDiagnosticsKey holds request-local inventories. Only names and
// counters are retained, never arguments, schemas, messages or tool output.
const OpenAIToolDiagnosticsKey = "openai_tool_diagnostics"

type OpenAIToolDiagnostics struct {
	InboundToolNames            []string `json:"inbound_tool_names,omitempty"`
	OutboundToolNames           []string `json:"outbound_tool_names,omitempty"`
	DiscoveredToolNames         []string `json:"discovered_tool_names,omitempty"`
	OutboundDiscoveredToolNames []string `json:"outbound_discovered_tool_names,omitempty"`
	MissingToolNames            []string `json:"missing_tool_names,omitempty"`
	InboundToolCalls            int      `json:"inbound_tool_calls,omitempty"`
	InboundToolOutputs          int      `json:"inbound_tool_outputs,omitempty"`
	ToolSearchDeclared          bool     `json:"tool_search_declared,omitempty"`
	HasPreviousResponse         bool     `json:"has_previous_response,omitempty"`
	ToolsDisabled               bool     `json:"tools_disabled,omitempty"`
	CodexClient                 bool     `json:"codex_client,omitempty"`
	Diagnosis                   string   `json:"diagnosis,omitempty"`
}

type openAIToolDiagnosticsState struct {
	OpenAIToolDiagnostics
	attempt int
}

func RecordOpenAIToolIngressDiagnostics(c *gin.Context, body []byte, codexClient bool) {
	if c == nil {
		return
	}
	state := &openAIToolDiagnosticsState{OpenAIToolDiagnostics: inspectOpenAIToolPayload(body)}
	state.CodexClient = codexClient
	c.Set(OpenAIToolDiagnosticsKey, state)
}

// RecordOpenAIToolEgressDiagnostics compares declarations at a forwarding
// boundary. It observes the request; it cannot verify the client's executors or
// infer what tools a user intended to install. Retries emit separate events.
func RecordOpenAIToolEgressDiagnostics(c *gin.Context, body []byte) {
	if c == nil {
		return
	}
	value, _ := c.Get(OpenAIToolDiagnosticsKey)
	previous, ok := value.(*openAIToolDiagnosticsState)
	if !ok || previous == nil {
		return // This forwarding path has no client inventory to compare.
	}
	state := *previous
	outbound := inspectOpenAIToolPayload(body)
	state.OutboundToolNames = outbound.InboundToolNames
	state.OutboundDiscoveredToolNames = outbound.DiscoveredToolNames
	state.ToolsDisabled = outbound.ToolsDisabled
	state.attempt++

	// Compare original identities while leaving wire names visible in the log.
	comparable := state
	comparable.OutboundToolNames = originalOpenAIToolDiagnosticNames(c, state.OutboundToolNames)
	comparable.OutboundDiscoveredToolNames = originalOpenAIToolDiagnosticNames(c, state.OutboundDiscoveredToolNames)
	state.Diagnosis = diagnoseOpenAIToolPayload(&comparable)
	state.MissingToolNames = comparable.MissingToolNames
	c.Set(OpenAIToolDiagnosticsKey, &state)
	if !state.CodexClient && !hasOpenAIToolSignal(state) {
		return
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	logger.FromContext(ctx).With(
		zap.String("component", "service.openai_tool_diagnostics"),
		zap.String("diagnosis", state.Diagnosis),
		zap.Int("attempt", state.attempt),
		zap.Bool("codex_client", state.CodexClient),
		zap.Strings("inbound_tool_names", state.InboundToolNames),
		zap.Strings("outbound_tool_names", state.OutboundToolNames),
		zap.Strings("discovered_tool_names", state.DiscoveredToolNames),
		zap.Strings("outbound_discovered_tool_names", state.OutboundDiscoveredToolNames),
		zap.Strings("missing_tool_names", state.MissingToolNames),
		zap.Int("inbound_tool_calls", state.InboundToolCalls),
		zap.Int("inbound_tool_outputs", state.InboundToolOutputs),
		zap.Bool("tool_search_declared", state.ToolSearchDeclared),
		zap.Bool("has_previous_response", state.HasPreviousResponse),
		zap.Bool("tools_disabled", state.ToolsDisabled),
	).Info("openai_tool_diagnostics")
}

func GetOpenAIToolDiagnostics(c *gin.Context) (OpenAIToolDiagnostics, bool) {
	if c == nil {
		return OpenAIToolDiagnostics{}, false
	}
	value, _ := c.Get(OpenAIToolDiagnosticsKey)
	state, ok := value.(*openAIToolDiagnosticsState)
	if !ok || state == nil {
		return OpenAIToolDiagnostics{}, false
	}
	return state.OpenAIToolDiagnostics, true
}

func inspectOpenAIToolPayload(body []byte) OpenAIToolDiagnostics {
	// Read only inventory fields. A full JSON unmarshal would allocate another
	// copy of potentially large image/file/tool-output history on every turn.
	root := parseRawJSONView(body)
	if !root.IsObject() || !gjson.ValidBytes(body) {
		return OpenAIToolDiagnostics{Diagnosis: "invalid_payload"}
	}
	result := OpenAIToolDiagnostics{
		HasPreviousResponse: strings.TrimSpace(root.Get("previous_response_id").String()) != "",
		ToolsDisabled:       root.Get("tool_choice").String() == "none",
	}
	names := make(map[string]struct{})
	discovered := make(map[string]struct{})
	var addTool func(gjson.Result, map[string]struct{}, string)
	addTool = func(tool gjson.Result, into map[string]struct{}, namespace string) {
		if !tool.IsObject() {
			return
		}
		typ := strings.TrimSpace(tool.Get("type").String())
		name := strings.TrimSpace(tool.Get("name").String())
		if typ == "namespace" {
			if name == "" {
				return
			}
			children := tool.Get("tools")
			if !children.IsArray() || children.Raw == "[]" {
				children = tool.Get("children")
			}
			if children.IsArray() {
				children.ForEach(func(_, child gjson.Result) bool {
					addTool(child, into, name)
					return true
				})
			}
			return
		}
		if typ == "function" && name == "" {
			name = strings.TrimSpace(tool.Get("function.name").String())
		}
		if typ == "tool_search" {
			name = "tool_search" // Also the proxy function name after conversion.
		}
		if name == "" && typ != "function" && typ != "custom" {
			name = typ // Built-in tools such as web_search have no name field.
			if label := tool.Get("server_label").String(); typ == "mcp" && label != "" {
				name += ":" + label
			}
		}
		if name != "" {
			if namespace != "" {
				name = openAIToolDiagnosticNamespaceName(namespace, name)
			}
			// The raw JSON view borrows body memory; keep only the small name.
			into[strings.Clone(name)] = struct{}{}
		}
	}
	collect := func(tools gjson.Result, into map[string]struct{}, declarations bool) {
		if tools.IsArray() {
			tools.ForEach(func(_, tool gjson.Result) bool {
				if declarations && tool.Get("type").String() == "tool_search" {
					result.ToolSearchDeclared = true
				}
				addTool(tool, into, "")
				return true
			})
		}
	}
	collect(root.Get("tools"), names, true)
	input := root.Get("input")
	if input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			switch strings.TrimSpace(item.Get("type").String()) {
			case "additional_tools":
				collect(item.Get("tools"), names, true)
			case "tool_search_output":
				result.InboundToolOutputs++
				status := item.Get("status")
				if !status.Exists() || status.String() == "completed" {
					collect(item.Get("tools"), discovered, false)
				}
			case "function_call", "custom_tool_call", "tool_search_call":
				result.InboundToolCalls++
			case "function_call_output", "custom_tool_call_output":
				result.InboundToolOutputs++
			}
			return true
		})
	}
	result.InboundToolNames = sortedToolNames(names)
	result.DiscoveredToolNames = sortedToolNames(discovered)
	return result
}

// Use the protocol adapter's naming rule, including its hash for long names.
func openAIToolDiagnosticNamespaceName(namespace, name string) string {
	mapping := apicompat.NamespaceToolNames([]apicompat.ResponsesTool{{
		Type: "namespace", Name: namespace,
		Tools: []apicompat.ResponsesTool{{Type: "function", Name: name}},
	}})
	for flat := range mapping {
		return flat
	}
	return name
}

func originalOpenAIToolDiagnosticNames(c *gin.Context, names []string) []string {
	reverse := codexToolNameReverseFromContext(c)
	mapping, _ := openAIResponsesClientToolMapping(c)
	grokMapping, _ := grokResponsesClientToolMapping(c)
	namespaces := openAIResponsesNamespaceNames(c)
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		if original, ok := reverse[name]; ok {
			name = original
		} else {
			identity, found := namespaces[name]
			if !found {
				identity, found = mapping.NamespaceTools[name]
			}
			if !found {
				identity, found = grokMapping.NamespaceTools[name]
			}
			if found {
				child := identity.Name
				if original, ok := reverse[child]; ok {
					child = original
				}
				name = openAIToolDiagnosticNamespaceName(identity.Namespace, child)
			} else {
				for alias, original := range reverse {
					if strings.HasSuffix(name, "__"+alias) {
						name = strings.TrimSuffix(name, alias) + original
						break
					}
				}
			}
		}
		result[name] = struct{}{}
	}
	return sortedToolNames(result)
}

func diagnoseOpenAIToolPayload(state *openAIToolDiagnosticsState) string {
	if len(state.DiscoveredToolNames) > 0 && !state.ToolSearchDeclared {
		return "client_discovered_tools_not_declared"
	}
	state.MissingToolNames = missingOpenAIToolNames(state.OpenAIToolDiagnostics)
	if state.Diagnosis == "invalid_payload" {
		return "invalid_payload"
	}
	if len(state.MissingToolNames) > 0 {
		for _, name := range state.MissingToolNames {
			for _, discovered := range state.DiscoveredToolNames {
				if name == discovered {
					return "gateway_dropped_discovered_tools"
				}
			}
		}
		return "gateway_dropped_tools"
	}
	if state.ToolsDisabled {
		return "tools_disabled"
	}
	if len(state.OutboundToolNames)+len(state.OutboundDiscoveredToolNames) == 0 {
		if state.HasPreviousResponse {
			return "continuation_context_not_inspected"
		}
		if state.InboundToolCalls > 0 || state.InboundToolOutputs > 0 {
			return "tool_history_without_declarations"
		}
		return "no_tools_declared"
	}
	return "tools_present"
}

func missingOpenAIToolNames(state OpenAIToolDiagnostics) []string {
	available := make(map[string]struct{})
	for _, names := range [][]string{state.OutboundToolNames, state.OutboundDiscoveredToolNames} {
		for _, name := range names {
			available[name] = struct{}{}
		}
	}
	missing := make(map[string]struct{})
	for _, names := range [][]string{state.InboundToolNames, state.DiscoveredToolNames} {
		for _, name := range names {
			if _, ok := available[name]; !ok {
				missing[name] = struct{}{}
			}
		}
	}
	return sortedToolNames(missing)
}

func hasOpenAIToolSignal(state openAIToolDiagnosticsState) bool {
	return len(state.InboundToolNames)+len(state.OutboundToolNames)+len(state.DiscoveredToolNames)+len(state.OutboundDiscoveredToolNames) > 0 ||
		state.InboundToolCalls > 0 || state.InboundToolOutputs > 0 || state.ToolSearchDeclared
}

func sortedToolNames(names map[string]struct{}) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
