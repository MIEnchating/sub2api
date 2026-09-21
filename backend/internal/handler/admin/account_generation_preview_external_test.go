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

func TestGenerationPreviewHandlerRejectsInvalidRequestsBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, input := range map[string]struct{ id, body string }{
		"invalid account":  {"not-an-id", `{}`},
		"negative account": {"-1", `{}`},
		"malformed JSON":   {"1", `{`},
		"missing prompt":   {"1", `{"model_id":"gpt-6-astra","request_id":"preview-request","reasoning_effort":"low","timeout_seconds":5}`},
		"oversized body":   {"1", `{"prompt":"` + strings.Repeat("a", 129<<10) + `"}`},
	} {
		t.Run(name, func(t *testing.T) {
			handler := admin.NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.POST("/accounts/:id/generate-preview", handler.GeneratePreview)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/accounts/"+input.id+"/generate-preview", strings.NewReader(input.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		})
	}
}
