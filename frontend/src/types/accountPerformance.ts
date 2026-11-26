import type { OpsAccountRequestStats } from '@/api/admin/ops'
export interface PerformanceBounds { min: number | null; max: number | null }
export interface PerformanceRule { pause: PerformanceBounds; recovery: PerformanceBounds }
export const metricKeys = ['success_rate', 'cache_rate', 'ttft', 'duration'] as const
export type PerformanceMetric = typeof metricKeys[number]
export const observedMetricKeys = ['tps', 'degradation', 'observed_ttft', 'upstream_error'] as const
export type ObservedPerformanceMetric = typeof observedMetricKeys[number]

export type PerformancePhase = 'disabled' | 'monitoring' | 'paused' | 'trial'
export interface PerformanceConfig {
 version: 2
 enabled: boolean
 success_rate: PerformanceRule
 cache_rate: PerformanceRule
 ttft: PerformanceRule
 duration: PerformanceRule
 tps: PerformanceBounds
 degradation: PerformanceBounds
 observed_ttft: PerformanceBounds
 upstream_error: PerformanceBounds
 min_samples: number
 auto_recover: boolean
 cooldown_minutes: number
 recovery_samples: number
}
export const DEFAULT_PERFORMANCE_WINDOW_MINUTES = 60

// Display the window used to calculate this response, including data returned
// before a global setting change, rather than guessing from local settings.
export function performanceWindowMinutes(metrics?: Partial<AccountPerformance>): number {
 const minutes = (Date.parse(metrics?.window_end ?? '') - Date.parse(metrics?.window_start ?? '')) / 60_000
 return Number.isInteger(minutes) && minutes > 0 ? minutes : DEFAULT_PERFORMANCE_WINDOW_MINUTES
}

export interface UpstreamHTTPErrorEvidence {
 rate_limited?: number
 percent: number | null
 failures: number
 total: number
 source: 'process_local_http_responses'
 observed_since: string
 window_complete: boolean
}
export interface AccountPerformance {
 statistics_status?: 'ready' | 'stale' | 'pending' | 'unavailable'
 statistics_updated_at?: string
 upstream_error?: UpstreamHTTPErrorEvidence
 avg_cache_percent?: number | null
 cache_samples?: number
 avg_tps: number | null
 degradation_percent: number | null
 avg_ttft_ms: number | null
 tps_samples: number
 ttft_samples: number
 quality_samples: number
 window_start: string
 window_end: string
 phase?: PerformancePhase
 recover_after?: string | null
 trial_ends_at?: string | null
 trial_limit?: number
 monitoring_available: boolean
 account_id: number
 metrics: OpsAccountRequestStats
 config: PerformanceConfig
 revision: number
 paused: boolean
 pause_generation: number
 paused_at: string | null
 trial_started_at: string | null
 trial_requests: number
 metrics_since: string | null
 reason: string
}
export type AccountPerformanceState = Omit<AccountPerformance, 'metrics'>
export function performancePhase(state: Pick<AccountPerformance, 'phase' | 'paused' | 'trial_started_at' | 'config'>): PerformancePhase {
 if (state.phase) return state.phase
 if (state.paused) return state.trial_started_at ? 'trial' : 'paused'
 return state.config.enabled ? 'monitoring' : 'disabled'
}
export interface PerformanceEvent {
 reason?: string
 id: number
 action: 'configure' | 'pause' | 'release' | 'trial' | 'recover' | 'retire'
 actor_id: number | null
 revision: number
 pause_generation: number
 created_at: string
 evidence: unknown
}
const bounds = (): PerformanceBounds => ({ min: null, max: null })
const rule = (): PerformanceRule => ({ pause: bounds(), recovery: bounds() })
export const emptyPerformanceConfig = (): PerformanceConfig => ({
 version: 2, enabled: false, success_rate: rule(), cache_rate: rule(), ttft: rule(), duration: rule(),
 tps: bounds(), degradation: bounds(), observed_ttft: bounds(), upstream_error: bounds(),
 min_samples: 20, auto_recover: true, cooldown_minutes: 5, recovery_samples: 3
})
