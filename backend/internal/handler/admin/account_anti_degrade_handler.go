package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Retired endpoints remain explicit for cached clients. Only restoration of
// previously enabled presets is supported; no new preset may be applied.
func retiredAccountProtection(c *gin.Context) {
	response.Error(c, http.StatusGone, "Account protection presets have been retired; configure fingerprint, TLS, and concurrency independently")
}

func (h *AccountHandler) ListAntiDegradeStrategies(c *gin.Context) {
	retiredAccountProtection(c)
}

func (h *AccountHandler) PreviewAntiDegrade(c *gin.Context) {
	retiredAccountProtection(c)
}

func (h *AccountHandler) ApplyAntiDegrade(c *gin.Context) {
	retiredAccountProtection(c)
}

func (h *AccountHandler) antiDegradeService() *service.AntiDegradeService {
	if h == nil || h.adminService == nil {
		return nil
	}
	return service.NewAntiDegradeService(h.adminService)
}

func parseAntiDegradeAccountID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	return id, true
}

// RevertAntiDegrade restores the snapshot captured when protection was
// applied. The explicit confirmation prevents accidental disabling.
func (h *AccountHandler) RevertAntiDegrade(c *gin.Context) {
	id, ok := parseAntiDegradeAccountID(c)
	if !ok {
		return
	}
	var req struct {
		ConfirmDisable bool `json:"confirm_disable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.ConfirmDisable {
		response.BadRequest(c, "恢复旧版预设配置需要管理员明确确认")
		return
	}
	svc := h.antiDegradeService()
	if svc == nil {
		response.BadRequest(c, "account protection service unavailable")
		return
	}
	account, err := svc.Revert(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}
