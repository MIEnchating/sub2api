package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestExcelBPSPreservesProtectionAndIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name, body, mode                     string
		invalid, stripImages, stripEncrypted bool
	}{
		{name: "invalid protection", body: `{"model":"gpt-5.6-sol","input":"hello"}`, invalid: true},
		{name: "image filtering", mode: "enforce", stripImages: true, body: `{"model":"gpt-5.6-sol","input":[{"role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"data:image/png;base64,PRIVATE"}]}]}`},
		{name: "encrypted filtering", mode: "enforce", stripEncrypted: true, body: `{"model":"gpt-5.6-sol","input":[{"role":"user","content":"hello"},{"type":"agent_message","content":[{"type":"input_text","text":"answer"},{"type":"encrypted_content","encrypted_content":"PRIVATE"}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{}
			svc := openAIClientToolsTestService(upstream)
			a := excelAccount()
			a.Extra[requestIntegrityModeKey] = tc.mode
			a.Extra[ExcelBPSIgnoreImagesKey] = tc.stripImages
			a.Extra[ExcelBPSIgnoreEncryptedContentKey] = tc.stripEncrypted
			if tc.invalid {
				a.Extra[AntiDegradationExtraKey] = true
				a.Extra[AntiDegradeMarkerExtraKey] = map[string]any{"enabled": true, "mode": "unknown"}
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			_, err := svc.Forward(context.Background(), c, a, []byte(tc.body))
			require.Error(t, err)
			require.Empty(t, upstream.requests, "protected requests must fail before sending or uploading")
			if !tc.invalid {
				require.Contains(t, err.Error(), "input")
			}
		})
	}
}

type excelBPSProtectionTransport struct {
	httpUpstreamRecorder
	concurrency int
	profile     *tlsfingerprint.Profile
}

func (u *excelBPSProtectionTransport) Do(req *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
	u.concurrency = concurrency
	return u.httpUpstreamRecorder.Do(req, proxy, id, concurrency)
}
func (u *excelBPSProtectionTransport) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.profile = profile
	return u.Do(req, proxy, id, concurrency)
}

func TestExcelBPSPreservesProtectedTransport(t *testing.T) {
	for _, mode := range []AntiDegradeMode{AntiDegradeModeGeneric, AntiDegradeModeSingleMachine} {
		t.Run(string(mode), func(t *testing.T) {
			a := excelAccount()
			protection, _ := newAntiDegradeTestService(a)
			a, err := protection.ApplyAntiDegradeMode(context.Background(), a.ID, mode)
			require.NoError(t, err)
			a.Concurrency = 0 // Historical zero/unlimited rows still receive the protection cap.
			upstream := &excelBPSProtectionTransport{}
			svc := openAIClientToolsTestService(nil)
			svc.httpUpstream = upstream
			svc.cfg.Gateway.TLSFingerprint.Enabled = true
			req, err := newExcelBPSRequest(context.Background(), nil, "secret", "account")
			require.NoError(t, err)
			_, err = svc.doExcelBPSHTTP(req, "", a)
			require.NoError(t, err)
			require.Equal(t, AntiDegradeConcurrencyCap, upstream.concurrency)
			require.Len(t, upstream.requests, 1)
			if mode == AntiDegradeModeSingleMachine {
				require.NotNil(t, upstream.profile)
			}
			require.Empty(t, req.Header.Get("x-codex-turn-state"))
		})
	}
}
