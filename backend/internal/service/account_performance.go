package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/upstreamstats"
)

var ErrPerformanceConflict = errors.New("performance monitor changed; reload before saving")

type PerformanceBounds struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

func (b PerformanceBounds) Validate(percent bool) error {
	for _, v := range []*float64{b.Min, b.Max} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 1e9 || (percent && *v > 100)) {
			return fmt.Errorf("invalid performance threshold")
		}
	}
	if b.Min != nil && b.Max != nil && *b.Min > *b.Max {
		return fmt.Errorf("minimum exceeds maximum")
	}
	return nil
}
func (b PerformanceBounds) Breached(v *float64) bool {
	return v != nil && ((b.Min != nil && *v < *b.Min) || (b.Max != nil && *v > *b.Max))
}

// Empty bounds disable a metric. Percentages use 0..100; latency uses milliseconds.
type PerformanceRule struct {
	Pause    PerformanceBounds `json:"pause"`
	Recovery PerformanceBounds `json:"recovery"`
}

func (r PerformanceRule) Enabled() bool { return r.Pause.Min != nil || r.Pause.Max != nil }
func (r PerformanceRule) RecoveryBounds() PerformanceBounds {
	b := r.Recovery
	if b.Min == nil {
		b.Min = r.Pause.Min
	}
	if b.Max == nil {
		b.Max = r.Pause.Max
	}
	return b
}

type PerformanceConfig struct {
	Version         int               `json:"version"`
	Enabled         bool              `json:"enabled"`
	SuccessRate     PerformanceRule   `json:"success_rate"`
	CacheRate       PerformanceRule   `json:"cache_rate"`
	TTFT            PerformanceRule   `json:"ttft"`
	Duration        PerformanceRule   `json:"duration"`
	TPS             PerformanceBounds `json:"tps"`
	Degradation     PerformanceBounds `json:"degradation"`
	ObservedTTFT    PerformanceBounds `json:"observed_ttft"`
	UpstreamError   PerformanceBounds `json:"upstream_error"`
	MinSamples      int64             `json:"min_samples"`
	AutoRecover     bool              `json:"auto_recover"`
	CooldownMinutes int               `json:"cooldown_minutes"`
	RecoverySamples int64             `json:"recovery_samples"`
}

func DefaultPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{Version: 2, MinSamples: 20, AutoRecover: true, CooldownMinutes: 5, RecoverySamples: 3}
}
func (c PerformanceConfig) Validate() error {
	if c.Version != 2 {
		return fmt.Errorf("reload the metric configuration before saving")
	}
	if c.MinSamples < 1 || c.MinSamples > 10000 || c.RecoverySamples < 1 || c.RecoverySamples > 100 || c.CooldownMinutes < 1 || c.CooldownMinutes > 1440 {
		return fmt.Errorf("invalid sample count or cooldown")
	}
	enabled := false
	for i, r := range []PerformanceRule{c.SuccessRate, c.CacheRate, c.TTFT, c.Duration} {
		if err := r.Pause.Validate(i < 2); err != nil {
			return err
		}
		if err := r.Recovery.Validate(i < 2); err != nil {
			return err
		}
		if !r.Enabled() && (r.Recovery.Min != nil || r.Recovery.Max != nil) {
			return fmt.Errorf("recovery requires a pause threshold")
		}
		b := r.RecoveryBounds()
		if err := b.Validate(i < 2); err != nil {
			return err
		}
		if (r.Pause.Min != nil && b.Min != nil && *b.Min < *r.Pause.Min) || (r.Pause.Max != nil && b.Max != nil && *b.Max > *r.Pause.Max) {
			return fmt.Errorf("recovery thresholds must be within the normal range")
		}
		enabled = enabled || r.Enabled()
	}
	for i, bounds := range c.observedBounds() {
		if err := bounds.Validate(i == 1 || i == 3); err != nil {
			return err
		}
		enabled = enabled || bounds.Min != nil || bounds.Max != nil
	}
	if c.Enabled && !enabled {
		return fmt.Errorf("select at least one metric")
	}
	return nil
}

type AccountPerformance struct {
	// Display-only cache state. Safety evaluation always reads live SQL evidence.
	StatisticsStatus    string                 `json:"statistics_status,omitempty"`
	StatisticsUpdatedAt *time.Time             `json:"statistics_updated_at,omitempty"`
	UpstreamError       upstreamstats.Evidence `json:"upstream_error"`
	AvgTPS              *float64               `json:"avg_tps"`
	DegradationPercent  *float64               `json:"degradation_percent"`
	AvgTTFTMs           *float64               `json:"avg_ttft_ms"`
	AvgCachePercent     *float64               `json:"avg_cache_percent"`
	CacheSamples        int                    `json:"cache_samples"`
	TPSSamples          int                    `json:"tps_samples"`
	TTFTSamples         int                    `json:"ttft_samples"`
	QualitySamples      int                    `json:"quality_samples"`
	WindowStart         time.Time              `json:"window_start"`
	WindowEnd           time.Time              `json:"window_end"`
	MonitoringAvailable bool                   `json:"monitoring_available"`
	AccountID           int64                  `json:"account_id"`
	Metrics             OpsAccountRequestStats `json:"metrics"`
	Config              PerformanceConfig      `json:"config"`
	Revision            int64                  `json:"revision"`
	Paused              bool                   `json:"paused"`
	PauseGeneration     int64                  `json:"pause_generation"`
	PausedAt            *time.Time             `json:"paused_at"`
	TrialStartedAt      *time.Time             `json:"trial_started_at"`
	TrialRequests       int64                  `json:"trial_requests"`
	MetricsSince        *time.Time             `json:"metrics_since"`
	Reason              string                 `json:"reason"`
}

// Snapshot callers own the returned evidence; do not expose cache pointers.
func cloneAccountPerformance(in *AccountPerformance) *AccountPerformance {
	if in == nil {
		return nil
	}
	out := *in
	clone := func(p *float64) *float64 {
		if p == nil {
			return nil
		}
		value := *p
		return &value
	}
	out.AvgTPS, out.AvgTTFTMs, out.DegradationPercent = clone(in.AvgTPS), clone(in.AvgTTFTMs), clone(in.DegradationPercent)
	out.AvgCachePercent = clone(in.AvgCachePercent)
	if in.StatisticsUpdatedAt != nil {
		at := *in.StatisticsUpdatedAt
		out.StatisticsUpdatedAt = &at
	}
	out.UpstreamError.Percent = clone(in.UpstreamError.Percent)
	for _, bounds := range []*PerformanceBounds{&out.Config.TPS, &out.Config.ObservedTTFT, &out.Config.Degradation, &out.Config.UpstreamError} {
		bounds.Min, bounds.Max = clone(bounds.Min), clone(bounds.Max)
	}
	for _, rule := range []*PerformanceRule{&out.Config.SuccessRate, &out.Config.CacheRate, &out.Config.TTFT, &out.Config.Duration} {
		rule.Pause.Min, rule.Pause.Max = clone(rule.Pause.Min), clone(rule.Pause.Max)
		rule.Recovery.Min, rule.Recovery.Max = clone(rule.Recovery.Min), clone(rule.Recovery.Max)
	}
	out.TrialStartedAt = clonePerformanceTime(in.TrialStartedAt)
	out.MetricsSince = clonePerformanceTime(in.MetricsSince)
	out.Metrics.SuccessRate = clone(in.Metrics.SuccessRate)
	out.Metrics.CacheRate = clone(in.Metrics.CacheRate)
	out.Metrics.AvgFirstTokenMs = clone(in.Metrics.AvgFirstTokenMs)
	out.Metrics.AvgDurationMs = clone(in.Metrics.AvgDurationMs)
	if in.PausedAt != nil {
		value := *in.PausedAt
		out.PausedAt = &value
	}
	return &out
}

// Shared validation for display evidence and scheduling. Unknown is not zero.
func validPerformanceMetric(value *float64) bool {
	return value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0
}

func performanceUpstreamErrorRate(e upstreamstats.Evidence) *float64 {
	if e.Source != "process_local_http_responses" || e.Total <= 0 || e.Failures < 0 || e.Failures > e.Total || !validPerformanceMetric(e.Percent) || *e.Percent > 100 {
		return nil
	}
	value := float64(e.Failures) / float64(e.Total)
	return &value
}

func clonePerformanceTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (c PerformanceConfig) observedBounds() [4]PerformanceBounds {
	return [4]PerformanceBounds{c.TPS, c.Degradation, c.ObservedTTFT, c.UpstreamError}
}

func (c PerformanceConfig) UsesObservedThresholds() bool {
	for _, bounds := range c.observedBounds() {
		if bounds.Min != nil || bounds.Max != nil {
			return true
		}
	}
	return false
}

// The observed thresholds retain their existing manual-release policy.
// An automatic trial may only clear a pause caused by bounded recovery rules.
func PerformanceRequiresManualRelease(reason string) bool {
	for _, metric := range strings.Split(reason, ",") {
		switch metric {
		case "tps", "degradation", "observed_ttft", "upstream_error":
			return true
		}
	}
	return false
}

func (m AccountPerformance) observedBreaches() []string {
	var reasons []string
	for _, sample := range []struct {
		name   string
		count  int
		value  *float64
		bounds PerformanceBounds
	}{
		{"tps", m.TPSSamples, m.AvgTPS, m.Config.TPS},
		{"degradation", m.QualitySamples, m.DegradationPercent, m.Config.Degradation},
		{"observed_ttft", m.TTFTSamples, m.AvgTTFTMs, m.Config.ObservedTTFT},
	} {
		if sample.count > 0 && validPerformanceMetric(sample.value) && sample.bounds.Breached(sample.value) {
			reasons = append(reasons, sample.name)
		}
	}
	if rate := performanceUpstreamErrorRate(m.UpstreamError); rate != nil {
		ratePercent := percent(rate)
		if m.Config.UpstreamError.Breached(ratePercent) {
			reasons = append(reasons, "upstream_error")
		}
	}
	return reasons
}

type performanceMeasurement struct {
	name    string
	rule    PerformanceRule
	value   *float64
	samples int64
}

func percent(v *float64) *float64 {
	if v == nil {
		return nil
	}
	n := *v * 100
	return &n
}
func (m AccountPerformance) measurements() []performanceMeasurement {
	return []performanceMeasurement{
		{"success_rate", m.Config.SuccessRate, percent(m.Metrics.SuccessRate), m.Metrics.RequestSamples},
		{"cache_rate", m.Config.CacheRate, percent(m.Metrics.CacheRate), m.Metrics.CacheSamples},
		{"ttft", m.Config.TTFT, m.Metrics.AvgFirstTokenMs, m.Metrics.FirstTokenSamples},
		{"duration", m.Config.Duration, m.Metrics.AvgDurationMs, m.Metrics.DurationSamples},
	}
}
func (m AccountPerformance) Breaches() []string {
	var reasons []string
	minimum := m.Config.MinSamples
	if m.TrialStartedAt != nil {
		minimum = m.Config.RecoverySamples
	}
	for _, v := range m.measurements() {
		if v.rule.Enabled() && v.samples >= minimum && validPerformanceMetric(v.value) && v.rule.Pause.Breached(v.value) {
			reasons = append(reasons, v.name)
		}
	}
	return append(reasons, m.observedBreaches()...)
}
func (m AccountPerformance) ReadyForRecovery() bool {
	enabled := false
	for _, v := range m.measurements() {
		if !v.rule.Enabled() {
			continue
		}
		enabled = true
		if v.samples < m.Config.RecoverySamples || !validPerformanceMetric(v.value) || v.rule.RecoveryBounds().Breached(v.value) {
			return false
		}
	}
	return enabled
}

// Trial admission is persistent and atomic across instances. Cancel is called
// only when later concurrency/admission checks reject the request.
type PerformanceAdmission interface {
	AcquirePerformanceTrial(context.Context, int64) (allowed bool, cancel func(), err error)
}

type PerformanceEvent struct {
	ID              int64           `json:"id"`
	Action          string          `json:"action"`
	ActorID         *int64          `json:"actor_id"`
	Revision        int64           `json:"revision"`
	PauseGeneration int64           `json:"pause_generation"`
	Evidence        json.RawMessage `json:"evidence"`
	CreatedAt       time.Time       `json:"created_at"`
}
type AccountPerformanceRepository interface {
	Events(context.Context, int64) ([]PerformanceEvent, error)
	Read(context.Context, []int64, time.Time) (map[int64]AccountPerformance, error)
	Save(context.Context, int64, PerformanceConfig, int64, int64) error
	Release(context.Context, int64, int64, int64, int64) error
	Evaluate(context.Context, int64, time.Time) error
	EnabledIDs(context.Context) ([]int64, error)
	Paused(context.Context, []int64) (map[int64]bool, error)
}

type AccountPerformanceService struct {
	Repo   AccountPerformanceRepository
	ops    *OpsService
	cancel context.CancelFunc
	wg     sync.WaitGroup
	// Only the evaluator goroutine owns this cursor. Advance on attempts,
	// including timeouts, so slow accounts cannot starve the remaining IDs.
	evaluationCursor int64
}

func NewAccountPerformanceService(repo AccountPerformanceRepository) *AccountPerformanceService {
	return &AccountPerformanceService{Repo: repo}
}
func (s *AccountPerformanceService) MonitoringAvailable(ctx context.Context) bool {
	return s.ops == nil || s.ops.IsMonitoringEnabled(ctx)
}
func (s *AccountPerformanceService) Read(ctx context.Context, ids []int64, now time.Time) (map[int64]AccountPerformance, error) {
	rows, err := s.Repo.Read(ctx, ids, now)
	if err != nil {
		return nil, err
	}
	available := s.MonitoringAvailable(ctx)
	for id, m := range rows {
		m.MonitoringAvailable = available
		if !available {
			m.Metrics = OpsAccountRequestStats{WindowStart: m.Metrics.WindowStart, WindowEnd: m.Metrics.WindowEnd}
			m.AvgTPS, m.AvgTTFTMs, m.AvgCachePercent, m.DegradationPercent = nil, nil, nil, nil
			m.TPSSamples, m.TTFTSamples, m.CacheSamples, m.QualitySamples = 0, 0, 0, 0
			m.UpstreamError = upstreamstats.Evidence{}
			m.StatisticsStatus, m.StatisticsUpdatedAt = "unavailable", nil
		}
		rows[id] = m
	}
	return rows, nil
}
func (s *AccountPerformanceService) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.evaluate(ctx)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.evaluate(ctx)
			}
		}
	}()
}
func (s *AccountPerformanceService) Stop() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	if stopper, ok := s.Repo.(interface{ Stop() }); ok {
		stopper.Stop()
	}
}
func (s *AccountPerformanceService) evaluate(ctx context.Context) {
	if !s.MonitoringAvailable(ctx) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	ids, err := s.Repo.EnabledIDs(ctx)
	if err != nil {
		slog.Error("account performance list", "error", err)
		return
	}
	start := 0
	for i, id := range ids {
		if id > s.evaluationCursor {
			start = i
			break
		}
	}
	for offset := range ids {
		if ctx.Err() != nil {
			return
		}
		id := ids[(start+offset)%len(ids)]
		s.evaluationCursor = id
		if err := s.Repo.Evaluate(ctx, id, time.Now()); err != nil {
			slog.Error("account performance evaluate", "account_id", id, "error", err)
		}
	}
}

// Live authority check is intentionally outside all Redis/regional snapshots.
// A database error fails closed; cached permission cannot override an auto pause.
type PerformancePauseReader interface {
	PerformancePaused(context.Context, []int64) (map[int64]bool, error)
}

// A subscriber's permission comes only from its authenticated, expiring local
// snapshot. Missing accounts are denials, never a lookup in the local database.
type localPerformancePauseReader struct{ reader LocalSchedulerSnapshotReader }

func (r localPerformancePauseReader) PerformancePaused(ctx context.Context, ids []int64) (map[int64]bool, error) {
	details, ok := r.reader.(LocalSchedulerDetailReader)
	if !ok {
		return nil, ErrLocalSchedulerSnapshotUnavailable
	}
	accounts, err := details.GetLocalAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	paused := make(map[int64]bool, len(ids))
	for _, id := range ids {
		a := accounts[id]
		paused[id] = a == nil || !a.IsSchedulable()
	}
	return paused, nil
}

func (s *SchedulerSnapshotService) filterPerformanceAccounts(ctx context.Context, accounts []Account) ([]Account, error) {
	if s.localSnapshotReader != nil {
		return accounts, nil
	}
	reader, ok := s.accountRepo.(PerformancePauseReader)
	if !ok || len(accounts) == 0 {
		return accounts, nil
	}
	ids := make([]int64, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	paused, err := reader.PerformancePaused(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(accounts))
	for _, a := range accounts {
		if !a.PerformancePaused && !paused[a.ID] {
			out = append(out, a)
		}
	}
	return out, nil
}
func (s *SchedulerSnapshotService) markPerformanceAccounts(ctx context.Context, accounts map[int64]*Account) (map[int64]*Account, error) {
	if s.localSnapshotReader != nil {
		return accounts, nil
	}
	reader, ok := s.accountRepo.(PerformancePauseReader)
	if !ok || len(accounts) == 0 {
		return accounts, nil
	}
	ids := make([]int64, 0, len(accounts))
	for id := range accounts {
		ids = append(ids, id)
	}
	paused, err := reader.PerformancePaused(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*Account, len(accounts))
	for id, a := range accounts {
		if a != nil {
			copy := *a
			copy.PerformancePaused = copy.PerformancePaused || paused[id]
			out[id] = &copy
		}
	}
	return out, nil
}
