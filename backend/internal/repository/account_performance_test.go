package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func performanceLockRow(config string, paused bool, generation int64, reason string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"config", "revision", "paused", "pause_generation", "paused_at", "trial_started_at", "reason"}).
		AddRow(config, 1, paused, generation, nil, nil, reason)
}

func expectBaselinePerformance(mock sqlmock.Sqlmock, config string, revision int64, paused bool, generation int64, reason string, ids ...int64) {
	rows := sqlmock.NewRows([]string{"id", "config", "revision", "paused", "pause_generation", "paused_at", "reason", "trial_started_at", "trial_requests", "metrics_since"})
	for _, id := range ids {
		rows.AddRow(id, config, revision, paused, generation, nil, reason, nil, 0, nil)
	}
	mock.ExpectQuery("SELECT a.id,COALESCE").WithArgs(pq.Array(ids)).WillReturnRows(rows)
	mock.ExpectQuery("WITH errors AS MATERIALIZED").WillReturnRows(sqlmock.NewRows([]string{
		"account_id", "cache_rate", "avg_first_token_ms", "avg_duration_ms", "cache_samples", "first_token_samples",
		"duration_samples", "success_rate", "successes", "failures", "samples",
	}))
}

var performanceColumns = []string{"id", "tps", "tps_n", "ttft", "ttft_n", "degraded", "quality_n", "config", "revision", "paused", "pause_generation", "paused_at", "reason", "cache_percent", "cache_n"}

func performanceRow(config service.PerformanceConfig) *sqlmock.Rows {
	raw, _ := json.Marshal(config)
	return sqlmock.NewRows(performanceColumns).AddRow(7, nil, 0, nil, 0, nil, 0, string(raw), 1, false, 0, nil, "", nil, 0)
}
func TestAccountPerformanceQueryBoundsAndMissingSamples(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}()
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT value FROM settings").WithArgs(service.SettingKeyAccountPerformanceWindowMinutes).WillReturnRows(sqlmock.NewRows([]string{"value"}))
	mock.ExpectQuery("WITH source").WithArgs("{7}", now.Add(-60*time.Minute), now).WillReturnRows(performanceRow(service.PerformanceConfig{}))
	got, err := readObservedAccountPerformance(context.Background(), db, []int64{7}, now)
	require.NoError(t, err)
	require.Nil(t, got[7].AvgCachePercent)
	require.Zero(t, got[7].CacheSamples)
	require.Nil(t, got[7].AvgTPS)
	require.Nil(t, got[7].AvgTTFTMs)
	require.Nil(t, got[7].DegradationPercent)
	require.Equal(t, now.Add(-60*time.Minute), got[7].WindowStart)
	require.NoError(t, mock.ExpectationsWereMet())
	// SQL contract only; these assertions do not claim PostgreSQL execution.
	require.Contains(t, performanceObservedSQL, "u.created_at >= $2")
	require.Contains(t, performanceObservedSQL, "\u0027Z$\u0027,\u0027+00:00\u0027")
	require.Contains(t, performanceObservedSQL, "LIMIT 2")
	require.Contains(t, performanceObservedSQL, "event_at >= $2 AND event_at <= $3")
	require.Contains(t, performanceObservedSQL, "MIN(r.started_at)>=$2")
	require.Contains(t, performanceObservedSQL, "NULLIF(COUNT(*) FILTER(WHERE passed OR failed),0)")
}
func TestAccountPerformanceEvaluateNoSamplesAndHeldLock(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing_samples", true: "peer_holds_row"}[locked], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() {
				mock.ExpectClose()
				require.NoError(t, db.Close())
			}()
			repo := &accountPerformanceRepository{db: db}
			min := float64(10)
			cfg := service.DefaultPerformanceConfig()
			cfg.Enabled = true
			cfg.TPS.Min = &min
			raw, _ := json.Marshal(cfg)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM accounts.*FOR SHARE").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			lock := mock.ExpectQuery("SELECT config,revision,paused,pause_generation.*FOR UPDATE SKIP LOCKED").WithArgs(int64(7))
			if locked {
				lock.WillReturnError(sql.ErrNoRows)
			} else {
				lock.WillReturnRows(performanceLockRow(string(raw), false, 0, ""))
				expectBaselinePerformance(mock, string(raw), 1, false, 0, "", 7)
				mock.ExpectQuery("SELECT value FROM settings").WillReturnRows(sqlmock.NewRows([]string{"value"}))
				mock.ExpectQuery("WITH source").WillReturnRows(performanceRow(cfg))
			}
			mock.ExpectRollback()
			require.NoError(t, repo.Evaluate(context.Background(), 7, time.Now()))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestAccountPerformanceEvaluatePauseAuditAtomicAndLatched(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}()
	repo := &accountPerformanceRepository{db: db}
	min := float64(10)
	cfg := service.DefaultPerformanceConfig()
	cfg.Enabled = true
	cfg.TPS.Min = &min
	raw, _ := json.Marshal(cfg)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM accounts.*FOR SHARE").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery("SELECT config,revision,paused,pause_generation.*FOR UPDATE SKIP LOCKED").WithArgs(int64(7)).WillReturnRows(performanceLockRow(string(raw), false, 0, ""))
	expectBaselinePerformance(mock, string(raw), 1, false, 0, "", 7)
	mock.ExpectQuery("SELECT value FROM settings").WillReturnRows(sqlmock.NewRows([]string{"value"}))
	mock.ExpectQuery("WITH source").WillReturnRows(sqlmock.NewRows(performanceColumns).AddRow(7, 5, 2, nil, 0, nil, 0, string(raw), 1, false, 0, nil, "", nil, 0))
	mock.ExpectExec("UPDATE account_performance_monitors SET paused=true").WithArgs(int64(7), now, "tps").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO account_performance_events").WithArgs(int64(7), "pause", int64(0), int64(1), int64(1), sqlmock.AnyArg()).WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	require.Error(t, repo.Evaluate(context.Background(), 7, now))
	// Already paused skips metrics and any write; no healthy/expired samples can reopen it.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM accounts.*FOR SHARE").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery("SELECT config,revision,paused,pause_generation.*FOR UPDATE SKIP LOCKED").WillReturnRows(performanceLockRow(string(raw), true, 1, "tps"))
	mock.ExpectRollback()
	require.NoError(t, repo.Evaluate(context.Background(), 7, now.Add(time.Hour)))
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestAccountPerformanceReleaseRequiresExactOwnershipAndDisablesEvaluation(t *testing.T) {
	for _, n := range []int64{0, 1} {
		t.Run(map[int64]string{0: "stale", 1: "current"}[n], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() {
				mock.ExpectClose()
				require.NoError(t, db.Close())
			})
			repo := &accountPerformanceRepository{db: db}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM accounts.*FOR SHARE").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			mock.ExpectExec("UPDATE account_performance_monitors SET paused=false.*config=jsonb_set.*revision=revision\\+1.*WHERE account_id=\\$1 AND revision=\\$2 AND pause_generation=\\$3 AND paused").WithArgs(int64(7), int64(2), int64(3)).WillReturnResult(sqlmock.NewResult(0, n))
			if n == 1 {
				mock.ExpectExec("INSERT INTO account_performance_events").WithArgs(int64(7), "release", int64(9), int64(3), int64(3), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = repo.Release(context.Background(), 7, 2, 3, 9)
			if n == 0 {
				require.ErrorIs(t, err, service.ErrPerformanceConflict)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestAccountPerformanceMigrationSequenceAndSchema(t *testing.T) {
	files, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	sort.Strings(files)
	target := "339_account_performance_monitor.sql"
	i := sort.SearchStrings(files, target)
	require.Less(t, i, len(files))
	require.Equal(t, target, files[i])
	require.True(t, strings.HasPrefix(files[i-1], "338_"))
	data, err := fs.ReadFile(migrations.FS, target)
	require.NoError(t, err)
	notx, err := validateMigrationExecutionMode(target, string(data))
	require.NoError(t, err)
	require.False(t, notx)
	require.NotContains(t, strings.ToUpper(string(data)), "UPDATE ACCOUNTS")
	require.Contains(t, string(data), "account_performance_monitors")
	require.Contains(t, string(data), "account_performance_events")
	for file, fields := range map[string][]string{"066_add_scheduled_test_tables.sql": {"started_at", "finished_at", "updated_at"}, "243_pelican_scheduled_tests.sql": {"pelican_config"}, "247_account_quality_ops.sql": {"quality_action"}, "248_account_quality_judgment.sql": {"quality_judgment", "quality_round_id"}, "010_add_usage_logs_aggregated_indexes.sql": {"account_id, created_at"}} {
		source, err := fs.ReadFile(migrations.FS, file)
		require.NoError(t, err)
		for _, field := range fields {
			require.Contains(t, string(source), field)
		}
	}
}
