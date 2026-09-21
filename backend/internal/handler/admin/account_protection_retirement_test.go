package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountProtectionPresetEndpointsAreRetired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// No service is supplied: retired calls must return without reading or
	// mutating account state, including requests from an old cached frontend.
	h := &admin.AccountHandler{}
	for _, tc := range []struct {
		method, path string
		handler      gin.HandlerFunc
	}{
		{http.MethodGet, "/accounts/anti-degrade/strategies", h.ListAntiDegradeStrategies},
		{http.MethodGet, "/accounts/71/anti-degrade", h.PreviewAntiDegrade},
		{http.MethodPost, "/accounts/71/anti-degrade/apply", h.ApplyAntiDegrade},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			router := gin.New()
			router.Handle(tc.method, tc.path, tc.handler)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path+"?mode=mode1", nil))
			require.Equal(t, http.StatusGone, rec.Code)
			require.Contains(t, rec.Body.String(), "retired")
		})
	}
}

func TestLegacyProtectionRestoreStillRequiresConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &admin.AccountHandler{}
	router := gin.New()
	router.POST("/accounts/:id/anti-degrade/revert", h.RevertAntiDegrade)
	for _, body := range []string{`{}`, `{"confirm_disable":false}`} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/accounts/71/anti-degrade/revert", strings.NewReader(body)))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "管理员明确确认")
	}
}
