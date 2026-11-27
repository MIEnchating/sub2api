package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type oauthRequestProxyRepo struct {
	ProxyRepository
	proxies map[int64]*Proxy
}

func (r *oauthRequestProxyRepo) GetByID(_ context.Context, int64ID int64) (*Proxy, error) {
	if proxy := r.proxies[int64ID]; proxy != nil {
		return proxy, nil
	}
	return nil, errors.New("synthetic private proxy address")
}

type oauthRequestProxyClient struct {
	OpenAIOAuthClient
	proxies []string
}

func (c *oauthRequestProxyClient) ExchangeCode(_ context.Context, _, _, _, proxyURL, clientID string) (*openai.TokenResponse, error) {
	c.proxies = append(c.proxies, proxyURL)
	if clientID == openai.ExcelClientID {
		credentials := excelTestCredentials("request-proxy", time.Now().Add(time.Hour))
		accessToken, accessOK := credentials["access_token"].(string)
		idToken, idOK := credentials["id_token"].(string)
		if !accessOK || !idOK {
			return nil, errors.New("missing synthetic Excel OAuth credentials")
		}
		return &openai.TokenResponse{AccessToken: accessToken, IDToken: idToken, ExpiresIn: 3600}, nil
	}
	return &openai.TokenResponse{AccessToken: "synthetic", ExpiresIn: 3600}, nil
}

func oauthRequestProxyFixture(t *testing.T, request bool) (*OpenAIOAuthService, *oauthRequestProxyRepo, *oauthRequestProxyClient, *OpenAIExchangeCodeInput) {
	t.Helper()
	repo := &oauthRequestProxyRepo{proxies: map[int64]*Proxy{
		11: {ID: 11, Protocol: "http", Host: "192.0.2.11", Port: 8080, Status: StatusActive},
		22: {ID: 22, Protocol: "http", Host: "192.0.2.22", Port: 8080, Status: StatusActive},
	}}
	client := &oauthRequestProxyClient{}
	svc := NewOpenAIOAuthService(repo, client)
	t.Cleanup(svc.Stop)
	svc.sessionStore.Set("synthetic-session", &openai.OAuthSession{State: "synthetic-state", CodeVerifier: "verifier", ProxyURL: repo.proxies[11].URL(), RequestProxy: request, CreatedAt: time.Now()})
	return svc, repo, client, &OpenAIExchangeCodeInput{SessionID: "synthetic-session", State: "synthetic-state", Code: "synthetic-code", ProxyMode: OpenAIProxyModeRequest, ProxyIDs: []int64{11, 22}}
}

func TestOpenAIOAuthRequestProxyExchangeCannotUpgradeSingleSession(t *testing.T) {
	svc, _, client, input := oauthRequestProxyFixture(t, false)
	_, err := svc.ExchangeCode(context.Background(), input)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Empty(t, client.proxies)
}

func TestOpenAIOAuthSingleProxyExchangeRetainsOverride(t *testing.T) {
	svc, repo, client, input := oauthRequestProxyFixture(t, false)
	input.ProxyMode = ""
	input.ProxyIDs = nil
	override := int64(22)
	input.ProxyID = &override
	_, err := svc.ExchangeCode(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []string{repo.proxies[22].URL()}, client.proxies)
}
