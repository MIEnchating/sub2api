package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestListRequestDetailsReturnsFirstTokenLatency(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	start := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	filter := &service.OpsRequestDetailFilter{
		StartTime: &start,
		EndTime:   &end,
		Page:      1,
		PageSize:  10,
	}

	mock.ExpectQuery(`SELECT COUNT\(1\) FROM combined`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT\s+kind,\s+created_at,\s+request_id,\s+platform,\s+model,\s+duration_ms,\s+first_token_ms`).
		WithArgs(start, end, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"kind", "created_at", "request_id", "platform", "model", "duration_ms", "first_token_ms",
			"status_code", "error_id", "phase", "severity", "message", "user_id", "api_key_id", "account_id", "group_id", "stream",
		}).AddRow("success", start, "request-1", "openai", "gpt-5", 6800, 1250,
			nil, nil, nil, nil, nil, 1, 2, 105, 8, true))

	items, total, err := repo.ListRequestDetails(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].FirstTokenMs)
	require.Equal(t, 1250, *items[0].FirstTokenMs)
	require.NoError(t, mock.ExpectationsWereMet())
}
