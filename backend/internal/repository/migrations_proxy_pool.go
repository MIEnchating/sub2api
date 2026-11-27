package repository

import (
	"context"
	"database/sql"
)

func executeTransactionalMigration(ctx context.Context, tx *sql.Tx, name, content string) error {
	switch name {
	case "302_qq_bot_image_intent.sql":
		// Keep the immutable migration identity and the source reference used
		// by text/vision history. Fresh upgrades exclude the fork's generation
		// plans. Already-applied migrations retain their tables and records.
		content = "ALTER TABLE qq_bot_message_references ADD COLUMN IF NOT EXISTS source_job_id TEXT NOT NULL DEFAULT ''"
	case "404_retire_vip_credit_and_account_multi_proxy.sql":
		// Keep the original migration identity and credit retirement for fresh
		// upgrades. The proxy retirement is superseded by 405. Even a temporary
		// route rewrite can fire irreversible managed-IP retirement triggers.
		content = "UPDATE auto_credit_config SET enabled = false, version = version + 1 WHERE enabled"
	}
	_, err := tx.ExecContext(ctx, content)
	return err
}
