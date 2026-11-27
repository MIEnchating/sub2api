package service

import (
	"bytes"
	"strings"

	"github.com/tidwall/gjson"
)

const codexExecNotificationConversionMessage = "Codex exec notifications require a native Responses route"

func codexExecNotificationOutput(kind, name, namespace, status, output string) bool {
	return kind == "custom_tool_call_output" && name == "exec" && namespace == "" && status == "" && strings.TrimSpace(output) != ""
}

// Return call IDs whose native history contains Codex progress notifications.
// A known call from a different namespace or tool keeps its regular contract.
func codexExecNotificationCallIDs(input []any) map[string]bool {
	known := make(map[string]bool)
	var ids map[string]bool
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		id, _ := item["call_id"].(string)
		if id == "" {
			continue
		}
		kind, _ := item["type"].(string)
		name, _ := item["name"].(string)
		namespace, _ := item["namespace"].(string)
		status, _ := item["status"].(string)
		output, _ := item["output"].(string)
		switch {
		case isCodexToolCallContextItemType(kind):
			known[id] = kind == "custom_tool_call" && name == "exec" && (namespace == "" || namespace == "functions")
		case codexExecNotificationOutput(kind, name, namespace, status, output):
			if ids == nil {
				ids = make(map[string]bool)
			}
			ids[id] = true
		}
	}
	for id, isExec := range known {
		if !isExec {
			delete(ids, id)
		}
	}
	return ids
}

func hasCodexExecNotifications(body []byte) bool {
	if !bytes.Contains(body, []byte("custom_tool_call_output")) && !bytes.Contains(body, []byte(`\u`)) {
		return false
	}
	return len(rawCodexExecNotificationCallIDs(parseRawJSONView(body).Get("input"))) > 0
}

func rawCodexExecNotificationCallIDs(input gjson.Result) map[string]bool {
	if !input.IsArray() {
		return nil
	}
	var ids map[string]bool
	input.ForEach(func(_, item gjson.Result) bool {
		if id := item.Get("call_id").Str; id != "" && rawCodexExecNotificationOutput(item) {
			if ids == nil {
				ids = make(map[string]bool)
			}
			ids[id] = true
		}
		return true
	})
	if len(ids) > 0 {
		input.ForEach(func(_, item gjson.Result) bool {
			id, kind := item.Get("call_id").Str, item.Get("type").Str
			if ids[id] && isCodexToolCallContextItemType(kind) {
				namespace := item.Get("namespace").Str
				if kind != "custom_tool_call" || item.Get("name").Str != "exec" || (namespace != "" && namespace != "functions") {
					delete(ids, id)
				}
			}
			return true
		})
	}
	return ids
}

func rawCodexExecNotificationOutput(item gjson.Result) bool {
	return codexExecNotificationOutput(item.Get("type").Str, item.Get("name").Str, item.Get("namespace").Str, item.Get("status").Str, item.Get("output").Str)
}

// The ordinary result and the progress notifications share a call ID. The
// native input still needs a preceding call or an authenticated response anchor.
func ValidateCodexExecNotificationContext(body []byte) bool {
	if !bytes.Contains(body, []byte("custom_tool_call_output")) && !bytes.Contains(body, []byte(`\u`)) {
		return true
	}
	root := parseRawJSONView(body)
	hasAnchor := strings.TrimSpace(root.Get("previous_response_id").Str) != ""
	rawInput := root.Get("input")
	ids := rawCodexExecNotificationCallIDs(rawInput)
	if len(ids) == 0 {
		return true
	}
	calls := make(map[string]bool)
	results := make(map[string]bool)
	valid := true
	rawInput.ForEach(func(_, item gjson.Result) bool {
		id := item.Get("call_id").String()
		if !ids[id] {
			return true
		}
		switch item.Get("type").String() {
		case "custom_tool_call":
			if calls[id] {
				valid = false
				return false
			}
			calls[id] = true
		case "custom_tool_call_output":
			if !calls[id] && !hasAnchor {
				valid = false
				return false
			}
			if !rawCodexExecNotificationOutput(item) {
				if results[id] {
					valid = false
					return false
				}
				results[id] = true
			}
		}
		return true
	})
	return valid
}
