package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GeneratePreview is deliberately separate from Test: a display-only request
// must never recover an account after success or reconcile its runtime state.
func (h *AccountHandler) GeneratePreview(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "账号 ID 无效")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	var input service.AccountGenerationPreviewRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "生成预览请求格式无效或过大")
		return
	}
	if err := input.Validate(); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.accountTestService.GenerateAccountPreview(c.Request.Context(), accountID, input)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}
