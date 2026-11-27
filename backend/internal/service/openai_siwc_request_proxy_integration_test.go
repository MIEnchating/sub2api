//go:build unit && bps_request_integration

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func enableLocalSIWCRequestProxies(t *testing.T, gateway *OpenAIGatewayService, account *Account) {
	t.Helper()
	reauth, reader, repo, _, _, _ := newReauthTestService("acct-1")
	t.Cleanup(reauth.oauth.Stop)
	account.Credentials = cloneReauthMap(reader.account.Credentials)
	account.Extra[OpenAISIWCEnabledKey] = true
	account.Extra[OpenAIProxyModeKey] = OpenAIProxyModeRequest
	account.Extra[OpenAIProxyIDsKey] = []int64{1, 2, 3}
	reader.account = account
	store := &siwcTestRepo{reauthTestRepo: repo}
	raw, err := json.Marshal(siwcGrant{AccessToken: "independent-siwc", RefreshToken: "siwc-refresh", ClientID: "issued", Subject: "subscriber", Workspace: "acct-1", Email: "user@example.com", Scope: openai.SIWCDirectScope, ExpiresAt: time.Now().Add(time.Hour).Unix()})
	require.NoError(t, err)
	store.ciphertext = "encrypted:" + string(raw)
	reauth.repo = store
	reauth.oauth.proxyRepo = gateway.proxyRepo
	gateway.excelOAuthReauth = reauth
}

type siwcRequestProxyAdmissionRepo struct {
	AccountRepository
	request, latest *Account
	reads           int
}

func (r *siwcRequestProxyAdmissionRepo) GetOpenAITurnAdmission(context.Context, int64) (*Account, *Account, error) {
	r.reads++
	if r.reads == 1 {
		return r.request, nil, nil
	}
	return r.latest, nil, nil
}

// Refresh uses a real local HTTP proxy just like inference; only the external
// OAuth endpoint is replaced, and the production grant/CAS path stays intact.
type siwcRequestProxyTokenClient struct{ *reauthTestOAuthClient }

func (c *siwcRequestProxyTokenClient) SIWCToken(ctx context.Context, form url.Values, proxy, _ string) (*openai.TokenResponse, *openai.SIWCIdentity, error) {
	u, err := url.Parse(proxy)
	if err != nil || u.Host == "" {
		return nil, nil, fmt.Errorf("refresh must use a configured proxy")
	}
	tr := &http.Transport{Proxy: http.ProxyURL(u)}
	defer tr.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://auth.openai.test/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	var token openai.TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	return &token, nil, err
}
