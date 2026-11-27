package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// PanelAPIKeyAuth requires an existing JWT subject and ownership. Never install
// this on public gateway routes: an ID is not an API credential.
func PanelAPIKeyAuth(keys *service.APIKeyService, subscriptions *service.SubscriptionService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return
		}
		id, err := strconv.ParseInt(c.Param("key_id"), 10, 64)
		if err != nil || id <= 0 {
			AbortWithError(c, http.StatusBadRequest, "INVALID_API_KEY_ID", "Invalid API key ID")
			return
		}
		key, err := keys.GetForOwnerAuth(c.Request.Context(), id, subject.UserID)
		if err != nil {
			if errors.Is(err, service.ErrAPIKeyNotFound) {
				AbortWithError(c, http.StatusNotFound, "API_KEY_NOT_FOUND", "API key not found")
			} else {
				AbortWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to validate API key")
			}
			return
		}
		SetOpsFallbackAPIKey(c, key)
		if accessErr := ValidateAPIKeyAccess(c, key, cfg); accessErr != nil {
			status := accessErr.Status
			// A rejected key does not invalidate the already authenticated panel session.
			if status == http.StatusUnauthorized {
				status = http.StatusForbidden
			}
			AbortWithError(c, status, accessErr.Code, accessErr.Message)
			return
		}
		admitLoadedAPIKey(c, key, keys, subscriptions, cfg, false, func() {
			// Revalidate by owner-bound ID. No raw API credential is needed by
			// the panel or installed into the standard request context.
			lookup := func(ctx context.Context, _ string) (*service.APIKey, error) {
				return keys.GetForOwnerAuth(ctx, id, subject.UserID)
			}
			nextWithAPIKeyAdmissionLookup(c, lookup, c.Param("key_id"), ip.GetSecurityClientIP(c, cfg.TrustForwardedIPForAPIKeyACL()), key, false)
		})
	}
}
