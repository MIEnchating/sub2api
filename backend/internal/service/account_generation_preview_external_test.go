package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Any repository mutation panics through the nil embedded interface: previews
// may only read the selected account, including on 401, 429 and success.
type previewRepository struct {
	service.AccountRepository
	account *service.Account
}

func (r previewRepository) GetByID(_ context.Context, id int64) (*service.Account, error) {
	if id != r.account.ID {
		return nil, fmt.Errorf("account not found")
	}
	return r.account, nil
}

type previewTransport struct {
	service.HTTPUpstream
	t             *testing.T
	status        int
	body          string
	requests      int
	expectedProxy string
}

func (u *previewTransport) DoWithTLS(r *http.Request, proxy string, id int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests++
	require.Equal(u.t, u.expectedProxy, proxy)
	require.Equal(u.t, int64(1223), id)
	require.Equal(u.t, "https://chatgpt.com/backend-api/codex/responses", r.URL.String())
	require.Equal(u.t, "Bearer preview-private-access", r.Header.Get("Authorization"))
	require.Equal(u.t, "preview-request", r.Header.Get("X-Request-ID"))
	require.True(u.t, service.AccountProtectionOutcomeExcluded(r.Context()))
	require.True(u.t, service.HTTPUpstreamRedirectsDisabled(r.Context()))
	require.Nil(u.t, r.GetBody, "preview requests must not opt into shared HTTP retries")
	var body map[string]any
	require.NoError(u.t, json.NewDecoder(r.Body).Decode(&body))
	require.Contains(u.t, fmt.Sprint(body["input"]), "Draw an SVG")
	require.Equal(u.t, false, body["store"])
	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(u.t, ok)
	require.Equal(u.t, "low", reasoning["effort"])
	return &http.Response{StatusCode: u.status, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Codex-Primary-Used-Percent": []string{"88"}}, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}

func previewFixture(t *testing.T, status int, body string, changes ...func(*service.Account)) (*service.AccountTestService, *previewTransport) {
	t.Helper()
	account := &service.Account{ID: 1223, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "preview-private-access", "refresh_token": "preview-private-refresh"}}
	for _, change := range changes {
		change(account)
	}
	transport := &previewTransport{t: t, status: status, body: body}
	return service.NewAccountTestService(previewRepository{account: account}, nil, nil, nil, nil, transport, nil, nil), transport
}

func TestGenerationPreviewRejectsMissingCredentialChangedTypeAndUnavailableProxyBeforeRequest(t *testing.T) {
	for name, change := range map[string]func(*service.Account){
		"missing credential": func(a *service.Account) { a.Credentials = map[string]any{} },
		"wrong platform":     func(a *service.Account) { a.Platform = service.PlatformAnthropic },
		"wrong type":         func(a *service.Account) { a.Type = service.AccountTypeAPIKey },
		"missing proxy":      func(a *service.Account) { id := int64(7); a.ProxyID = &id },
	} {
		t.Run(name, func(t *testing.T) {
			svc, transport := previewFixture(t, 200, "", change)
			_, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
			require.Error(t, err)
			require.Zero(t, transport.requests)
		})
	}
}

func TestGenerationPreviewAcceptsTerminalOutputWithoutDeltas(t *testing.T) {
	svc, _ := previewFixture(t, 200, `data: {"type":"response.completed","response":{"status":"completed","model":"gpt-6-astra","output":[{"content":[{"type":"output_text","text":"<svg/>"}]}]}}`+"\n\n")
	result, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
	require.NoError(t, err)
	require.Equal(t, "<svg/>", result.Text)
}

func TestGenerationPreviewRejectsOversizedResponse(t *testing.T) {
	svc, _ := previewFixture(t, 200, `data: {"type":"response.output_text.delta","delta":"`+strings.Repeat("x", 2<<20)+`"}`+"\n\n")
	result, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
	require.Error(t, err)
	require.Nil(t, result)
}

func previewRequest() service.AccountGenerationPreviewRequest {
	return service.AccountGenerationPreviewRequest{ModelID: "gpt-6-astra", Prompt: "Draw an SVG", ReasoningEffort: "low", RequestID: "preview-request", TimeoutSeconds: 5}
}

func TestGenerationPreviewSuccessUsesPromptAndDoesNotWriteAccountState(t *testing.T) {
	svc, transport := previewFixture(t, 200, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<svg/>\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"model\":\"gpt-6-astra\"}}\n\n")
	result, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
	require.NoError(t, err)
	require.Equal(t, "<svg/>", result.Text)
	require.Equal(t, int64(1223), result.AccountID)
	require.Equal(t, "preview-request", result.RequestID)
	require.Equal(t, 1, transport.requests)
}

func TestGenerationPreviewFailureDoesNotWriteAccountStateOrReturnCredentials(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			svc, transport := previewFixture(t, status, `{"error":{"message":"preview-private-access preview-private-refresh preview\u002dprivate\u002daccess"}}`)
			result, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
			require.Error(t, err)
			require.Nil(t, result)
			require.NotContains(t, err.Error(), "preview-private-")
			require.Contains(t, err.Error(), fmt.Sprint(status))
			require.Equal(t, 1, transport.requests)
		})
	}
}

func TestGenerationPreviewUsesAccountProxyAndKeepsProxyCredentialsPrivate(t *testing.T) {
	svc, transport := previewFixture(t, 401, `{"error":{"message":"private-proxy-password"}}`, func(a *service.Account) {
		id := int64(7)
		a.ProxyID = &id
		a.Proxy = &service.Proxy{ID: id, Protocol: "http", Host: "proxy.invalid", Port: 8080, Username: "private-proxy-user", Password: "private-proxy-password"}
	})
	transport.expectedProxy = "http://private-proxy-user:private-proxy-password@proxy.invalid:8080"
	_, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-proxy-password")
	require.Equal(t, 1, transport.requests)
}

func TestGenerationPreviewRejectsIncompleteEmptyFailedAndSensitiveStreams(t *testing.T) {
	for name, body := range map[string]string{
		"missing completion": "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<svg/>\"}\n\n",
		"empty completion":   "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n",
		"failed completion":  "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"preview-private-access\"}}}\n\n",
		"split credential":   "data: {\"type\":\"response.output_text.delta\",\"delta\":\"preview-private-\"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"refresh\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := previewFixture(t, 200, body)
			result, err := svc.GenerateAccountPreview(context.Background(), 1223, previewRequest())
			require.Error(t, err)
			require.Nil(t, result)
			require.NotContains(t, err.Error(), "preview-private-")
		})
	}
}

func TestGenerationPreviewRejectsInvalidInputBeforeUpstreamRequest(t *testing.T) {
	for name, change := range map[string]func(*service.AccountGenerationPreviewRequest){
		"empty prompt":       func(r *service.AccountGenerationPreviewRequest) { r.Prompt = " " },
		"invalid request ID": func(r *service.AccountGenerationPreviewRequest) { r.RequestID = "bad\nheader" },
		"invalid timeout":    func(r *service.AccountGenerationPreviewRequest) { r.TimeoutSeconds = 121 },
		"invalid reasoning":  func(r *service.AccountGenerationPreviewRequest) { r.ReasoningEffort = "invented" },
	} {
		t.Run(name, func(t *testing.T) {
			svc, transport := previewFixture(t, 200, "")
			input := previewRequest()
			change(&input)
			_, err := svc.GenerateAccountPreview(context.Background(), 1223, input)
			require.Error(t, err)
			require.Zero(t, transport.requests)
		})
	}
}
