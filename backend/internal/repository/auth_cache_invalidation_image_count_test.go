package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestImageCountBillingMigrationPreservesDigestInvalidation(t *testing.T) {
	previous, err := migrations.FS.ReadFile("300_api_key_digest_only.sql")
	require.NoError(t, err)
	current, err := migrations.FS.ReadFile("308_group_image_count_auth_cache_invalidation.sql")
	require.NoError(t, err)
	const signature = "CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()"
	start := strings.Index(string(previous), signature)
	require.NotEqual(t, -1, start)
	oldFunction := string(previous)[start:]
	end := strings.Index(oldFunction, "\n$$;")
	require.NotEqual(t, -1, end)
	oldFunction = oldFunction[:end+len("\n$$;")]
	// Adding a billing guard must not undo digest storage, fallback-group keys,
	// soft-delete handling or the existing security/profit invalidation guards.
	withoutNewGuard := strings.ReplaceAll(string(current),
		"       AND OLD.image_count_billing IS NOT DISTINCT FROM NEW.image_count_billing\n", "")
	require.Contains(t, string(current), "OLD.image_count_billing IS NOT DISTINCT FROM NEW.image_count_billing")
	require.Contains(t, withoutNewGuard, oldFunction)
	require.NotContains(t, string(current), "DROP TABLE")
	require.NotContains(t, string(current), "DELETE FROM")
}
