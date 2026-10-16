package service

import (
	"maps"
)

// IsRetiredAccountExtraKey identifies obsolete credentials and configuration.
// Keep this boundary for old backups, stale caches and full-account writes;
// these keys no longer enable any gateway behavior.
func IsRetiredAccountExtraKey(key string) bool {
	// This fork retains active harvesting; only account-owned proxy material is obsolete.
	return key == "codex_harvest_proxy_url" || key == "codex_ticket_harvest_proxy_ids"
}

// StripRetiredAccountExtra returns a copy without obsolete account material.
// It must not mutate shared scheduler snapshots or the caller's input.
func StripRetiredAccountExtra(extra map[string]any) map[string]any {
	clean := maps.Clone(extra)
	for key := range clean {
		if IsRetiredAccountExtraKey(key) {
			delete(clean, key)
		}
	}
	return clean
}
