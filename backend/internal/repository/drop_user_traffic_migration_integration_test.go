//go:build integration

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestDropUserTrafficMigration(t *testing.T) {
	tx := testTx(t)
	// Use only temporary relations so a regression cannot change shared fixtures.
	_, err := tx.Exec("SET LOCAL search_path TO pg_temp")
	require.NoError(t, err)
	for _, name := range []string{"user_traffic_events", "user_traffic_hourly", "usage_logs"} {
		_, err = tx.Exec("CREATE TEMP TABLE " + name + " AS SELECT 1 AS id")
		require.NoError(t, err)
	}
	sql, err := migrations.FS.ReadFile("398_drop_user_traffic.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		require.NoError(t, archiveRetiredMigrationTables(t.Context(), tx, "398_drop_user_traffic.sql"))
		_, err = tx.Exec(string(sql))
		require.NoError(t, err)
	}
	var removed bool
	require.NoError(t, tx.QueryRow("SELECT to_regclass('user_traffic_events') IS NULL AND to_regclass('user_traffic_hourly') IS NULL").Scan(&removed))
	require.True(t, removed)
	var id int
	for _, table := range []string{"archive_398_user_traffic_events", "archive_398_user_traffic_hourly", "usage_logs"} {
		require.NoError(t, tx.QueryRow("SELECT id FROM "+table).Scan(&id))
		require.Equal(t, 1, id)
	}
}

func TestRetirementRetainsQualityTelemetry(t *testing.T) {
	tx := testTx(t)
	_, err := tx.Exec("SET LOCAL search_path TO pg_temp")
	require.NoError(t, err)
	_, err = tx.Exec("CREATE TEMP TABLE passive_quality_events AS SELECT 42 AS id")
	require.NoError(t, err)
	require.NoError(t, archiveRetiredMigrationTables(t.Context(), tx, "324_retire_passive_quality_protection.sql"))
	// The retirement SQL still needs the source to build the configuration archive.
	var source, archived int
	require.NoError(t, tx.QueryRow("SELECT id FROM passive_quality_events").Scan(&source))
	_, err = tx.Exec("DROP TABLE passive_quality_events")
	require.NoError(t, err)
	require.NoError(t, tx.QueryRow("SELECT id FROM archive_324_passive_quality_events").Scan(&archived))
	require.Equal(t, 42, source)
	require.Equal(t, source, archived)
}
