package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func toolTestResponse(protocol, name, args string) map[string]any {
	if protocol == "responses" {
		return map[string]any{"status": "completed", "output": []any{map[string]any{"type": "function_call", "name": name, "arguments": args, "call_id": "call-1"}}, "usage": map[string]int{"input_tokens": 11, "output_tokens": 7}}
	}
	return map[string]any{"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": map[string]any{"content": nil, "tool_calls": []any{map[string]any{"id": "call-1", "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}}}, "usage": map[string]int{"prompt_tokens": 11, "completion_tokens": 7}}
}

func toolTestJSON(t *testing.T, protocol, name, args string) []byte {
	t.Helper()
	raw, err := json.Marshal(toolTestResponse(protocol, name, args))
	require.NoError(t, err)
	return raw
}

func toolTestStream(t *testing.T, protocol, name, args string) []byte {
	t.Helper()
	if protocol == "responses" {
		return []byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\"}}\n\ndata: {\"type\":\"response.completed\",\"response\":" + string(toolTestJSON(t, protocol, name, args)) + "}\n\n")
	}
	var b strings.Builder
	runes := []rune(args)
	for i, part := range []string{string(runes[:len(runes)/2]), string(runes[len(runes)/2:])} {
		fn := map[string]string{"arguments": part}
		call := map[string]any{"index": 0, "function": fn}
		if i == 0 {
			call["id"], call["type"], fn["name"] = "call-1", "function", name
		}
		event, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{call}}}}})
		require.NoError(t, err)
		fmt.Fprintf(&b, "data: %s\n\n", event)
	}
	_, _ = b.WriteString("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7}}\n\ndata: [DONE]\n\n")
	return []byte(b.String())
}

func siteQueryTestResponse(t *testing.T, protocol, args string, stream bool) []byte {
	t.Helper()
	if stream {
		return toolTestStream(t, protocol, siteQueryToolName, args)
	}
	return toolTestJSON(t, protocol, siteQueryToolName, args)
}

func TestSiteChatSelectsReadOnlyQueryAcrossProtocols(t *testing.T) {
	for _, protocol := range []string{"chat_completions", "responses"} {
		for _, images := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				t.Run(protocol+"/"+boolText(images)+"/"+boolText(stream), func(t *testing.T) {
					calls := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						var body map[string]any
						require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

						require.Equal(t, "auto", body["tool_choice"])
						require.Equal(t, false, body["parallel_tool_calls"])
						tools, err := json.Marshal(body["tools"])
						require.NoError(t, err)
						require.Contains(t, string(tools), siteQueryToolName)

						require.NotContains(t, string(tools), "generate_image")
						require.NotContains(t, string(tools), "edit_image")
						require.NotContains(t, string(tools), "image_plan")
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
						}
						_, _ = w.Write(siteQueryTestResponse(t, protocol, `{"kind":"model_price","model":"model-test","activity":"","page":1}`, stream))
					}))
					defer server.Close()
					s := newMatureFixtureService(nil, nil, false, nil)
					c := DefaultConfig()
					c.BaseURL, c.ChatProtocol = server.URL, protocol
					message := Message{Role: "user", Content: "model-test多少钱？"}
					if images {
						message.Images = []string{probeImage()}
					}
					turn, err := s.siteChatTurn(context.Background(), c, []Message{message})
					require.NoError(t, err)
					require.Equal(t, &siteQuery{Kind: "model_price", Model: "model-test", Page: 1}, turn.SiteQuery)
					require.EqualValues(t, 11, turn.InputTokens)
					require.Equal(t, 1, calls)
				})
			}
		}
	}
}

func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func TestSiteChatPlainTextUsesSingleRequest(t *testing.T) {
	for _, protocol := range []string{"chat_completions", "responses"} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if protocol == "responses" {
					_, _ = w.Write([]byte(`{"status":"completed","output_text":"你好！","usage":{"input_tokens":5,"output_tokens":2}}`))
				} else {
					_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"你好！"}}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`))
				}
			}))
			defer server.Close()
			c := DefaultConfig()
			c.BaseURL, c.ChatProtocol = server.URL, protocol
			s := newMatureFixtureService(nil, nil, false, nil)
			turn, err := s.siteChatTurn(context.Background(), c, []Message{{Role: "user", Content: "你好"}})
			require.NoError(t, err)
			require.Equal(t, "你好！", turn.Text)
			require.Nil(t, turn.SiteQuery)
			require.EqualValues(t, 5, turn.InputTokens)
			require.Equal(t, 1, calls)
		})
	}
}

func TestSiteChatRejectsForgedIdentityPartialAndDisabledImageTools(t *testing.T) {
	for name, raw := range map[string][]byte{
		"forged user":        siteQueryTestResponse(t, "chat_completions", `{"kind":"model_price","model":"model-test","activity":"","page":1,"user_id":7}`, false),
		"forged account":     siteQueryTestResponse(t, "chat_completions", `{"kind":"account_balance","model":"","activity":"","page":1,"member_id":"other-user"}`, false),
		"forged email":       siteQueryTestResponse(t, "chat_completions", `{"kind":"account_balance","model":"","activity":"","page":1,"email":"other@example.com"}`, false),
		"account parameters": siteQueryTestResponse(t, "chat_completions", `{"kind":"account_balance","model":"other-user","activity":"","page":1}`, false),
		"unknown kind":       siteQueryTestResponse(t, "chat_completions", `{"kind":"transfer","model":"","activity":"","page":1}`, false),
		"partial stream":     []byte(strings.ReplaceAll(string(siteQueryTestResponse(t, "chat_completions", `{"kind":"channel_status","model":"","activity":"","page":1}`, true)), "data: [DONE]", "")),
		"disabled image":     toolTestJSON(t, "chat_completions", "generate_image", `{"prompt":"not permitted"}`),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if name == "partial stream" {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				_, _ = w.Write(raw)
			}))
			defer server.Close()
			c := DefaultConfig()
			c.BaseURL = server.URL
			turn, err := newMatureFixtureService(nil, nil, false, nil).siteChatTurn(context.Background(), c, []Message{{Role: "user", Content: "查站点"}})
			require.Error(t, err)
			require.Nil(t, turn.SiteQuery)
			require.Equal(t, 1, calls, "incomplete generation must not replay a submitted POST")
		})
	}
}

func TestSiteChatSelectsPersonalQueryAcrossProtocols(t *testing.T) {
	for _, protocol := range []string{"chat_completions", "responses"} {
		for _, tc := range []struct{ kind, question string }{
			{"account_balance", "帮我看一下我的余额"},
			{"today_usage", "我今天用了多少"},
			{"lottery_chances", "我还有几次抽奖"},
		} {
			t.Run(protocol+"/"+tc.kind, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					encoded, err := json.Marshal(body)
					require.NoError(t, err)
					require.Contains(t, string(encoded), tc.question)
					require.Contains(t, string(encoded), "不要求用户重新发送固定查询指令")
					for _, kind := range []string{"account_balance", "today_usage", "lottery_chances"} {
						require.Contains(t, string(encoded), kind)
					}
					args, err := json.Marshal(siteQuery{Kind: tc.kind, Page: 1})
					require.NoError(t, err)
					_, _ = w.Write(siteQueryTestResponse(t, protocol, string(args), false))
				}))
				defer server.Close()
				c := DefaultConfig()
				c.BaseURL, c.ChatProtocol = server.URL, protocol
				turn, err := newMatureFixtureService(nil, nil, false, nil).siteChatTurn(context.Background(), c, []Message{{Role: "user", Content: tc.question}})
				require.NoError(t, err)
				require.Equal(t, &siteQuery{Kind: tc.kind, Page: 1}, turn.SiteQuery)
				require.Empty(t, turn.Text, "the backend returns personal data directly")
				require.Equal(t, 1, calls)
			})
		}
	}
}
