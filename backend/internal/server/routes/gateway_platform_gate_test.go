package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayPlatformGateSettingRepo struct {
	values map[string]string
}

func (r *gatewayPlatformGateSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *gatewayPlatformGateSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *gatewayPlatformGateSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *gatewayPlatformGateSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *gatewayPlatformGateSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *gatewayPlatformGateSettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *gatewayPlatformGateSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestRequireEnabledGatewayPlatformUsesResolvedCompositeTarget(t *testing.T) {
	repo := &gatewayPlatformGateSettingRepo{values: map[string]string{
		service.SettingKeyGatewayPlatformEnabled: `{"openai":false}`,
	}}
	settings := service.NewSettingService(repo, nil)
	require.NoError(t, settings.WarmGatewayRuntimePolicy(context.Background()))

	router := newGatewayPlatformGateRouter(settings, service.PlatformOpenAI)
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), "platform_disabled")
}

func TestRequireEnabledGatewayPlatformAllowsEnabledTarget(t *testing.T) {
	repo := &gatewayPlatformGateSettingRepo{values: map[string]string{
		service.SettingKeyGatewayPlatformEnabled: `{"openai":true}`,
	}}
	settings := service.NewSettingService(repo, nil)
	require.NoError(t, settings.WarmGatewayRuntimePolicy(context.Background()))

	router := newGatewayPlatformGateRouter(settings, service.PlatformOpenAI)
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
}

func newGatewayPlatformGateRouter(settings *service.SettingService, targetPlatform string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(1)
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformComposite},
		})
		c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), targetPlatform))
		c.Next()
	})
	router.Use(requireEnabledGatewayPlatform(settings, false, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	}))
	router.POST("/", func(c *gin.Context) {})
	return router
}
