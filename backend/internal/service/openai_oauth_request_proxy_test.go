package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type openAIMaintenanceProxyRepo struct {
	ProxyRepository
	proxies   map[int64]*Proxy
	err       error
	requested []int64
}

func (r *openAIMaintenanceProxyRepo) GetByID(_ context.Context, id int64) (*Proxy, error) {
	r.requested = append(r.requested, id)
	if r.err != nil {
		return nil, r.err
	}
	return r.proxies[id], nil
}

type openAIMaintenanceOAuthClient struct {
	openaiOAuthClientRefreshStub
	proxies []string
}

func (s *openAIMaintenanceOAuthClient) RefreshTokenWithClientID(_ context.Context, _, proxyURL, _ string) (*openai.TokenResponse, error) {
	s.proxies = append(s.proxies, proxyURL)
	return nil, errors.New("fake token endpoint")
}

func TestOpenAIOAuthRefreshExplicitSingleOverridesLegacyGroup(t *testing.T) {
	id := int64(90)
	repo := &openAIMaintenanceProxyRepo{proxies: map[int64]*Proxy{90: {ID: 90, Protocol: "http", Host: "single.example", Port: 8080}}}
	client := &openAIMaintenanceOAuthClient{}
	svc := NewOpenAIOAuthService(repo, client)
	defer svc.Stop()
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, ProxyID: &id, Credentials: map[string]any{"refresh_token": "fake"}, Extra: map[string]any{"openai_proxy_mode": "single", "openai_excel_bps": true, "openai_excel_bps_proxy_mode": "request", "openai_excel_bps_proxy_ids": []any{float64(1)}}}
	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.ErrorContains(t, err, "fake token endpoint")
	require.Equal(t, []string{"http://single.example:8080"}, client.proxies)
}
