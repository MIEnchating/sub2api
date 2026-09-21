package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/gin-gonic/gin"
)

const maximumPreviewBytes = 2 << 20

type generationPreviewContextKey struct{}

// AccountGenerationPreviewRequest generates display-only content. It never
// invokes account recovery, credential refresh, quota reconciliation or tests.
type AccountGenerationPreviewRequest struct {
	ModelID         string `json:"model_id"`
	Prompt          string `json:"prompt"`
	ReasoningEffort string `json:"reasoning_effort"`
	RequestID       string `json:"request_id"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
}

type AccountGenerationPreviewResult struct {
	AccountID int64  `json:"account_id"`
	RequestID string `json:"request_id"`
	Model     string `json:"model"`
	Text      string `json:"text"`
}

func (r AccountGenerationPreviewRequest) Validate() error {
	for _, value := range []string{r.ModelID, r.RequestID} {
		if strings.TrimSpace(value) == "" || len(value) > 256 || !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return apperrors.BadRequest("INVALID_PREVIEW_REQUEST", "模型和请求 ID 必须是有效的非空字符串")
		}
	}
	if strings.TrimSpace(r.Prompt) == "" || len(r.Prompt) > 65536 || !utf8.ValidString(r.Prompt) || r.TimeoutSeconds < 1 || r.TimeoutSeconds > 120 {
		return apperrors.BadRequest("INVALID_PREVIEW_REQUEST", "提示词不能为空或超过 64 KiB，超时必须为 1 到 120 秒")
	}
	switch r.ReasoningEffort {
	case "none", "low", "medium", "high":
		return nil
	default:
		return apperrors.BadRequest("INVALID_PREVIEW_REQUEST", "推理强度必须为 none、low、medium 或 high")
	}
}

func (s *AccountTestService) GenerateAccountPreview(ctx context.Context, accountID int64, input AccountGenerationPreviewRequest) (*AccountGenerationPreviewResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if accountID <= 0 {
		return nil, apperrors.BadRequest("INVALID_ACCOUNT_ID", "账号 ID 无效")
	}
	ctx = context.WithValue(WithAccountProtectionOutcomeExcluded(ctx), generationPreviewContextKey{}, true)
	ctx = WithHTTPUpstreamRedirectsDisabled(ctx)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(input.TimeoutSeconds)*time.Second)
	defer cancel()
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.ID != accountID {
		return nil, apperrors.NotFound("ACCOUNT_NOT_FOUND", "生成预览账号不存在，请刷新账号列表")
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth || account.IsCredentialShadow() || account.IsOpenAIAgentIdentity() {
		return nil, apperrors.BadRequest("PREVIEW_ACCOUNT_UNSUPPORTED", "生成预览仅支持直接保存 OAuth 凭据的 OpenAI 账号")
	}
	model := normalizeOpenAIModelForUpstream(account, account.GetMappedModel(input.ModelID))
	if isOpenAIImageModel(model) {
		return nil, apperrors.BadRequest("PREVIEW_MODEL_UNSUPPORTED", "生成预览需要文本模型，请重新选择模型")
	}
	token := account.GetOpenAIAccessToken()
	if token == "" {
		return nil, apperrors.BadRequest("PREVIEW_CREDENTIAL_MISSING", "OAuth Access Token 缺失，请刷新或重新授权账号")
	}
	secrets := []string{token}
	for key, value := range account.Credentials {
		if IsSensitiveCredentialKey(key) || key == "chatgpt_account_id" || key == "account_id" {
			if secret, ok := value.(string); ok && secret != "" {
				secrets = append(secrets, secret)
			}
		}
	}
	payload := createOpenAITestPayload(model, true, input.Prompt)
	// The Codex endpoint does not support max_output_tokens. Cancellation and
	// bounded response parsing limit the preview; never replay a rejected body.
	delete(payload, "max_output_tokens")
	applyAccountTestReasoningEffort(payload, input.ReasoningEffort)
	local := &gin.Context{Request: (&http.Request{Header: make(http.Header)}).WithContext(ctx)}
	if err := prepareIntelligentTestProtection(local, account, payload); err != nil {
		return nil, previewFailure("账号请求保护配置无效，请检查账号配置")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, previewFailure("生成预览请求编码失败")
	}
	request, err := http.NewRequestWithContext(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(raw))
	if err != nil {
		return nil, previewFailure("生成预览请求创建失败")
	}
	request.Host = "chatgpt.com"
	// Shared HTTP retries require GetBody. A preview must never replay a
	// generation, including when a retry policy is inherited from middleware.
	request.GetBody = nil
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("OpenAI-Beta", "responses=experimental")
	canonical := resolveCodexOutboundIdentity("")
	request.Header.Set("Originator", canonical.originator)
	request.Header.Set("User-Agent", canonical.userAgent)
	setOpenAIChatGPTAccountHeaders(request.Header, account)
	enforceCodexIdentityHeadersWithUA(request.Header, account.GetOpenAIUserAgent())
	account.ApplyHeaderOverrides(request.Header)
	if err := applyIntelligentTestProtection(local, account, request.Header, raw); err != nil {
		return nil, previewFailure("账号请求保护校验失败，请检查账号配置")
	}
	request.Header.Set("X-Request-ID", input.RequestID)
	for _, key := range []string{"Authorization", "Cookie", "ChatGPT-Account-Id"} {
		if value := request.Header.Get(key); value != "" {
			secrets = append(secrets, value, strings.TrimPrefix(value, "Bearer "))
		}
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy == nil {
		return nil, apperrors.BadRequest("PREVIEW_PROXY_UNAVAILABLE", "账号代理配置不可用，请检查代理后重试")
	}
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
		secrets = append(secrets, proxyURL, account.Proxy.Username, account.Proxy.Password)
	}
	resp, err := s.doOpenAIAccountTestUpstream(request, proxyURL, account, true)
	if err != nil {
		return nil, previewFailure("生成预览请求失败或超时，请检查账号代理与网络后重试")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, previewFailure(fmt.Sprintf("生成预览上游返回 HTTP %d：%s", resp.StatusCode, previewSafeText(previewErrorDetail(body), secrets)))
	}
	text, responseModel, err := readGenerationPreview(resp.Body)
	if err != nil {
		return nil, previewFailure(previewSafeText(err.Error(), secrets))
	}
	for _, secret := range secrets {
		if secret != "" && (strings.Contains(text, secret) || strings.Contains(responseModel, secret)) {
			return nil, previewFailure("生成结果包含敏感信息，已拒绝展示")
		}
	}
	return &AccountGenerationPreviewResult{AccountID: accountID, RequestID: input.RequestID, Model: responseModel, Text: text}, nil
}

func previewFailure(message string) error {
	return apperrors.New(http.StatusBadGateway, "ACCOUNT_PREVIEW_FAILED", message)
}

func previewSafeText(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[已隐藏]")
		}
	}
	value = logredact.RedactText(value)
	runes := []rune(value)
	if len(runes) > 1000 {
		value = string(runes[:1000]) + "…"
	}
	return value
}
