package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/upstreamstats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type accountPerformanceRepository struct {
	db      *sql.DB
	display *accountPerformanceDisplayCache
}

func NewAccountPerformanceRepository(db *sql.DB, rdb *redis.Client, cfg *config.Config) service.AccountPerformanceRepository {
	r := &accountPerformanceRepository{db: db}
	if cfg == nil || rdb == nil {
		return r
	}
	r.display = newAccountPerformanceDisplayCache(rdb, cfg.Database.DBName, func(ctx context.Context, ids []int64, minutes int, now time.Time) (map[int64]service.AccountPerformance, error) {
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='4500ms'; SET LOCAL lock_timeout='500ms'; SET LOCAL work_mem='8MB'; SET LOCAL max_parallel_workers_per_gather=0`); err != nil {
			return nil, err
		}
		metrics, err := readAccountPerformanceWindow(ctx, tx, ids, minutes, now)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return metrics, nil
	})
	return r
}

func (r *accountPerformanceRepository) Stop() {
	if r.display != nil {
		r.display.stop()
	}
}

const performanceStateColumns = `a.id,COALESCE(m.config,'{}'),COALESCE(m.revision,0),COALESCE(m.paused,false),
 COALESCE(m.pause_generation,0),m.paused_at,COALESCE(m.reason,''),m.trial_started_at,
 COALESCE(m.trial_requests,0),m.metrics_since`

const performanceMetricsSQL = `SELECT ` + performanceStateColumns + `
FROM accounts a LEFT JOIN account_performance_monitors m ON a.id=m.account_id
WHERE a.id=ANY($1) AND a.deleted_at IS NULL`

func scanPerformanceState(row interface{ Scan(...any) error }) (service.AccountPerformance, error) {
	m := service.AccountPerformance{Config: service.DefaultPerformanceConfig(), MonitoringAvailable: true}
	var raw []byte
	if err := row.Scan(&m.AccountID, &raw, &m.Revision, &m.Paused, &m.PauseGeneration, &m.PausedAt, &m.Reason, &m.TrialStartedAt, &m.TrialRequests, &m.MetricsSince); err != nil {
		return m, err
	}
	if err := json.Unmarshal(raw, &m.Config); err != nil {
		return m, err
	}
	m.DescribeState()
	return m, nil
}

func (r *accountPerformanceRepository) Read(ctx context.Context, ids []int64, now time.Time, includeMetrics bool) (map[int64]service.AccountPerformance, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out := make(map[int64]service.AccountPerformance, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, performanceMetricsSQL, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		m, err := scanPerformanceState(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[m.AccountID] = m
	}
	err = rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if !includeMetrics {
		return out, nil
	}
	// Most accounts share the rolling hour. Trial/recovered accounts use a fresh
	// baseline so old failures cannot immediately pause a recovered account again.
	type window struct{ start, end time.Time }
	windows := map[window][]int64{}
	for id, m := range out {
		start, end := m.MetricWindow(now)
		w := window{start, end}
		windows[w] = append(windows[w], id)
	}
	for w, ids := range windows {
		stats, err := readOpsAccountRequestStats(ctx, r.db, ids, w.start, w.end)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			m := out[id]
			m.Metrics = service.OpsAccountRequestStats{WindowStart: w.start, WindowEnd: w.end}
			if entry := stats[strconv.FormatInt(id, 10)]; entry != nil {
				m.Metrics = *entry
			}
			out[id] = m
		}
	}
	var observed map[int64]service.AccountPerformance
	if r.display == nil {
		observed, err = readObservedAccountPerformance(ctx, r.db, ids, now)
	} else {
		observed, err = r.readDisplay(ctx, out, now)
	}
	if err != nil {
		for id, m := range out {
			m.StatisticsStatus = "unavailable"
			out[id] = m
		}
		return out, nil
	}
	for id, m := range out {
		if evidence, ok := observed[id]; ok {
			setObservedPerformance(&m, evidence)
			out[id] = m
		}
	}
	return out, nil
}

// Score billed text usage in the selected window. Quality rounds use their
// own completion evidence; a latency sample does not establish stream success.
const performanceObservedSQL = `
WITH usage AS (
 SELECT account_id,
   AVG(output_tokens * 1000.0 / NULLIF(duration_ms, 0)) FILTER (WHERE output_tokens >= 2 AND duration_ms > 0) AS tps,
   COUNT(*) FILTER (WHERE output_tokens >= 2 AND duration_ms > 0) AS tps_n,
   AVG(first_token_ms) FILTER (WHERE stream AND output_tokens >= 2 AND first_token_ms BETWEEN 0 AND duration_ms) AS ttft,
   COUNT(*) FILTER (WHERE stream AND output_tokens >= 2 AND first_token_ms BETWEEN 0 AND duration_ms) AS ttft_n,
   AVG(100.0 * cache_read_tokens / NULLIF(input_tokens::numeric + cache_read_tokens + cache_creation_tokens, 0))
     FILTER (WHERE input_tokens >= 0 AND cache_read_tokens >= 0 AND cache_creation_tokens >= 0) AS cache_percent,
   COUNT(*) FILTER (WHERE input_tokens >= 0 AND cache_read_tokens >= 0 AND cache_creation_tokens >= 0
     AND input_tokens::numeric + cache_read_tokens + cache_creation_tokens > 0) AS cache_n
 FROM usage_logs
 WHERE account_id = ANY($1) AND created_at >= $2 AND created_at <= $3
   AND quality_usage IS NULL AND (http_status IS NULL OR http_status BETWEEN 200 AND 299)
   AND COALESCE(image_count, 0) = 0 AND COALESCE(image_output_tokens, 0) = 0
   AND COALESCE(billing_mode, 'token') NOT IN ('image','video')
 GROUP BY account_id
), rounds AS (
 SELECT p.account_id,p.id,r.quality_round_id,
 bool_or(r.pelican_config @> '{"quality_429_streak":3,"quality_http_status":429}' OR r.quality_judgment->>'verdict'='incorrect' AND r.status='failed' AND r.error_message IN ('answer_mismatch','state_degraded')
    AND (COALESCE(r.pelican_config->>'question_kind','')<>'random_reasoning'
      OR r.pelican_config->'quality_batch'->>'decision'='peer_passed')) AS failed,
 bool_and(COALESCE(r.quality_judgment->>'verdict'='correct' AND r.status='success'
    AND (COALESCE(r.pelican_config->>'question_kind','')<>'random_reasoning'
      OR r.pelican_config->'quality_batch'->>'decision'='passed'),false)) AS passed
 FROM scheduled_test_plans p JOIN scheduled_test_results r ON r.plan_id=p.id
 WHERE p.account_id=ANY($1) AND p.enabled AND jsonb_typeof(p.pelican_config->'quality')='object'
 -- Probe records are created after execution. The indexed completion-time
 -- lower bound preserves the entire round (event-time HAVING below is final).
 AND r.created_at >= $2
 AND r.quality_round_id<>''
 GROUP BY p.account_id,p.id,r.quality_round_id
 HAVING MIN(r.started_at)>=$2 AND MAX(r.finished_at)<=$3 AND MIN(r.started_at)>=MAX(p.updated_at)
 AND bool_and(r.finished_at IS NOT NULL AND r.finished_at>=r.started_at)
 AND bool_and(r.quality_action NOT IN ('','stale_run','comparison_pending'))
 AND bool_and(COALESCE(r.pelican_config->>'parallel_count','1') ~ '^[1-8]$')
 AND bool_and(r.pelican_config->'model_ids' IS NULL OR
  (jsonb_typeof(r.pelican_config->'model_ids')='array'
   AND jsonb_array_length(CASE WHEN jsonb_typeof(r.pelican_config->'model_ids')='array' THEN r.pelican_config->'model_ids' ELSE '[]'::jsonb END)<=50))
 AND COUNT(*)=MAX(CASE WHEN COALESCE(r.pelican_config->>'parallel_count','1') ~ '^[1-8]$'
  THEN COALESCE(r.pelican_config->>'parallel_count','1')::int *
   GREATEST(1,jsonb_array_length(CASE WHEN jsonb_typeof(r.pelican_config->'model_ids')='array' THEN r.pelican_config->'model_ids' ELSE '[]'::jsonb END)) END)
 ), quality AS (
 SELECT account_id,100.0*COUNT(*) FILTER(WHERE failed)/NULLIF(COUNT(*) FILTER(WHERE passed OR failed),0) AS degraded,
 COUNT(*) FILTER(WHERE passed OR failed) AS quality_n FROM rounds GROUP BY account_id
)
SELECT ids.id,u.tps,COALESCE(u.tps_n,0),u.ttft,COALESCE(u.ttft_n,0),q.degraded,COALESCE(q.quality_n,0),
 COALESCE(m.config,'{}'),COALESCE(m.revision,0),COALESCE(m.paused,false),COALESCE(m.pause_generation,0),m.paused_at,COALESCE(m.reason,''),u.cache_percent,COALESCE(u.cache_n,0)
FROM unnest($1::bigint[]) ids(id)
JOIN accounts a ON a.id=ids.id AND a.deleted_at IS NULL
LEFT JOIN usage u ON u.account_id=ids.id LEFT JOIN quality q ON q.account_id=ids.id
LEFT JOIN account_performance_monitors m ON m.account_id=ids.id
`

func readAccountPerformanceWindowMinutes(ctx context.Context, db sqlExecutor) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT value FROM settings WHERE key=$1`, service.SettingKeyAccountPerformanceWindowMinutes)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var value string
	if rows.Next() {
		if err := rows.Scan(&value); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return service.ParseAccountPerformanceWindowMinutes(value)
}

func readObservedAccountPerformance(ctx context.Context, db sqlExecutor, ids []int64, now time.Time) (map[int64]service.AccountPerformance, error) {
	out := make(map[int64]service.AccountPerformance, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	minutes, err := readAccountPerformanceWindowMinutes(ctx, db)
	if err != nil {
		return nil, err
	}
	return readAccountPerformanceWindow(ctx, db, ids, minutes, now)
}

func readAccountPerformanceWindow(ctx context.Context, db sqlExecutor, ids []int64, minutes int, now time.Time) (map[int64]service.AccountPerformance, error) {
	return readAccountPerformanceRange(ctx, db, ids, now.Add(-time.Duration(minutes)*time.Minute), now)
}

func readAccountPerformanceRange(ctx context.Context, db sqlExecutor, ids []int64, since, now time.Time) (map[int64]service.AccountPerformance, error) {
	out := make(map[int64]service.AccountPerformance, len(ids))
	rows, err := db.QueryContext(ctx, performanceObservedSQL, pq.Array(ids), since, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		m := service.AccountPerformance{Config: service.DefaultPerformanceConfig()}
		var config []byte
		if err := rows.Scan(&m.AccountID, &m.AvgTPS, &m.TPSSamples, &m.AvgTTFTMs, &m.TTFTSamples, &m.DegradationPercent, &m.QualitySamples, &config, &m.Revision, &m.Paused, &m.PauseGeneration, &m.PausedAt, &m.Reason, &m.AvgCachePercent, &m.CacheSamples); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(config, &m.Config); err != nil {
			return nil, err
		}
		m.WindowStart, m.WindowEnd = since, now
		out[m.AccountID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Read and the transactional evaluator share the exact process-local
	// response store and configured window, including incomplete windows.
	evidence := upstreamstats.Default.Snapshot(ids, since, now)
	for id, m := range out {
		m.UpstreamError = evidence[id]
		out[id] = m
	}
	return out, nil
}
func setObservedPerformance(m *service.AccountPerformance, evidence service.AccountPerformance) {
	m.AvgTPS, m.AvgTTFTMs, m.AvgCachePercent, m.DegradationPercent = evidence.AvgTPS, evidence.AvgTTFTMs, evidence.AvgCachePercent, evidence.DegradationPercent
	m.TPSSamples, m.TTFTSamples, m.CacheSamples, m.QualitySamples = evidence.TPSSamples, evidence.TTFTSamples, evidence.CacheSamples, evidence.QualitySamples
	m.UpstreamError = evidence.UpstreamError
	m.WindowStart, m.WindowEnd = evidence.WindowStart, evidence.WindowEnd
	m.StatisticsStatus, m.StatisticsUpdatedAt = evidence.StatisticsStatus, evidence.StatisticsUpdatedAt
}

func (r *accountPerformanceRepository) EnabledIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT m.account_id FROM account_performance_monitors m JOIN accounts a ON a.id=m.account_id WHERE m.config->>'enabled'='true' AND a.deleted_at IS NULL ORDER BY m.account_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func performanceRoutingStates(ctx context.Context, db sqlExecutor, ids []int64) (map[int64]service.PerformanceRoutingState, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out := map[int64]service.PerformanceRoutingState{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT account_id, CASE WHEN `+trialEligibleSQL+`
 THEN COALESCE(next_trial_at,trial_started_at) END FROM account_performance_monitors WHERE paused AND account_id=ANY($1) AND NOT EXISTS (SELECT FROM ai_ops_controls c WHERE c.account_id=account_performance_monitors.account_id AND c.ai_paused)
 UNION ALL SELECT account_id,NULL::timestamptz FROM ai_ops_controls WHERE ai_paused AND account_id=ANY($1)`,
		pq.Array(ids), service.PerformanceTrialDuration.Seconds(), service.PerformanceTrialBudgetMultiplier)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var due *time.Time
		if err := rows.Scan(&id, &due); err != nil {
			return nil, err
		}
		out[id] = service.PerformanceRoutingState{Blocked: due == nil, TrialDueAt: due}
	}
	return out, rows.Err()
}
func (r *accountRepository) PerformanceRoutingStates(ctx context.Context, ids []int64) (map[int64]service.PerformanceRoutingState, error) {
	return performanceRoutingStates(ctx, r.sql, ids)
}
func performancePaused(ctx context.Context, db sqlExecutor, ids []int64) (map[int64]bool, error) {
	states, err := performanceRoutingStates(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	paused := make(map[int64]bool, len(states))
	for id, state := range states {
		if state.Blocked {
			paused[id] = true
		}
	}
	return paused, nil
}
func (r *accountPerformanceRepository) Paused(ctx context.Context, ids []int64) (map[int64]bool, error) {
	return performancePaused(ctx, r.db, ids)
}
func (r *accountRepository) PerformancePaused(ctx context.Context, ids []int64) (map[int64]bool, error) {
	return performancePaused(ctx, r.sql, ids)
}
func (r *accountPerformanceRepository) Save(ctx context.Context, id int64, c service.PerformanceConfig, revision, actor int64) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockPerformanceAccount(ctx, tx, id); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_performance_monitors(account_id) VALUES($1) ON CONFLICT DO NOTHING`, id)
	if err != nil {
		return err
	}
	var gen int64
	err = tx.QueryRowContext(ctx, `WITH current AS (
 SELECT account_id, paused AND string_to_array(reason,',') && ARRAY['tps','degradation','observed_ttft','upstream_error'] AS manual_pause
 FROM account_performance_monitors WHERE account_id=$1 AND revision=$3 FOR UPDATE
)
UPDATE account_performance_monitors SET config=$2,revision=revision+1,
 paused=current.manual_pause,paused_at=CASE WHEN current.manual_pause THEN paused_at END,
 reason=CASE WHEN current.manual_pause THEN reason ELSE '' END,
 trial_started_at=NULL,trial_requests=0,next_trial_at=NULL,metrics_since=NULL,updated_at=NOW()
 FROM current WHERE account_performance_monitors.account_id=current.account_id RETURNING pause_generation`, id, string(raw), revision).Scan(&gen)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrPerformanceConflict
	}
	if err != nil {
		return err
	}
	if err = performanceAudit(ctx, tx, id, "configure", actor, revision+1, gen, raw); err != nil {
		return err
	}
	return tx.Commit()
}
func performanceAudit(ctx context.Context, tx *sql.Tx, id int64, action string, actor, revision, gen int64, evidence []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO account_performance_events(account_id,action,actor_id,revision,pause_generation,evidence) VALUES($1,$2,NULLIF($3::bigint,0),$4,$5,$6)`, id, action, actor, revision, gen, string(evidence))
	return err
}

// Soft deletion changes a non-key column: KEY SHARE would allow it through.
// All monitor mutations lock account -> monitor, holding the account active
// until the state change and audit commit together.
func lockPerformanceAccount(ctx context.Context, tx *sql.Tx, id int64) error {
	var existing int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, id).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrPerformanceConflict
	}
	return err
}

func (r *accountPerformanceRepository) Release(ctx context.Context, id, revision, generation, actor int64) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockPerformanceAccount(ctx, tx, id); err != nil {
		return err
	}
	// Explicit release disables evaluation atomically; no old evaluator can re-pause.
	res, err := tx.ExecContext(ctx, `UPDATE account_performance_monitors SET paused=false,paused_at=NULL,reason='',trial_started_at=NULL,trial_requests=0,next_trial_at=NULL,metrics_since=NULL,config=jsonb_set(config,'{enabled}','false'),revision=revision+1,updated_at=NOW() WHERE account_id=$1 AND revision=$2 AND pause_generation=$3 AND paused`, id, revision, generation)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrPerformanceConflict
	}
	if err = performanceAudit(ctx, tx, id, "release", actor, revision+1, generation, []byte(`{"enabled":false}`)); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *accountPerformanceRepository) Evaluate(ctx context.Context, id int64, now time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockPerformanceAccount(ctx, tx, id); err != nil {
		if errors.Is(err, service.ErrPerformanceConflict) {
			return nil
		}
		return err
	}
	m, err := scanPerformanceState(tx.QueryRowContext(ctx, `SELECT `+performanceStateColumns+`
 FROM accounts a JOIN account_performance_monitors m ON a.id=m.account_id WHERE m.account_id=$1 FOR UPDATE OF m SKIP LOCKED`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if m.Phase == service.PerformanceDisabled || (m.Paused && service.PerformanceRequiresManualRelease(m.Reason)) {
		return nil
	}
	if err = m.Config.Validate(); err != nil {
		return err
	}
	if m.Phase == service.PerformancePaused {
		if m.RecoverAfter == nil || now.Before(*m.RecoverAfter) {
			return nil
		}
		_, err = tx.ExecContext(ctx, `UPDATE account_performance_monitors SET trial_started_at=$2,metrics_since=$2,trial_requests=0,next_trial_at=$2,updated_at=NOW() WHERE account_id=$1`, id, now)
		if err != nil {
			return err
		}
		if err = performanceAudit(ctx, tx, id, "trial", 0, m.Revision, m.PauseGeneration, []byte(`{}`)); err != nil {
			return err
		}
		return tx.Commit()
	}
	start, end := m.MetricWindow(now)
	metrics, err := readOpsAccountRequestStats(ctx, tx, []int64{id}, start, end)
	if err != nil {
		return err
	}
	m.Metrics = service.OpsAccountRequestStats{WindowStart: start, WindowEnd: end}
	if entry := metrics[strconv.FormatInt(id, 10)]; entry != nil {
		m.Metrics = *entry
	}
	if m.Config.UsesObservedThresholds() {
		minutes, err := readAccountPerformanceWindowMinutes(ctx, tx)
		if err != nil {
			return err
		}
		since := end.Add(-time.Duration(minutes) * time.Minute)
		if m.MetricsSince != nil && m.MetricsSince.After(since) {
			since = *m.MetricsSince
		}
		observed, err := readAccountPerformanceRange(ctx, tx, []int64{id}, since, end)
		if err != nil {
			return err
		}
		setObservedPerformance(&m, observed[id])
	}
	action, reason := m.MetricDecision(now)
	gen := m.PauseGeneration
	switch action {
	case "pause":
		gen++
		_, err = tx.ExecContext(ctx, `UPDATE account_performance_monitors SET paused=true,pause_generation=pause_generation+1,paused_at=$2,reason=$3,trial_started_at=NULL,trial_requests=0,next_trial_at=NULL,updated_at=NOW() WHERE account_id=$1`, id, now, reason)
	case "recover":
		_, err = tx.ExecContext(ctx, `UPDATE account_performance_monitors SET paused=false,paused_at=NULL,reason='',trial_started_at=NULL,trial_requests=0,next_trial_at=NULL,updated_at=NOW() WHERE account_id=$1`, id)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	evidence, err := json.Marshal(struct {
		service.AccountPerformance
		DecisionReason string `json:"decision_reason,omitempty"`
	}{m, reason})
	if err != nil {
		return err
	}
	if err = performanceAudit(ctx, tx, id, action, 0, m.Revision, gen, evidence); err != nil {
		return err
	}
	return tx.Commit()
}

const trialEligibleSQL = `trial_started_at IS NOT NULL
 AND trial_started_at + make_interval(secs => $2) > NOW()
 AND COALESCE(next_trial_at,trial_started_at) <= NOW()
 AND trial_requests < (config->>'recovery_samples')::int * $3`

// The UPDATE rechecks its predicate under the row lock when gateways race.
// Healthy accounts, including accounts with no rules, never take a write lock.
const performanceTrialSQL = `WITH admitted AS (
 UPDATE account_performance_monitors SET trial_requests=trial_requests+1,next_trial_at=NOW()+INTERVAL '5 seconds'
 WHERE account_id=$1 AND paused AND ` + trialEligibleSQL + ` RETURNING 1
) SELECT EXISTS (SELECT 1 FROM admitted) OR NOT EXISTS (
 SELECT 1 FROM account_performance_monitors WHERE account_id=$1 AND paused)`

func (r *accountRepository) AdmitPerformanceRequest(ctx context.Context, id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rows, err := r.sql.QueryContext(ctx, performanceTrialSQL, id, service.PerformanceTrialDuration.Seconds(), service.PerformanceTrialBudgetMultiplier)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	var allowed bool
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, sql.ErrNoRows
	}
	if err := rows.Scan(&allowed); err != nil {
		return false, err
	}
	return allowed, rows.Close()
}

func (r *accountPerformanceRepository) Events(ctx context.Context, id int64) ([]service.PerformanceEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,action,actor_id,revision,pause_generation,evidence,created_at FROM account_performance_events WHERE account_id=$1 ORDER BY id DESC LIMIT 50`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PerformanceEvent, 0)
	for rows.Next() {
		var e service.PerformanceEvent
		if err := rows.Scan(&e.ID, &e.Action, &e.ActorID, &e.Revision, &e.PauseGeneration, &e.Evidence, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.ReadPauseReason()
		out = append(out, e)
	}
	return out, rows.Err()
}
