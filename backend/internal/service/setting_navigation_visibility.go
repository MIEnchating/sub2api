package service

import (
	"encoding/json"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const maxNavigationVisibilityItems = 256

var protectedNavigationPaths = map[string]struct{}{
	"/dashboard":       {},
	"/admin/dashboard": {},
	"/admin/settings":  {},
}

// normalizeNavigationItemVisibility validates the operator-provided map while
// keeping the storage format extensible for newly added frontend routes.
func normalizeNavigationItemVisibility(input map[string]bool) (map[string]bool, error) {
	if len(input) > maxNavigationVisibilityItems {
		return nil, infraerrors.BadRequest("INVALID_NAVIGATION_ITEM_VISIBILITY", fmt.Sprintf("too many navigation items (max %d)", maxNavigationVisibilityItems))
	}

	result := make(map[string]bool, len(input))
	for rawPath, visible := range input {
		path := strings.TrimSpace(rawPath)
		if len(path) > 1 {
			path = strings.TrimRight(path, "/")
		}
		if path == "" || !strings.HasPrefix(path, "/") || len(path) > 200 || strings.ContainsAny(path, "?#") {
			return nil, infraerrors.BadRequest("INVALID_NAVIGATION_ITEM_VISIBILITY", fmt.Sprintf("invalid navigation path %q", rawPath))
		}
		if _, protected := protectedNavigationPaths[path]; protected {
			return nil, infraerrors.BadRequest("INVALID_NAVIGATION_ITEM_VISIBILITY", fmt.Sprintf("navigation path %q cannot be hidden or overridden", path))
		}
		result[path] = visible
	}
	return result, nil
}

func parseNavigationItemVisibility(raw, legacyUserSubscriptions, legacyAdminSubscriptions string) map[string]bool {
	result := map[string]bool{}
	if strings.TrimSpace(raw) != "" {
		var stored map[string]bool
		if err := json.Unmarshal([]byte(raw), &stored); err == nil {
			for path, visible := range stored {
				if len(path) > 1 {
					path = strings.TrimRight(path, "/")
				}
				if _, protected := protectedNavigationPaths[path]; protected {
					continue
				}
				if path != "" && strings.HasPrefix(path, "/") && len(path) <= 200 && !strings.ContainsAny(path, "?#") {
					result[path] = visible
				}
			}
		}
	}

	// Old installations may only have the two dedicated subscription switches.
	if _, exists := result["/subscriptions"]; !exists {
		result["/subscriptions"] = !isFalseSettingValue(legacyUserSubscriptions)
	}
	if _, exists := result["/admin/subscriptions"]; !exists {
		result["/admin/subscriptions"] = !isFalseSettingValue(legacyAdminSubscriptions)
	}
	return result
}
