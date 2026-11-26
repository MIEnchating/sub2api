package repository

import (
	"context"
	"database/sql"
	"fmt"
)

// The upstream SQL and its checksum are immutable. Save retired records in
// the same transaction as the retirement, so concurrent writers cannot slip
// rows between the snapshot and DROP, and a failed upgrade leaves no archive.
func archiveRetiredMigrationTables(ctx context.Context, tx *sql.Tx, name string) error {
	var tables []string
	switch name {
	case "324_retire_passive_quality_protection.sql":
		tables = []string{
			"group_quality_report_snapshots", "group_quality_budget_minutes",
			"group_quality_traffic_denials", "group_quality_traffic_snapshots",
			"group_quality_traffic_leases", "passive_quality_leases",
			"passive_quality_states", "passive_quality_events", "passive_quality_failures",
			"passive_quality_cache_reports", "passive_quality_telemetry_gaps",
			"passive_quality_telemetry_gap_scopes", "passive_quality_policies",
			"quality_probe_outcomes", "quality_supplier_accounts", "quality_suppliers",
		}
	case "390_remove_oauth_429_daily_budget.sql":
		tables = []string{"oauth_429_daily_budget"}
	case "398_drop_user_traffic.sql":
		tables = []string{"user_traffic_events", "user_traffic_hourly"}
	default:
		return nil
	}
	for _, table := range tables {
		archive := "archive_" + name[:3] + "_" + table
		if name != "324_retire_passive_quality_protection.sql" {
			// These migrations only drop tables. A rename preserves their rows,
			// indexes and grants without copying traffic history.
			if _, err := tx.ExecContext(ctx, "ALTER TABLE IF EXISTS "+table+" RENAME TO "+archive); err != nil {
				return err
			}
			var shadowed bool
			if err := tx.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&shadowed); err != nil {
				return err
			}
			if shadowed {
				return fmt.Errorf("another %s is visible after archiving", table)
			}
			continue
		}
		// Migration 324 reads the original relations to reconcile active
		// protection and archives settings before dropping the telemetry.
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err := tx.ExecContext(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "SET LOCAL row_security = off"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "CREATE TABLE "+archive+" AS TABLE "+table); err != nil {
			return err
		}
		// Archive rows are for the database owner. Default reader grants must
		// not expose old telemetry through the new table.
		if _, err := tx.ExecContext(ctx, "ALTER TABLE "+archive+" ENABLE ROW LEVEL SECURITY"); err != nil {
			return err
		}
	}
	return nil
}
