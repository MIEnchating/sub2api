package qqbot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// These cases exercise the full public confirmation transaction without SMTP or
// network listeners, including failures after the account locks are acquired.
func TestConfirmTransferAuthorizationAndAtomicity(t *testing.T) {
	for _, scenario := range []string{"success", "revoked_binding", "changed_sender_email", "changed_recipient_email", "missing_recipient", "insufficient_balance", "expired_after_lock", "credit_failure", "history_failure"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			s := New(db, nil, true, nil)
			c := DefaultConfig()
			c.Enabled, c.TransferEnabled, c.AppID = true, true, "app"
			raw, err := json.Marshal(c)
			require.NoError(t, err)
			token := randomID() + randomID()
			mock.ExpectBegin()
			mock.ExpectExec("SELECT pg_advisory_xact_lock_shared").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT value FROM settings").WithArgs(configKey).WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(string(raw)))
			mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("app").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT r.from_user_id").WithArgs(codeHash(token), "app").WillReturnRows(sqlmock.NewRows([]string{"from", "to", "from_email", "to_email", "amount", "net", "expires", "member"}).AddRow(1, 2, "sender@example.com", "recipient@example.com", "5.00000000", "4.95000000", time.Now().Add(time.Minute), "member"))
			binding := mock.ExpectQuery("SELECT user_id FROM qq_bot_bindings").WithArgs("app", "member", int64(1))
			if scenario == "revoked_binding" {
				binding.WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
			} else {
				binding.WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(1))
				senderEmail, recipientEmail, balance := "sender@example.com", "recipient@example.com", "10.00000000"
				if scenario == "changed_sender_email" {
					senderEmail = "new@example.com"
				}
				if scenario == "changed_recipient_email" {
					recipientEmail = "new@example.com"
				}
				if scenario == "insufficient_balance" {
					balance = "1.00000000"
				}
				rows := sqlmock.NewRows([]string{"id", "email", "balance"}).AddRow(1, senderEmail, balance)
				if scenario != "missing_recipient" {
					rows.AddRow(2, recipientEmail, "0.00000000")
				}
				mock.ExpectQuery("SELECT id,email,balance::text FROM users.*ORDER BY id FOR NO KEY UPDATE").WithArgs(int64(1), int64(2)).WillReturnRows(rows).RowsWillBeClosed()
				if scenario == "success" || scenario == "expired_after_lock" || scenario == "credit_failure" || scenario == "history_failure" {
					mock.ExpectExec("UPDATE users SET balance=balance-\\$1").WithArgs("5.00000000", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
					credit := mock.ExpectExec("UPDATE users SET balance=balance\\+\\$1").WithArgs("4.95000000", int64(2))
					if scenario == "credit_failure" {
						credit.WillReturnError(errors.New("credit failed"))
					} else {
						credit.WillReturnResult(sqlmock.NewResult(0, 1))
						history := mock.ExpectExec("INSERT INTO qq_bot_transfer_history.*SELECT.*FROM qq_bot_transfer_requests WHERE token_hash=\\$1").WithArgs(codeHash(token))
						if scenario == "history_failure" {
							history.WillReturnError(errors.New("history failed"))
						} else {
							history.WillReturnResult(sqlmock.NewResult(1, 1))
							var affected int64 = 1
							if scenario == "expired_after_lock" {
								affected = 0
							}
							mock.ExpectExec("DELETE FROM qq_bot_transfer_requests.*expires_at>clock_timestamp").WithArgs(codeHash(token)).WillReturnResult(sqlmock.NewResult(0, affected))
						}
					}
				}
			}
			if scenario == "success" {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = s.ConfirmTransfer(context.Background(), token)
			if scenario == "success" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestParseTransferArgsBoundsDecimalBeforeRescaling(t *testing.T) {
	for _, amount := range []string{"1e2147483647", "1e-2147483647", "99999999999", "0.000000001", "-1", "NaN", "1.2.3", "0.00001"} {
		_, _, err := parseTransferArgs(amount + " recipient@example.com")
		require.Error(t, err, amount)
	}
	amount, email, err := parseTransferArgs("1.23456789 recipient@example.com")
	require.NoError(t, err)
	require.Equal(t, "1.2345", amount.StringFixed(4))
	require.Equal(t, "recipient@example.com", email)
}
