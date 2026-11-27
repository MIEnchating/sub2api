//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestRetireVIPCreditAndAccountMultiProxy(t *testing.T) {
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
CREATE TEMP TABLE accounts (
 id bigint PRIMARY KEY, extra jsonb DEFAULT '{}', proxy_id bigint, schedulable boolean DEFAULT true,
 updated_at timestamptz DEFAULT now(), deleted_at timestamptz, parent_account_id bigint
);
CREATE TEMP TABLE proxies (id bigint PRIMARY KEY, protocol text, status text, expires_at timestamptz, deleted_at timestamptz);
CREATE TEMP TABLE account_proxies (account_id bigint, proxy_id bigint, position int, created_at timestamptz DEFAULT now(), PRIMARY KEY (account_id, proxy_id));
CREATE TEMP TABLE scheduler_outbox (event_type text, account_id bigint);
CREATE TEMP TABLE auto_credit_config (enabled boolean, version bigint);
CREATE TEMP TABLE auto_credit_ledger (id bigint, amount numeric);
CREATE TEMP TABLE schema_migrations (filename text PRIMARY KEY);
CREATE TEMP TABLE openai_vm_bindings (account_id bigint, managed_proxy_id bigint);
CREATE TEMP TABLE openai_ipv6_bindings (account_id bigint, managed_proxy_id bigint);
CREATE TEMP TABLE ovh_ipv6_leases (owner_account_id bigint, proxy_id bigint);
INSERT INTO auto_credit_config VALUES (true, 7), (false, 5);
INSERT INTO auto_credit_ledger VALUES (1, 42.50);
INSERT INTO proxies VALUES (11, 'https', 'active', NULL, NULL), (12, 'socks5', 'active', NULL, NULL),
 (13, 'http', 'active', now() - interval '1 day', NULL);
INSERT INTO accounts (id, extra, proxy_id) VALUES
 (1, '{"other":"kept"}', 12),
 (2, '{"openai_proxy_mode":"request","openai_proxy_ids":[13,11],"openai_excel_bps":true}', NULL),
 (3, '{"openai_excel_bps_proxy_mode":"request","openai_excel_bps_proxy_ids":[13],"openai_excel_bps":true}', NULL),
 (4, '{"openai_proxy_mode":"single","openai_proxy_ids":[11]}', NULL),
 (5, '{}', NULL),
 (6, '{"openai_excel_bps_proxy_mode":"request","openai_excel_bps_proxy_ids":[11],"openai_excel_bps":false}', NULL);
INSERT INTO account_proxies (account_id, proxy_id, position) VALUES (1,11,0),(1,12,1);
`)
	require.NoError(t, err)
	migration, err := FS.ReadFile("404_retire_vip_credit_and_account_multi_proxy.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	for _, check := range []string{
		"SELECT NOT EXISTS (SELECT 1 FROM auto_credit_config WHERE enabled) AND EXISTS (SELECT 1 FROM auto_credit_config WHERE version=8)",
		"SELECT amount=42.50 FROM auto_credit_ledger WHERE id=1",
		"SELECT count(*)=2 FROM retired_account_proxy_bindings WHERE account_id=1",
		"SELECT count(*)=0 FROM account_proxies",
		"SELECT proxy_id=12 AND schedulable AND extra->>'other'='kept' FROM accounts WHERE id=1",
		`SELECT proxy_id=11 AND schedulable AND extra->'openai_excel_bps'='true' AND extra->'retired_request_proxy'->'openai_proxy_ids'='[13,11]' AND NOT extra ? 'openai_proxy_mode' FROM accounts WHERE id=2`,
		"SELECT proxy_id IS NULL AND NOT schedulable AND extra->'retired_multi_proxy_needs_review'='true' FROM accounts WHERE id=3",
		"SELECT count(*)=3 FROM accounts WHERE id IN (4,5,6) AND proxy_id IS NULL AND schedulable",
		"SELECT count(*)=5 FROM scheduler_outbox WHERE event_type='account_changed'",
	} {
		var ok bool
		require.NoError(t, tx.QueryRowContext(ctx, check).Scan(&ok), check)
		require.True(t, ok, check)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(filename) VALUES ('404_retire_vip_credit_and_account_multi_proxy.sql')`)
	require.NoError(t, err)
	restore, err := FS.ReadFile("405_restore_account_multi_proxy.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(restore))
	require.NoError(t, err)
	for _, check := range []string{
		"SELECT count(*)=2 FROM account_proxies WHERE account_id=1",
		"SELECT proxy_id=11 AND schedulable AND extra->'openai_proxy_ids'='[13,11]' AND extra ? 'retired_request_proxy' FROM accounts WHERE id=2",
		"SELECT proxy_id IS NULL AND NOT schedulable AND extra->'retired_multi_proxy_needs_review'='true' FROM accounts WHERE id=3",
		"SELECT count(*)=2 FROM scheduler_outbox WHERE event_type='account_changed' AND account_id=1",
	} {
		var ok bool
		require.NoError(t, tx.QueryRowContext(ctx, check).Scan(&ok), check)
		require.True(t, ok, check)
	}
}
