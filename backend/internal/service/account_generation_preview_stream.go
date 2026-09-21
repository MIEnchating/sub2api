package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type generationPreviewEvent struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta"`
	Text     string          `json:"text"`
	Error    json.RawMessage `json:"error"`
	Response *struct {
		Status string          `json:"status"`
		Model  string          `json:"model"`
		Error  json.RawMessage `json:"error"`
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	} `json:"response"`
}

func readGenerationPreview(body io.Reader) (string, string, error) {
	limited := &io.LimitedReader{R: body, N: maximumPreviewBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), maximumPreviewBytes)
	var text strings.Builder
	var data []string
	var model string
	done := false
	process := func() error {
		if len(data) == 0 {
			return nil
		}
		raw := strings.Join(data, "\n")
		data = nil
		if strings.TrimSpace(raw) == "[DONE]" {
			return errors.New("生成预览缺少完成事件，请重试")
		}
		var event generationPreviewEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return errors.New("生成预览事件格式无效，请重试")
		}
		if event.Type == "error" || event.Type == "response.failed" || event.Type == "response.incomplete" || (len(event.Error) > 0 && string(event.Error) != "null") {
			detail := event.Error
			if event.Response != nil && len(event.Response.Error) > 0 {
				detail = event.Response.Error
			}
			return fmt.Errorf("生成预览失败或结果不完整：%s", previewErrorDetail(detail))
		}
		switch event.Type {
		case "response.output_text.delta":
			_, _ = text.WriteString(event.Delta)
		case "response.output_text.done":
			if text.Len() == 0 {
				_, _ = text.WriteString(event.Text)
			}
		case "response.completed", "response.done":
			if event.Response == nil || event.Response.Status != "completed" || (len(event.Response.Error) > 0 && string(event.Response.Error) != "null") {
				return errors.New("生成预览未正常完成，请重试")
			}
			if text.Len() == 0 {
				for _, item := range event.Response.Output {
					for _, content := range item.Content {
						if content.Type == "output_text" {
							_, _ = text.WriteString(content.Text)
						}
					}
				}
			}
			model = event.Response.Model
			if len(model) > 256 || strings.TrimSpace(text.String()) == "" {
				return errors.New("生成预览未返回有效文本，请重试")
			}
			done = true
		}
		return nil
	}
	for scanner.Scan() {
		if limited.N <= 0 {
			return "", "", errors.New("生成预览响应超过 2 MiB，请简化提示词后重试")
		}
		line := scanner.Text()
		if line == "" {
			if err := process(); err != nil {
				return "", "", err
			}
			if done {
				return text.String(), model, nil
			}
		} else if strings.HasPrefix(line, "data:") {
			if len(data) > 0 && json.Valid([]byte(strings.Join(data, "\n"))) {
				if err := process(); err != nil {
					return "", "", err
				}
				if done {
					return text.String(), model, nil
				}
			}
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if scanner.Err() != nil || limited.N <= 0 {
		return "", "", errors.New("生成预览响应读取失败或超过 2 MiB，请重试")
	}
	if err := process(); err != nil {
		return "", "", err
	}
	if !done {
		return "", "", errors.New("生成预览事件流中断，未收到完成事件，请重试")
	}
	return text.String(), model, nil
}

// Decode JSON before redaction so JSON escapes cannot disguise credentials.
func previewErrorDetail(raw []byte) string {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) == nil && len(envelope.Error) > 0 {
		raw = envelope.Error
	}
	var message string
	if json.Unmarshal(raw, &message) == nil {
		return message
	}
	var detail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(raw, &detail) == nil {
		return strings.TrimSpace(strings.Join([]string{detail.Code, detail.Message, detail.Detail}, " "))
	}
	return "上游未返回可用错误详情，请检查账号授权、额度或稍后重试"
}
