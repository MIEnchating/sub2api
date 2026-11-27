package service

import (
	"context"
	"errors"
	"net/url"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

type remainingMaintenanceClient struct {
	*reauthTestOAuthClient
	credentials     map[string]any
	proxies         []string
	connectionCalls int
}

func (c *remainingMaintenanceClient) RefreshTokenWithClientID(_ context.Context, _, proxy, _ string) (*openai.TokenResponse, error) {
	c.proxies = append(c.proxies, proxy)
	return &openai.TokenResponse{AccessToken: reauthMapString(c.credentials, "access_token"), RefreshToken: reauthMapString(c.credentials, "refresh_token"), IDToken: reauthMapString(c.credentials, "id_token"), ExpiresIn: 3600}, nil
}

func (c *remainingMaintenanceClient) SIWCToken(_ context.Context, _ url.Values, proxy, _ string) (*openai.TokenResponse, *openai.SIWCIdentity, error) {
	c.proxies = append(c.proxies, proxy)
	identity := &openai.SIWCIdentity{Email: "user@example.com"}
	identity.Subject = "subscriber"
	return &openai.TokenResponse{AccessToken: "fresh-siwc-access", RefreshToken: "fresh-siwc-refresh", ExpiresIn: 3600, Scope: openai.SIWCDirectScope}, identity, nil
}

func (c *remainingMaintenanceClient) ConnectionToken(context.Context, url.Values, string, string, string) (*openai.TokenResponse, *openai.SIWCIdentity, error) {
	c.connectionCalls++
	identity := &openai.SIWCIdentity{}
	identity.Subject = "subscriber"
	return &openai.TokenResponse{AccessToken: "connection-access", RefreshToken: "connection-refresh", ExpiresIn: 3600}, identity, nil
}
func (c *remainingMaintenanceClient) RevokeConnectionToken(context.Context, string, string, string) error {
	c.connectionCalls++
	return nil
}

func remainingMaintenanceSelection(reader *reauthTestAccountReader, svc *OpenAIOAuthReauthService) *openAIMaintenanceProxyRepo {
	legacy := &Proxy{ID: 90, Protocol: "http", Host: "old.invalid", Port: 8080, Status: StatusActive}
	reader.proxy = legacy
	reader.account.ProxyID = &legacy.ID
	reader.account.Extra = map[string]any{OpenAIProxyModeKey: OpenAIProxyModeRequest, OpenAIProxyIDsKey: []int64{33}}
	repo := &openAIMaintenanceProxyRepo{proxies: map[int64]*Proxy{33: {ID: 33, Protocol: "http", Host: "selected.invalid", Port: 8080, Status: StatusActive}}}
	svc.oauth.proxyRepo = repo
	return repo
}

type remainingMaintenanceConnectionRepo struct {
	*connectionTypeRepo
}

func (r *remainingMaintenanceConnectionRepo) BeginAuthConnectionRefresh(context.Context, *OpenAIAuthConnection, string) (bool, error) {
	r.writes++
	return false, errors.New("unexpected refresh intent")
}
