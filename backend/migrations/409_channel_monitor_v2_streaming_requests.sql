-- Old buckets mix synchronous and streaming traffic without a transport dimension.
-- Preserve the historical aggregate records before restarting the bounded backfill.
SET LOCAL row_security = off;
DO $$
DECLARE
    source_table text;
    archive_table text;
BEGIN
    FOREACH source_table IN ARRAY ARRAY[
        'channel_monitor_v2_latency_histograms_1m',
        'channel_monitor_v2_error_metrics_1m',
        'channel_monitor_v2_user_metrics_1m',
        'channel_monitor_v2_metrics_1m',
        'channel_monitor_v2_metrics_rollup',
        'channel_monitor_v2_user_metrics_rollup',
        'channel_monitor_v2_latency_histograms_rollup',
        'channel_monitor_v2_error_metrics_rollup'
    ] LOOP
        archive_table := source_table || '_pre_stream';
        EXECUTE format('LOCK TABLE %I IN ACCESS EXCLUSIVE MODE', source_table);
        EXECUTE format('CREATE TABLE %I AS TABLE %I', archive_table, source_table);
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', archive_table);
        EXECUTE format('DELETE FROM %I', source_table);
    END LOOP;
END $$;

UPDATE channel_monitor_v2_watermarks
SET usage_coverage_start = NULL, error_coverage_start = NULL, data_through = NULL,
    last_successful_at = NULL, backfill_cursor = NULL, updated_at = NOW()
WHERE id = 1;

-- This timestamp also came from the unfiltered activity query.
UPDATE group_probes SET last_user_at = NULL WHERE last_user_at IS NOT NULL;
