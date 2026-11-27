//go:build unit && bps_request_integration

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mihomo"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

// This barrier belongs to the real OAuthRefreshAPI repository re-read. Admin
// UpdateAccount receives the original context, so it can save normally while
// the request-scoped GetByID waits with the already acquired inference lease.
type pr885OAuthConfigSwitchAccounts struct {
	AccountRepository
	mu      sync.Mutex
	account *Account
	ready   chan *openAIRequestProxyRoute
	resume  chan struct{}
	once    sync.Once
}

func pr885OAuthConfigSwitchClone(account *Account) *Account {
	copy := *account
	copy.Extra = cloneReauthMap(account.Extra)
	copy.Credentials = cloneReauthMap(account.Credentials)
	if account.ProxyID != nil {
		id := *account.ProxyID
		copy.ProxyID = &id
	}
	return &copy
}

func (r *pr885OAuthConfigSwitchAccounts) GetByID(ctx context.Context, id int64) (*Account, error) {
	if route, ok := openAIRequestProxyFromContext(ctx, id); ok {
		r.once.Do(func() {
			r.ready <- route
			select {
			case <-r.resume:
			case <-ctx.Done():
			}
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.account == nil || r.account.ID != id {
		return nil, ErrAccountNotFound
	}
	return pr885OAuthConfigSwitchClone(r.account), nil
}

func (r *pr885OAuthConfigSwitchAccounts) Update(_ context.Context, account *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.account = pr885OAuthConfigSwitchClone(account)
	return nil
}

func (r *pr885OAuthConfigSwitchAccounts) ListShadowsByParent(context.Context, int64) ([]*Account, error) {
	return nil, nil
}

type pr885OAuthConfigSwitchTokenClient struct {
	openaiOAuthClientRefreshStub
	directURL string
}

func (c *pr885OAuthConfigSwitchTokenClient) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxy, clientID string) (*openai.TokenResponse, error) {
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	endpoint := c.directURL + "/oauth/token"
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("invalid fixture proxy")
		}
		transport.Proxy = http.ProxyURL(u)
		endpoint = "http://auth.openai.test/oauth/token"
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {clientID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Transport: transport, Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("controlled ordinary OAuth token endpoint status %d", resp.StatusCode)
	}
	var token openai.TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	return &token, err
}

func pr885OAuthConfigSwitchAssertLeaseReturned(t *testing.T, accountID int64) {
	t.Helper()
	spare, err := mihomo.AcquireBPSStaticRequestLease(t.Context(), accountID, []int64{2})
	require.NoError(t, err)
	defer spare.Release()
	cursor, err := mihomo.AcquireBPSStaticRequestLease(t.Context(), accountID, []int64{1})
	require.NoError(t, err)
	cursor.Release()
	next, err := mihomo.AcquireBPSStaticRequestLease(t.Context(), accountID, []int64{1, 2})
	require.NoError(t, err)
	defer next.Release()
	require.Equal(t, int64(1), next.ProxyID, "ordinary OAuth refresh rejection must return request lease A")
}

type pr885OAuthConfigSwitchFixture struct {
	gateway  *OpenAIGatewayService
	account  *Account
	accounts *pr885OAuthConfigSwitchAccounts
	provider *OpenAITokenProvider
	hits     func() []requestProxyObservation
}

func newPR885OAuthConfigSwitchFixture(t *testing.T) pr885OAuthConfigSwitchFixture {
	t.Helper()
	var observedMu sync.Mutex
	var observed []requestProxyObservation
	handle := func(id int, w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		observedMu.Lock()
		observed = append(observed, requestProxyObservation{id: id, path: r.URL.Path, body: raw})
		observedMu.Unlock()
		if r.URL.Path == "/oauth/token" {
			// A real HTTP error also exercises the provider's production
			// default fallback policy. The fixture must not set Return to
			// conceal an inference send after a routing rejection.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"error":"controlled ordinary OAuth refresh failure"}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, bpsRequestCompletedWire)
	}
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handle(0, w, r) }))
	t.Cleanup(direct.Close)
	gateway, account := localBPSRequestProxyFixture(t, 3, handle)
	delete(account.Extra, "openai_excel_bps")
	account.Extra[OpenAISIWCEnabledKey] = false
	account.Extra[OpenAIProxyModeKey] = OpenAIProxyModeRequest
	account.Extra[OpenAIProxyIDsKey] = []int64{1}
	account.Credentials["refresh_token"] = "ordinary-oauth-refresh"
	account.Credentials["expires_at"] = time.Now().Add(-time.Hour).Format(time.RFC3339)
	account.Credentials["client_id"] = "ordinary-oauth-client"
	proxyA, err := gateway.proxyRepo.GetByID(t.Context(), 1)
	require.NoError(t, err)
	account.ProxyID, account.Proxy = &proxyA.ID, proxyA
	accounts := &pr885OAuthConfigSwitchAccounts{
		account: pr885OAuthConfigSwitchClone(account),
		ready:   make(chan *openAIRequestProxyRoute, 1), resume: make(chan struct{}),
	}
	oauth := NewOpenAIOAuthService(gateway.proxyRepo, &pr885OAuthConfigSwitchTokenClient{directURL: direct.URL})
	t.Cleanup(oauth.Stop)
	provider := NewOpenAITokenProvider(accounts, nil, oauth)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(accounts, nil), NewOpenAITokenRefresher(oauth, accounts))
	gateway.openAITokenProvider = provider
	gateway.accountRepo = accounts
	require.Equal(t, ProviderRefreshErrorUseExistingToken, provider.refreshPolicy.OnRefreshError, "preserve the real default provider policy")
	return pr885OAuthConfigSwitchFixture{
		gateway: gateway, account: account, accounts: accounts, provider: provider,
		hits: func() []requestProxyObservation {
			observedMu.Lock()
			defer observedMu.Unlock()
			return append([]requestProxyObservation(nil), observed...)
		},
	}
}
