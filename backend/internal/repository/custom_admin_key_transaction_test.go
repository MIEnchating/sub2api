//go:build unit

package repository

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestScopedAdminLockAccountPreservesProtectionLockOrder(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(map[bool]string{true: "existing", false: "missing"}[exists], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			repo := newAccountRepositoryWithSQL(client, db, nil)
			require.Error(t, repo.ScopedAdminLockAccount(context.Background(), 31))
			mock.ExpectBegin()
			tx, err := client.Tx(context.Background())
			require.NoError(t, err)
			// sqlmock checks order: membership gate, plans, then the account.
			mock.ExpectExec(`SELECT pg_advisory_xact_lock\(1937006960,1735553655\)`).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`SELECT p.id FROM scheduled_test_plans p`).WithArgs(int64(31)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			rows := sqlmock.NewRows([]string{"id"})
			if exists {
				rows.AddRow(31)
			}
			mock.ExpectQuery(`SELECT id FROM accounts WHERE id=\$1 AND deleted_at IS NULL FOR NO KEY UPDATE`).WithArgs(int64(31)).WillReturnRows(rows)
			err = repo.ScopedAdminLockAccount(dbent.NewTxContext(context.Background(), tx), 31)
			if exists {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, service.ErrAccountNotFound)
			}
			mock.ExpectRollback()
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
