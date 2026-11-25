package service

import (
	"context"
	"time"
)

// PelicanTestConfig stores intelligence test inputs; a missing kind preserves legacy HTML plans.
type PelicanTestConfig struct {
	Ciallo *CialloEvidence `json:"ciallo,omitempty"`
	// Optional display name stored in the existing configuration JSON.
	RuleName string `json:"rule_name,omitempty"`
	// Filled only on saved result snapshots, never used to configure a rule.
	QualityRestoreDiagnostic *QualityRestoreDiagnostic `json:"quality_restore_diagnostic,omitempty"`
	QualityModelOutcomes     map[string]string         `json:"quality_model_outcomes,omitempty"`
	QualityModelActions      map[string]string         `json:"quality_model_actions,omitempty"`
	TriggerSource            string                    `json:"trigger_source,omitempty"`
	RandomQuestion           *QualityRandomQuestion    `json:"random_question,omitempty"`
	QualityBatch             *QualityBatchEvidence     `json:"quality_batch,omitempty"`
	// BPSRecoveryPending is filled from persisted ownership when the runner
	// claims a plan. It is never accepted from or written to configuration JSON.
	BPSRecoveryPending bool           `json:"-"`
	Quality            *QualityPolicy `json:"quality,omitempty"`
	QuestionKind       string         `json:"question_kind,omitempty"`
	// Empty mode preserves the exact prompt of legacy/custom plans.
	PromptMode      string           `json:"prompt_mode,omitempty"`
	PelicanQuestion *PelicanQuestion `json:"pelican_question,omitempty"`
	ActualPrompt    string           `json:"actual_prompt,omitempty"`
	TestChannel     string           `json:"test_channel,omitempty"`
	Prompt          string           `json:"prompt"`
	ReasoningEffort string           `json:"reasoning_effort"`
	ParallelCount   int              `json:"parallel_count"`
	// ModelID is recorded with each result so later edits do not relabel history.
	ModelID string `json:"model_id,omitempty"`
	// ModelIDs selects all models in a quality round; old plans use ModelID.
	ModelIDs []string `json:"model_ids,omitempty"`
}

// ScheduledTestPlan represents a scheduled test plan domain model.
type ScheduledTestPlan struct {
	degradationRetest bool
	// Populated only from the repository's one-shot manual trigger marker.
	ManualQualityRun      bool              `json:"-"`
	ManualQualityRunID    string            `json:"-"`
	QualityBatchEvaluated bool              `json:"-"`
	QualityBatchModels    map[string]string `json:"-"`
	// Request-only: importing configuration preserves this target's ownership.
	PreserveQualityState     bool                      `json:"-"`
	QualityRestoreDiagnostic *QualityRestoreDiagnostic `json:"-"`
	QualityModelActions      map[string]string         `json:"-"`
	QualityModelOutcomes     map[string]string         `json:"-"`
	Quality5xxEpisode        int64                     `json:"-"`
	TriggerSource            string                    `json:"-"`
	TriggerObservedAt        *time.Time                `json:"-"`
	AccountName              string                    `json:"account_name,omitempty"`
	PelicanConfig            *PelicanTestConfig        `json:"pelican_config,omitempty"`
	RunningUntil             *time.Time                `json:"running_until,omitempty"`
	ExecutionLeaseOwner      string                    `json:"-"`
	ExecutionLeaseScope      string                    `json:"-"`
	ID                       int64                     `json:"id"`
	Name                     string                    `json:"name"`
	// SortOrder orders strategies; GroupIDs orders groups within a strategy.
	// A round snapshots the union of these groups once, deduplicated by account.
	GroupIDs           []int64                       `json:"group_ids"`
	ExcludedAccountIDs []int64                       `json:"excluded_account_ids"`
	MigrationNote      string                        `json:"migration_note,omitempty"`
	SortOrder          int                           `json:"sort_order"`
	AccountID          *int64                        `json:"account_id,omitempty"`
	GroupID            *int64                        `json:"group_id,omitempty"`
	TestDefinitionID   *int64                        `json:"test_definition_id,omitempty"`
	TestDefinitionIDs  []int64                       `json:"test_definition_ids"`
	TestType           string                        `json:"test_type"`
	TargetMode         string                        `json:"target_mode"`
	ModelID            string                        `json:"model_id"`
	ModelIDs           []string                      `json:"model_ids"`
	ReasoningEffort    string                        `json:"reasoning_effort,omitempty"`
	CronExpression     string                        `json:"cron_expression"`
	Schedule           *ScheduledTestSchedule        `json:"schedule,omitempty"`
	Enabled            bool                          `json:"enabled"`
	MaxResults         int                           `json:"max_results"`
	AutoRecover        bool                          `json:"auto_recover"`
	Protection         ScheduledTestProtectionConfig `json:"protection"`
	LastRunAt          *time.Time                    `json:"last_run_at"`
	NextRunAt          *time.Time                    `json:"next_run_at"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

// TargetAccountID is zero for plans that address groups or have no account.
func (p *ScheduledTestPlan) TargetAccountID() int64 {
	if p == nil || p.AccountID == nil {
		return 0
	}
	return *p.AccountID
}

// ScheduledTestResult represents a single test execution result.
type ScheduledTestResult struct {
	// Attempt-local Grok recovery evidence; never accepted from or stored in JSON.
	grokRecovery *grokTestRecovery

	// Runtime-only; set solely from an actual own-credential OAuth HTTP 401.
	DirectOAuth401        bool `json:"-"`
	DirectOAuthRevoked401 bool `json:"-"`

	QualityRoundID      string                        `json:"quality_round_id,omitempty"`
	QualityJudgment     *QualityJudgment              `json:"quality_judgment,omitempty"`
	QualityAction       string                        `json:"quality_action,omitempty"`
	PelicanConfig       *PelicanTestConfig            `json:"pelican_config,omitempty"`
	ExecutionLeaseOwner string                        `json:"-"`
	ExecutionLeaseScope string                        `json:"-"`
	AllPassProgress     *ScheduledTestAllPassProgress `json:"all_pass_progress,omitempty"`
	PassRate            *ScheduledTestPassRateSummary `json:"pass_rate,omitempty"`
	ExecutionInfo       *ScheduledTestExecutionInfo   `json:"execution_info,omitempty"`
	RunID               string                        `json:"-"`
	ID                  int64                         `json:"id"`
	PlanID              int64                         `json:"plan_id"`
	TestDefinitionID    *int64                        `json:"test_definition_id,omitempty"`
	PlanName            string                        `json:"plan_name"`
	TestName            string                        `json:"test_name"`
	TestKey             string                        `json:"test_key,omitempty"`
	// QualityVerdict is the saved check verdict, without private action details.
	QualityVerdict string `json:"quality_verdict,omitempty"`
	TestOrder      int    `json:"test_order"`
	GroupName      string `json:"group_name"`
	// ModelOrder is the zero-based position in the current plan's configured
	// model selection. Models no longer selected sort after configured models.
	ModelOrder int `json:"model_order"`
	// PlanOrder belongs to the rule that produced the result. GroupOrder is
	// resolved from the displayed group's current rules, so moving an account
	// does not carry its source rule's display position into the new group.
	PlanOrder            int                              `json:"plan_order"`
	GroupOrder           *int                             `json:"group_order,omitempty"`
	TargetMode           string                           `json:"target_mode,omitempty"`
	Status               string                           `json:"status"`
	ResponseText         string                           `json:"response_text"`
	OutputKind           string                           `json:"output_kind"`
	OutputHTML           string                           `json:"output_html,omitempty"`
	OutputNumeric        *float64                         `json:"output_numeric,omitempty"`
	OutputModelCheck     *ScheduledTestModelCheck         `json:"output_model_check,omitempty"`
	ProtectionDecision   *ScheduledTestProtectionDecision `json:"protection_decision,omitempty"`
	UpstreamModel        string                           `json:"-"`
	ReturnedModels       []string                         `json:"-"`
	ModelEvidenceInvalid bool                             `json:"-"`
	AccountID            *int64                           `json:"account_id,omitempty"`
	ModelSchedulable     *bool                            `json:"model_schedulable,omitempty"`
	AccountSchedulable   *bool                            `json:"account_schedulable,omitempty"` // current account status and scheduling switch; nil for account-free results
	AccountName          string                           `json:"account_name,omitempty"`        // populated only for authenticated administrator responses
	ModelID              string                           `json:"model_id"`
	ReasoningEffort      string                           `json:"reasoning_effort,omitempty"`
	GroupID              *int64                           `json:"group_id,omitempty"`
	ErrorMessage         string                           `json:"error_message"`
	LatencyMs            int64                            `json:"latency_ms"`
	StartedAt            time.Time                        `json:"started_at"`
	FinishedAt           time.Time                        `json:"finished_at"`
	CreatedAt            time.Time                        `json:"created_at"`
	// ExecutionSkipped is a safe public marker; skip diagnostics remain private.
	ExecutionSkipped bool `json:"execution_skipped,omitempty"`
}

// ScheduledTestProtectionDecision is populated for administrator result views.
// It records the verdict and actual account changes in the same transaction;
// public result endpoints deliberately do not expose it.
type ScheduledTestProtectionDecision struct {
	Status          string  `json:"status,omitempty"`
	Verdict         string  `json:"verdict"`
	Reason          string  `json:"reason,omitempty"`
	Scheduling      string  `json:"scheduling,omitempty"`
	GroupIDs        []int64 `json:"group_ids,omitempty"`
	AddedGroupIDs   []int64 `json:"added_group_ids,omitempty"`
	RemovedGroupIDs []int64 `json:"removed_group_ids,omitempty"`
}

// BeginRun atomically publishes the complete account/type snapshot before any
// upstream work begins, including records waiting for a worker.
type ScheduledTestRunRepository interface {
	BeginRun(context.Context, *ScheduledTestPlan, string, []*ScheduledTestResult) ([]*ScheduledTestResult, error)
}

type ScheduledTestDecisionRepository interface {
	RecordDecision(context.Context, *ScheduledTestResult, ScheduledTestProtectionDecision) error
}

type ScheduledTestResultHistory struct {
	Items        []*ScheduledTestResult `json:"items"`
	NextBeforeID *int64                 `json:"next_before_id,omitempty"`
}

type ScheduledTestDefinition struct {
	ID              int64     `json:"id"`
	Key             string    `json:"key"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Prompt          string    `json:"prompt"`
	OutputKind      string    `json:"output_kind"`
	MaxOutputTokens int       `json:"max_output_tokens"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ScheduledTestPlanRepository defines the data access interface for test plans.
type ScheduledTestPlanRepository interface {
	FinishTriggeredQuality(context.Context, *ScheduledTestPlan, time.Time, time.Time, time.Time) error
	ListQualityPlans(context.Context) ([]*ScheduledTestPlan, error)
	ApplyQualityOutcome(context.Context, *ScheduledTestPlan, time.Time, string) (string, error)
	TriggerQuality(context.Context, int64) error
	ClaimPelican(ctx context.Context, plan *ScheduledTestPlan, now, until, next time.Time) (bool, error)
	FinishPelican(ctx context.Context, id int64, until, finished time.Time) error
	Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error)
	List(ctx context.Context) ([]*ScheduledTestPlan, error)
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
	Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, plan *ScheduledTestPlan, lastRunAt time.Time, nextRunAt time.Time) error
	UpdateNextRun(ctx context.Context, plan *ScheduledTestPlan, nextRunAt time.Time) error
}

type ScheduledTestDefinitionRepository interface {
	Create(context.Context, *ScheduledTestDefinition) (*ScheduledTestDefinition, error)
	GetByID(context.Context, int64) (*ScheduledTestDefinition, error)
	GetByKey(context.Context, string) (*ScheduledTestDefinition, error)
	List(context.Context, bool) ([]*ScheduledTestDefinition, error)
	Update(context.Context, *ScheduledTestDefinition) (*ScheduledTestDefinition, error)
	Delete(context.Context, int64) error
}

// PelicanHistoryResult includes account identity without exposing account credentials.
type PelicanHistoryResult struct {
	ScheduledTestResult
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`
}
type PelicanHistoryPage struct {
	Items      []*PelicanHistoryResult `json:"items"`
	NextCursor int64                   `json:"next_cursor"`
}

// ScheduledTestResultRepository defines the data access interface for test results.
type ScheduledTestResultRepository interface {
	GetByID(ctx context.Context, id int64) (*ScheduledTestResult, error)
	ListQualityHistory(context.Context, int64, int) ([]*QualityHistoryResult, error)
	// accountID 0 lists every account.
	ListPelicanHistory(ctx context.Context, beforeID int64, limit int, accountID int64) ([]*PelicanHistoryResult, error)
	PruneExpiredPelican(ctx context.Context, before time.Time) error
	Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error)
	Update(ctx context.Context, result *ScheduledTestResult) error
	RestartFailed(ctx context.Context, result *ScheduledTestResult) error
	ListVisible(ctx context.Context, userID int64, limit int) ([]*ScheduledTestResult, error)
	ListVisibleHistory(ctx context.Context, userID, resultID, beforeID int64, limit int) ([]*ScheduledTestResult, error)
	Delete(ctx context.Context, id int64) error
	GetResult(ctx context.Context, planID, resultID int64) (*ScheduledTestResult, error)
	ListByPlanID(ctx context.Context, planID int64, limit int, includeContent ...bool) ([]*ScheduledTestResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
}

// ScheduledTestTargetAccountRepository returns all configured target accounts
// for an execution. Eligibility is checked again immediately before upstream
// work, allowing the runner to persist a visible skipped result for an account
// that was manually stopped or otherwise unavailable.
type ScheduledTestTargetAccountRepository interface {
	ListPlanTargetAccountIDs(context.Context, *ScheduledTestPlan, *int64) ([]int64, error)
}

// ScheduledTestRunEligibilityRepository checks a frozen round member without
// consulting its mutable group membership. Manual account stops still apply.
type ScheduledTestRunEligibilityRepository interface {
	IsPlanRunAccountEligible(context.Context, int64, string, int64) (bool, error)
}

type QualityHistoryResult struct {
	PelicanHistoryResult
	PassedCount  int     `json:"passed_count"`
	SkippedCount int     `json:"skipped_count"`
	TotalCount   int     `json:"total_count"`
	ResultIDs    []int64 `json:"result_ids"`
}
type QualityHistoryPage struct {
	Items      []*QualityHistoryResult `json:"items"`
	NextCursor int64                   `json:"next_cursor"`
}
