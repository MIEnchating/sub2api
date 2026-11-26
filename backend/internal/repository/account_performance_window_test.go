package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountPerformanceWindowChangesApplyToReadsAndEvaluation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	repo := &accountPerformanceRepository{db: db}
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	cfg := service.DefaultPerformanceConfig()
	cfg.Enabled = true
	min := 10.0
	cfg.TPS.Min = &min
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	for _, window := range []struct {
		value   string
		minutes int
	}{{"120", 120}, {"15", 15}, {"1", 1}, {"1440", 1440}} {
		for _, evaluate := range []bool{false, true} {
			if evaluate {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT id FROM accounts.*FOR SHARE").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
				mock.ExpectQuery("SELECT a.id,COALESCE.*FOR UPDATE OF m SKIP LOCKED").WithArgs(int64(7)).WillReturnRows(performanceLockRow(string(raw), false, 0, ""))
				expectPerformanceStats(mock)
			} else {
				expectBaselinePerformance(mock, string(raw), 1, false, 0, "", 7)
			}
			mock.ExpectQuery("SELECT value FROM settings").WithArgs(service.SettingKeyAccountPerformanceWindowMinutes).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(window.value))
			since := now.Add(-time.Duration(window.minutes) * time.Minute)
			mock.ExpectQuery("WITH usage").WithArgs("{7}", since, now).WillReturnRows(performanceRow(cfg))
			if evaluate {
				mock.ExpectRollback()
				require.NoError(t, repo.Evaluate(context.Background(), 7, now))
			} else {
				metrics, err := repo.Read(context.Background(), []int64{7}, now, true)
				require.NoError(t, err)
				require.Equal(t, since, metrics[7].WindowStart)
				require.Equal(t, now, metrics[7].WindowEnd)
			}
		}
	}
	require.NoError(t, mock.ExpectationsWereMet())
	mock.ExpectClose()
}

func TestAccountPerformanceWindowReadFailureDoesNotEvaluateOrPause(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "database_failure", true: "invalid_stored_window"}[invalid], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { require.NoError(t, db.Close()) }()
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM accounts.*FOR SHARE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			raw := `{"version":2,"enabled":true,"tps":{"min":10},"min_samples":20,"auto_recover":true,"cooldown_minutes":5,"recovery_samples":3}`
			mock.ExpectQuery("SELECT a.id,COALESCE.*FOR UPDATE OF m SKIP LOCKED").WillReturnRows(performanceLockRow(raw, false, 0, ""))
			expectPerformanceStats(mock)
			query := mock.ExpectQuery("SELECT value FROM settings").WithArgs(service.SettingKeyAccountPerformanceWindowMinutes)
			if invalid {
				query.WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("0"))
			} else {
				query.WillReturnError(errors.New("settings unavailable"))
			}
			mock.ExpectRollback()
			repo := &accountPerformanceRepository{db: db}
			require.Error(t, repo.Evaluate(context.Background(), 7, time.Now()))
			require.NoError(t, mock.ExpectationsWereMet())
			mock.ExpectClose()
		})
	}
}
