package admin

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) SetOpenAICodexTicketProxyTester(tester *service.OpenAICodexTicketProxyTester) {
	h.codexTicketProxyTester = tester
}

// TestOpenAICodexTicketProxy only accepts the single persisted harvest proxy (index 1).
// The independent diagnostic is available even while ticket harvesting is off.
func (h *SettingHandler) TestOpenAICodexTicketProxy(c *gin.Context) {
	var input struct {
		ProxyIndex int `json:"proxy_index"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.BadRequest(c, "Invalid proxy test request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		response.BadRequest(c, "Invalid proxy test request")
		return
	}
	response.Success(c, h.codexTicketProxyTester.Test(c.Request.Context(), input.ProxyIndex))
}
