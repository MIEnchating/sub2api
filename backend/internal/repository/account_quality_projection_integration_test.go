//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountQualityProjectionsRemainOwnedByDurableState(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	active := map[string]any{"model-a": "below_threshold"}
	passive := []any{map[string]any{"model": "model-a", "phase": "paused"}}
	snapshot := map[string]any{"active_pass_rate_holds": active, "passive_quality": passive, "setting": "original",
		"active_test_model_holds": active, "active_test_all_models_paused": true}
	account := &service.Account{
		Name: "quality-projection-copy", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{}, Extra: snapshot,
	}
	// Imported/duplicated accounts have no durable ownership; do not copy holds.
	require.NoError(t, repo.Create(ctx, account))
	stored, err := tx.Client().Account.Get(ctx, account.ID)
	require.NoError(t, err)
	require.NotContains(t, stored.Extra, "active_pass_rate_holds")
	require.NotContains(t, stored.Extra, "passive_quality")
	require.NotContains(t, stored.Extra, "active_test_model_holds")
	require.NotContains(t, stored.Extra, "active_test_all_models_paused")
	require.Contains(t, snapshot, "active_pass_rate_holds")

	// Simulate the dedicated owner's committed projection, then stale patches.
	_, err = tx.Client().Account.UpdateOneID(account.ID).SetExtra(snapshot).Save(ctx)
	require.NoError(t, err)
	for _, bulk := range []bool{false, true} {
		for _, hold := range []any{nil, map[string]any{"model-b": "stale"}} {
			patch := map[string]any{"active_pass_rate_holds": hold, "passive_quality": hold, "setting": "edited",
				"active_test_model_holds": hold, "active_test_all_models_paused": hold}
			if bulk {
				_, err = repo.BulkUpdate(ctx, []int64{account.ID}, service.AccountBulkUpdate{Extra: patch})
			} else {
				err = repo.UpdateExtra(ctx, account.ID, patch)
			}
			require.NoError(t, err)
			stored, err = tx.Client().Account.Get(ctx, account.ID)
			require.NoError(t, err)
			require.Equal(t, active, stored.Extra["active_pass_rate_holds"])
			require.Equal(t, passive, stored.Extra["passive_quality"])
			require.Equal(t, active, stored.Extra["active_test_model_holds"])
			require.Equal(t, true, stored.Extra["active_test_all_models_paused"])
			require.Equal(t, "edited", stored.Extra["setting"])
		}
	}
	// Once the owner recovers, an old patch cannot resurrect either restriction.
	_, err = tx.Client().Account.UpdateOneID(account.ID).SetExtra(map[string]any{}).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateExtra(ctx, account.ID, snapshot))
	_, err = repo.BulkUpdate(ctx, []int64{account.ID}, service.AccountBulkUpdate{Extra: snapshot})
	require.NoError(t, err)
	stored, err = tx.Client().Account.Get(ctx, account.ID)
	require.NoError(t, err)
	require.NotContains(t, stored.Extra, "active_pass_rate_holds")
	require.NotContains(t, stored.Extra, "passive_quality")
	require.NotContains(t, stored.Extra, "active_test_model_holds")
	require.NotContains(t, stored.Extra, "active_test_all_models_paused")
}
