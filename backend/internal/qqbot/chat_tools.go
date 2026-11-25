package qqbot

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

var errChatActionMissing = errors.New("模型未返回工具操作")

type chatTurnResult struct {
	Text          string
	SiteQuery     *siteQuery
	InputTokens   int64
	OutputTokens  int64
	ChatExecution ChatExecution
}

func wrapChatTool(protocol string, function map[string]any) map[string]any {
	if protocol == "responses" {
		function["type"] = "function"
		return function
	}
	return map[string]any{"type": "function", "function": function}
}

type chatToolCall struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatToolResponse struct {
	Status            string          `json:"status"`
	Error             json.RawMessage `json:"error"`
	IncompleteDetails json.RawMessage `json:"incomplete_details"`
	Choices           []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Output []struct {
		Type      string `json:"type"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"output"`
	Usage struct {
		Input      int64 `json:"input_tokens"`
		Output     int64 `json:"output_tokens"`
		Prompt     int64 `json:"prompt_tokens"`
		Completion int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func parseChatTurnWith(raw []byte, ct string, validate func(chatTurnResult, []chatToolCall) (chatTurnResult, error)) (chatTurnResult, error) {
	if strings.Contains(ct, "text/event-stream") {
		return parseChatToolStreamWith(raw, validate)
	}
	var v chatToolResponse
	if json.Unmarshal(raw, &v) != nil {
		return chatTurnResult{}, chatResponseError{Code: "malformed_response", Message: "模型响应格式不兼容", Retryable: true}
	}
	in, out := v.Usage.Input, v.Usage.Output
	if in == 0 {
		in = v.Usage.Prompt
	}
	if out == 0 {
		out = v.Usage.Completion
	}
	result := chatTurnResult{InputTokens: in, OutputTokens: out}
	if v.Status == "failed" || v.Status == "incomplete" || (len(v.Error) > 0 && string(v.Error) != "null") {
		return result, chatCompletionFailure(v.Status, v.Error, v.IncompleteDetails)
	}
	var calls []chatToolCall
	if len(v.Choices) > 0 {
		choice := v.Choices[0]
		if choice.FinishReason == "length" || choice.FinishReason == "content_filter" {
			details, _ := json.Marshal(map[string]string{"reason": choice.FinishReason})
			return result, chatCompletionFailure("incomplete", nil, details)
		}
		if len(choice.Message.ToolCalls) > 0 && choice.FinishReason != "tool_calls" {
			return result, incompleteChatResponse("模型工具调用未完成")
		}
		calls = append(calls, choice.Message.ToolCalls...)
	}
	for _, item := range v.Output {
		if item.Type == "function_call" {
			if v.Status != "completed" {
				return result, incompleteChatResponse("模型工具调用未完成")
			}
			call := chatToolCall{Type: "function"}
			call.Function.Name, call.Function.Arguments = item.Name, item.Arguments
			calls = append(calls, call)
		}
	}
	if len(calls) > 0 {
		return validate(result, calls)
	}
	return result, errChatActionMissing
}

func parseChatToolStreamWith(raw []byte, validate func(chatTurnResult, []chatToolCall) (chatTurnResult, error)) (chatTurnResult, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var calls []chatToolCall
	var completed json.RawMessage
	var result chatTurnResult
	done, toolFinished, responseTool, responseStream := false, false, false, false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			done = true
			continue
		}
		var event struct {
			Type     string                `json:"type"`
			Code     string                `json:"code"`
			Error    json.RawMessage       `json:"error"`
			Response json.RawMessage       `json:"response"`
			Item     struct{ Type string } `json:"item"`
			Choices  []struct {
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
				Delta        struct {
					ToolCalls []chatToolCall `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage struct {
				Prompt     int64 `json:"prompt_tokens"`
				Completion int64 `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			return result, chatResponseError{Code: "malformed_response", Message: "模型流式响应格式不兼容", Retryable: true}
		}
		if event.Type == "error" || event.Type == "response.failed" || event.Type == "response.incomplete" || (len(event.Error) > 0 && string(event.Error) != "null") {
			var response struct {
				Status  string          `json:"status"`
				Error   json.RawMessage `json:"error"`
				Details json.RawMessage `json:"incomplete_details"`
			}
			_ = json.Unmarshal(event.Response, &response)
			if len(response.Error) == 0 {
				response.Error = event.Error
			}
			if (len(response.Error) == 0 || string(response.Error) == "null") && event.Code != "" {
				response.Error, _ = json.Marshal(map[string]string{"code": event.Code})
			}
			if response.Status == "" && event.Type == "response.incomplete" {
				response.Status = "incomplete"
			}
			return result, chatCompletionFailure(response.Status, response.Error, response.Details)
		}
		if event.Type == "response.completed" {
			completed = event.Response
		}
		if strings.HasPrefix(event.Type, "response.") {
			responseStream = true
		}
		if event.Item.Type == "function_call" || strings.HasPrefix(event.Type, "response.function_call_arguments.") {
			responseTool = true
		}
		if event.Usage.Prompt > 0 || event.Usage.Completion > 0 {
			result.InputTokens, result.OutputTokens = event.Usage.Prompt, event.Usage.Completion
		}
		for _, choice := range event.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.FinishReason == "length" || choice.FinishReason == "content_filter" {
				details, _ := json.Marshal(map[string]string{"reason": choice.FinishReason})
				return result, chatCompletionFailure("incomplete", nil, details)
			}
			if choice.FinishReason == "tool_calls" {
				toolFinished = true
			}
			for _, delta := range choice.Delta.ToolCalls {
				if delta.Index != 0 {
					return result, errors.New("模型返回了多个工具调用")
				}
				if len(calls) == 0 {
					calls = append(calls, chatToolCall{})
				}
				if delta.Type != "" {
					if calls[0].Type != "" && calls[0].Type != delta.Type {
						return result, errors.New("模型返回了不支持的工具调用")
					}
					calls[0].Type = delta.Type
				}
				calls[0].Function.Name += delta.Function.Name
				calls[0].Function.Arguments += delta.Function.Arguments
			}
		}
	}
	if scanner.Err() != nil {
		return result, incompleteChatResponse("模型流式响应不完整")
	}
	if len(completed) > 0 {
		return parseChatTurnWith(completed, "application/json", validate)
	}
	if responseTool || responseStream || !done || (len(calls) > 0 && !toolFinished) {
		return result, incompleteChatResponse("模型工具调用未完成")
	}
	if len(calls) > 0 {
		return validate(result, calls)
	}
	return result, errChatActionMissing
}
