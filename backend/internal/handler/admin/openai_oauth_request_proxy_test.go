package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type oauthHandlerRequestProxyRepo struct {
	service.ProxyRepository
	proxies map[int64]*service.Proxy
}

func (r *oauthHandlerRequestProxyRepo) GetByID(_ context.Context, id int64) (*service.Proxy, error) {
	if proxy := r.proxies[id]; proxy != nil {
		return proxy, nil
	}
	return nil, errors.New("synthetic private proxy address")
}

type oauthHandlerRequestProxyClient struct {
	service.OpenAIOAuthClient
	proxies []string
}

func (c *oauthHandlerRequestProxyClient) ExchangeCode(_ context.Context, _, _, _, proxyURL, clientID string) (*openai.TokenResponse, error) {
	c.proxies = append(c.proxies, proxyURL)
	token := "synthetic"
	if clientID == openai.ExcelClientID {
		claims := fmt.Sprintf(`{"exp":%d,"client_id":%q,"https://api.openai.com/auth":{"chatgpt_account_id":"synthetic-account"}}`,
			time.Now().Add(time.Hour).Unix(), clientID)
		token = "e30." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".synthetic"
	}
	return &openai.TokenResponse{AccessToken: token, ExpiresIn: 3600}, nil
}

func TestOpenAIAdminRequestProxyAuthorizationRejectsInvalidSettings(t *testing.T) {
	svc := service.NewOpenAIOAuthService(&oauthHandlerRequestProxyRepo{proxies: map[int64]*service.Proxy{}}, nil)
	defer svc.Stop()
	handler := &OpenAIOAuthHandler{openaiOAuthService: svc}
	for _, body := range []string{`{"proxy_mode":"request","proxy_ids":[]}`, `{"proxy_mode":"request","proxy_ids":[99]}`, `{"proxy_mode":"request","proxy_ids":[-1]}`, `{"proxy_mode":"unknown","proxy_ids":[11]}`, `{"proxy_ids":[11]}`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/generate-auth-url", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.GenerateAuthURL(c)
		require.GreaterOrEqual(t, w.Code, http.StatusBadRequest, body)
		require.NotContains(t, w.Body.String(), "synthetic private proxy address")
	}
}

type oauthHandlerProxyFixture struct {
	repo   *oauthHandlerRequestProxyRepo
	client *oauthHandlerRequestProxyClient
	router *gin.Engine
}

func newOAuthHandlerProxyFixture(t *testing.T) *oauthHandlerProxyFixture {
	t.Helper()
	f := &oauthHandlerProxyFixture{
		repo: &oauthHandlerRequestProxyRepo{proxies: map[int64]*service.Proxy{
			11: {ID: 11, Protocol: "http", Host: "192.0.2.11", Port: 8080, Status: service.StatusActive},
			22: {ID: 22, Protocol: "http", Host: "192.0.2.22", Port: 8080, Status: service.StatusActive},
		}},
		client: &oauthHandlerRequestProxyClient{},
		router: gin.New(),
	}
	svc := service.NewOpenAIOAuthService(f.repo, f.client)
	t.Cleanup(svc.Stop)
	handler := &OpenAIOAuthHandler{openaiOAuthService: svc}
	f.router.POST("/generate-auth-url", handler.GenerateAuthURL)
	f.router.POST("/exchange-code", handler.ExchangeCode)
	return f
}

func (f *oauthHandlerProxyFixture) post(t *testing.T, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(encoded)))
	req.Header.Set("Content-Type", "application/json")
	f.router.ServeHTTP(w, req)
	return w
}

func (f *oauthHandlerProxyFixture) generate(t *testing.T, body map[string]any) (service.OpenAIAuthURLResult, map[string]any) {
	t.Helper()
	w := f.post(t, "/generate-auth-url", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var generated struct {
		Data service.OpenAIAuthURLResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &generated))
	authURL, err := url.Parse(generated.Data.AuthURL)
	require.NoError(t, err)
	return generated.Data, map[string]any{"session_id": generated.Data.SessionID, "code": "synthetic-code", "state": authURL.Query().Get("state")}
}

func TestOpenAIAdminSingleProxyAuthorizationKeepsCompatibility(t *testing.T) {
	for _, mode := range []string{"", "single"} {
		t.Run("mode="+mode, func(t *testing.T) {
			f := newOAuthHandlerProxyFixture(t)
			settings := map[string]any{"proxy_id": 11}
			if mode != "" {
				settings["proxy_mode"] = mode
			}
			generated, exchange := f.generate(t, settings)
			require.Empty(t, generated.ProxyMode)
			exchange["proxy_id"] = 22
			if mode != "" {
				exchange["proxy_mode"] = mode
			}
			w := f.post(t, "/exchange-code", exchange)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, []string{"http://192.0.2.22:8080"}, f.client.proxies)
		})
	}
}
