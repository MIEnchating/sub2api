package repository

import (
	"context"
	"encoding/json"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountPassRateProtectionSurvivesStaleWritesAndSchedulerProjection(t *testing.T) {
	for _, tt := range []struct {
		name              string
		current, incoming map[string]any
		explicitStatus    bool
	}{
		{"slow refresh preserves new hold", map[string]any{"model-a": "below_threshold"}, nil, false},
		{"stale hold cannot undo recovery", nil, map[string]any{"model-a": "below_threshold"}, false},
		{"stale model cannot replace current owner", map[string]any{"model-a": "below_threshold"}, map[string]any{"model-b": "below_threshold"}, false},
		{"explicit status does not release independent hold", map[string]any{"model-a": "below_threshold"}, nil, true},
		{"injected hold cannot pause unrelated models", nil, map[string]any{"": "below_threshold"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			current := map[string]any{}
			incoming := map[string]any{"setting": "edited"}
			if tt.current != nil {
				current["active_pass_rate_holds"] = tt.current
			}
			if tt.incoming != nil {
				incoming["active_pass_rate_holds"] = tt.incoming
			}
			raw, err := json.Marshal(current)
			require.NoError(t, err)
			mock.ExpectQuery(`(?s)SELECT.*status,.*schedulable,.*FOR NO KEY UPDATE`).
				WithArgs(int64(41), service.PlatformOpenAI, service.AccountTypeOAuth, `{"access_token":"new-token"}`, nil).
				WillReturnRows(sqlmock.NewRows([]string{"identity", "ollama_identity", "proxy_identity", "probe", "sync", "snapshot", "session", "auto", "ollama_snapshot", "opencode_identity", "opencode_auto", "opencode_snapshot", "extra", "status", "schedulable", "error_message", "grok_observed_models", "upstream_model_metadata"}).
					AddRow(false, false, true, nil, nil, nil, nil, nil, nil, false, nil, nil, raw, service.StatusActive, true, "", nil, nil))
			account := &service.Account{ID: 41, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: map[string]any{"access_token": "new-token"}, Extra: incoming, Status: service.StatusActive, Schedulable: true, StatusChanged: tt.explicitStatus}
			account.Extra, err = lockAndMergeAccountProbeExtra(context.Background(), client, account, nil, nil)
			require.NoError(t, err)
			require.Equal(t, "edited", account.Extra["setting"])
			if tt.current == nil {
				require.NotContains(t, account.Extra, "active_pass_rate_holds")
			} else {
				require.Equal(t, tt.current, account.Extra["active_pass_rate_holds"])
			}
			// Exercise the real metadata projection and JSON wire round trip before
			// asking the scheduler gate, not just the whitelist implementation.
			raw, err = json.Marshal(buildSchedulerMetadataAccount(*account))
			require.NoError(t, err)
			var cached service.Account
			require.NoError(t, json.Unmarshal(raw, &cached))
			require.Equal(t, tt.current == nil, cached.IsSchedulableForModelWithContext(context.Background(), "model-a"))
			require.True(t, cached.IsSchedulableForModelWithContext(context.Background(), "model-b"))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAccountQualityProjectionCannotBePatchedThroughGenericWrites(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		for _, injected := range []any{nil, map[string]any{"model-b": "stale hold"}} {
			name := "single"
			if bulk {
				name = "bulk"
			}
			t.Run(name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })
				client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
				t.Cleanup(func() { _ = client.Close() })
				repo := newAccountRepositoryWithSQL(client, db, nil)
				input := map[string]any{"setting": "edited", "active_pass_rate_holds": injected, "passive_quality": injected}
				mock.ExpectBegin()
				update := mock.ExpectExec(`UPDATE accounts SET extra = .*updated_at = NOW\(\)`)
				// Only ordinary settings enter the atomic JSONB patch. The live row's
				// projections must remain authoritative, including concurrent recovery.
				if bulk {
					update.WithArgs([]byte(`{"setting":"edited"}`), "{41}")
				} else {
					update.WithArgs(`{"setting":"edited"}`, int64(41))
				}
				update.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`INSERT INTO scheduler_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
				if bulk {
					_, err = repo.BulkUpdate(context.Background(), []int64{41}, service.AccountBulkUpdate{Extra: input})
				} else {
					err = repo.UpdateExtra(context.Background(), 41, input)
				}
				require.NoError(t, err)
				require.Contains(t, input, "active_pass_rate_holds", "do not mutate the caller's snapshot")
				require.Contains(t, input, "passive_quality")
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
