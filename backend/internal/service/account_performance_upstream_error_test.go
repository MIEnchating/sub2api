package service

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/upstreamstats"
	"github.com/stretchr/testify/require"
)

func TestAccountPerformanceUpstreamErrorValidationAndLegacyConfig(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		name   string
		bounds PerformanceBounds
		valid  bool
	}{
		{"unset", PerformanceBounds{}, true},
		{"full_range", PerformanceBounds{Min: ptr(0), Max: ptr(100)}, true},
		{"equal", PerformanceBounds{Min: ptr(25), Max: ptr(25)}, true},
		{"negative", PerformanceBounds{Min: ptr(-1)}, false},
		{"above_100", PerformanceBounds{Max: ptr(100.01)}, false},
		{"inverted", PerformanceBounds{Min: ptr(25), Max: ptr(20)}, false},
		{"nan", PerformanceBounds{Max: ptr(math.NaN())}, false},
		{"positive_infinity", PerformanceBounds{Min: ptr(math.Inf(1))}, false},
		{"negative_infinity", PerformanceBounds{Min: ptr(math.Inf(-1))}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultPerformanceConfig()
			cfg.Enabled = true
			cfg.TPS.Min = ptr(10)
			cfg.UpstreamError = tc.bounds
			if tc.valid {
				require.NoError(t, cfg.Validate())
			} else {
				require.Error(t, cfg.Validate())
			}
		})
	}
	legacy := DefaultPerformanceConfig()
	require.NoError(t, json.Unmarshal([]byte(`{"version":2,"enabled":true,"tps":{"min":10}}`), &legacy))
	require.Nil(t, legacy.UpstreamError.Min)
	require.Nil(t, legacy.UpstreamError.Max)
	require.NoError(t, legacy.Validate())
	p := 100.0
	m := AccountPerformance{Config: legacy, UpstreamError: upstreamstats.Evidence{Total: 1, Failures: 1, Percent: &p}}
	m.UpstreamError.Source = "process_local_http_responses"
	require.Empty(t, m.Breaches(), "configuration without an upstream bound must not implicitly enable an error-rate threshold")
}

func TestAccountPerformanceUpstreamErrorBreaches(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		name     string
		bounds   PerformanceBounds
		evidence upstreamstats.Evidence
		breach   bool
	}{
		{"missing_samples", PerformanceBounds{Min: ptr(10)}, upstreamstats.Evidence{}, false},
		{"percent_without_samples", PerformanceBounds{Max: ptr(20)}, upstreamstats.Evidence{Percent: ptr(100)}, false},
		{"samples_without_percent", PerformanceBounds{Min: ptr(10)}, upstreamstats.Evidence{Total: 1}, false},
		{"observed_zero_below_min", PerformanceBounds{Min: ptr(10)}, upstreamstats.Evidence{Total: 3, Percent: ptr(0)}, true},
		{"zero_equals_min", PerformanceBounds{Min: ptr(0)}, upstreamstats.Evidence{Total: 3, Percent: ptr(0)}, false},
		{"at_max", PerformanceBounds{Max: ptr(25)}, upstreamstats.Evidence{Total: 4, Failures: 1, Percent: ptr(25)}, false},
		{"above_max", PerformanceBounds{Max: ptr(20)}, upstreamstats.Evidence{Total: 4, Failures: 1, Percent: ptr(25)}, true},
		{"partial_window", PerformanceBounds{Max: ptr(20)}, upstreamstats.Evidence{Total: 1, Failures: 1, Percent: ptr(100), WindowComplete: false}, true},
		{"negative", PerformanceBounds{Min: ptr(10)}, upstreamstats.Evidence{Total: 1, Percent: ptr(-1)}, false},
		{"above_100", PerformanceBounds{Max: ptr(20)}, upstreamstats.Evidence{Total: 1, Percent: ptr(101)}, false},
		{"nan", PerformanceBounds{Max: ptr(20)}, upstreamstats.Evidence{Total: 1, Percent: ptr(math.NaN())}, false},
		{"infinity", PerformanceBounds{Max: ptr(20)}, upstreamstats.Evidence{Total: 1, Percent: ptr(math.Inf(1))}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.evidence.Source = "process_local_http_responses"
			m := AccountPerformance{Config: DefaultPerformanceConfig(), UpstreamError: tc.evidence}
			m.Config.UpstreamError = tc.bounds
			if tc.breach {
				require.Equal(t, []string{"upstream_error"}, m.Breaches())
			} else {
				require.Empty(t, m.Breaches())
			}
		})
	}
}

func TestAccountPerformanceManualReleaseReasonsAreIndependentOfAutoRecovery(t *testing.T) {
	cfg := DefaultPerformanceConfig()
	minSuccess, maxObservedTTFT := 95.0, 100.0
	cfg.Enabled = true
	cfg.SuccessRate.Pause.Min = &minSuccess
	cfg.ObservedTTFT.Max = &maxObservedTTFT
	require.NoError(t, cfg.Validate())

	successRate, observedTTFT := 0.90, 120.0
	m := AccountPerformance{Config: cfg, AvgTTFTMs: &observedTTFT, TTFTSamples: 1}
	m.Metrics.SuccessRate = &successRate
	m.Metrics.RequestSamples = cfg.MinSamples

	require.Equal(t, []string{"success_rate", "observed_ttft"}, m.Breaches())
	require.True(t, PerformanceRequiresManualRelease("success_rate,observed_ttft"))
	require.False(t, PerformanceRequiresManualRelease("success_rate,ttft"))
}
