package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestUltrafastAuthCacheMigrationPreservesBillingAndSecurityGuards(t *testing.T) {
	previous, err := migrations.FS.ReadFile("353_group_billing_auth_cache_compat.sql")
	require.NoError(t, err)
	current, err := migrations.FS.ReadFile("356_group_ultrafast_auth_cache_compat.sql")
	require.NoError(t, err)
	const signature = "CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()"
	start := strings.Index(string(previous), signature)
	require.NotEqual(t, -1, start)
	previousFunction := string(previous)[start:]
	withoutNewGuard := strings.ReplaceAll(string(current),
		"       AND OLD.force_openai_ultrafast IS NOT DISTINCT FROM NEW.force_openai_ultrafast\n", "")
	require.Contains(t, string(current), "OLD.force_openai_ultrafast IS NOT DISTINCT FROM NEW.force_openai_ultrafast")
	require.Contains(t, withoutNewGuard, previousFunction)
	require.NotContains(t, string(current), "DROP TABLE")
	require.NotContains(t, string(current), "DELETE FROM")
}

func TestFreeUltrafastAuthCacheMigrationPreservesExistingGuards(t *testing.T) {
	previous, err := migrations.FS.ReadFile("356_group_ultrafast_auth_cache_compat.sql")
	require.NoError(t, err)
	current, err := migrations.FS.ReadFile("359_group_free_openai_ultrafast_auth_cache_compat.sql")
	require.NoError(t, err)
	const signature = "CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()"
	const guard = "       AND OLD.free_openai_ultrafast IS NOT DISTINCT FROM NEW.free_openai_ultrafast\n"
	start := strings.Index(string(previous), signature)
	require.NotEqual(t, -1, start)
	require.Contains(t, string(current), guard)
	require.Contains(t, strings.Replace(string(current), guard, "", 1), string(previous)[start:])
}
