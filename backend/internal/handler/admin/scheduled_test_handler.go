package admin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ScheduledTestHandler handles admin scheduled-test-plan management.
type ScheduledTestHandler struct {
	scheduledTestSvc *service.ScheduledTestService
}

type testDefinitionRequest struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Prompt      string  `json:"prompt"`
	OutputKind  string  `json:"output_kind"`
	// Kind is accepted as a frontend-friendly alias for output_kind.
	Kind            string `json:"kind"`
	Enabled         *bool  `json:"enabled"`
	SortOrder       *int   `json:"sort_order"`
	MaxOutputTokens *int   `json:"max_output_tokens"`
}

func (h *ScheduledTestHandler) ListDefinitions(c *gin.Context) {
	// Admins must be able to edit/re-enable disabled definitions. Filtering is
	// opt-in for API clients via enabled_only=true.
	defs, err := h.scheduledTestSvc.ListDefinitions(c.Request.Context(), c.Query("enabled_only") == "true")
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, defs)
}
func (h *ScheduledTestHandler) CreateDefinition(c *gin.Context) {
	var req testDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	outputKind := req.OutputKind
	if outputKind == "" {
		outputKind = req.Kind
	}
	description := ""
	if req.Description != nil {
		description = *req.Description
	}
	d := &service.ScheduledTestDefinition{Key: req.Key, Name: req.Name, Description: description, Prompt: req.Prompt, OutputKind: outputKind, Enabled: true}
	if req.MaxOutputTokens != nil {
		d.MaxOutputTokens = *req.MaxOutputTokens
	}
	if req.SortOrder != nil {
		if *req.SortOrder < 0 {
			response.BadRequest(c, "sort_order must be non-negative")
			return
		}
		d.SortOrder = *req.SortOrder
	}
	if req.Enabled != nil {
		d.Enabled = *req.Enabled
	}
	out, err := h.scheduledTestSvc.CreateDefinition(c.Request.Context(), d)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, out)
}
func (h *ScheduledTestHandler) UpdateDefinition(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.BadRequest(c, "invalid definition id")
		return
	}
	d, e := h.scheduledTestSvc.GetDefinition(c.Request.Context(), id)
	if e != nil {
		response.NotFound(c, "definition not found")
		return
	}
	var req testDefinitionRequest
	if e = c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	if req.Key != "" {
		d.Key = req.Key
	}
	if req.Name != "" {
		d.Name = req.Name
	}
	// An explicit empty description clears the value; omitted fields preserve
	// the existing value for partial updates.
	if req.Description != nil {
		d.Description = *req.Description
	}
	if req.Prompt != "" {
		d.Prompt = req.Prompt
	}
	if req.MaxOutputTokens != nil {
		d.MaxOutputTokens = *req.MaxOutputTokens
	}
	if req.OutputKind != "" {
		d.OutputKind = req.OutputKind
	} else if req.Kind != "" {
		d.OutputKind = req.Kind
	}
	if req.Enabled != nil {
		d.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		if *req.SortOrder < 0 {
			response.BadRequest(c, "sort_order must be non-negative")
			return
		}
		d.SortOrder = *req.SortOrder
	}
	out, e := h.scheduledTestSvc.UpdateDefinition(c.Request.Context(), d)
	if e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	c.JSON(http.StatusOK, out)
}
func (h *ScheduledTestHandler) DeleteDefinition(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.BadRequest(c, "invalid definition id")
		return
	}
	if e = h.scheduledTestSvc.DeleteDefinition(c.Request.Context(), id); e != nil {
		if errors.Is(e, service.ErrScheduledTestDefinitionInUse) {
			response.Error(c, http.StatusConflict, e.Error())
			return
		}
		response.InternalError(c, e.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// NewScheduledTestHandler creates a new ScheduledTestHandler.
func NewScheduledTestHandler(scheduledTestSvc *service.ScheduledTestService) *ScheduledTestHandler {
	return &ScheduledTestHandler{scheduledTestSvc: scheduledTestSvc}
}

type createScheduledTestPlanRequest struct {
	AccountID          *int64                                `json:"account_id"`
	PelicanConfig      *service.PelicanTestConfig            `json:"pelican_config"`
	Name               string                                `json:"name"`
	SortOrder          *int                                  `json:"sort_order"`
	GroupIDs           []int64                               `json:"group_ids"`
	ExcludedAccountIDs []int64                               `json:"excluded_account_ids"`
	TestDefinitionIDs  []int64                               `json:"test_definition_ids"`
	ModelID            string                                `json:"model_id"`
	ModelIDs           []string                              `json:"model_ids"`
	ReasoningEffort    string                                `json:"reasoning_effort"`
	CronExpression     string                                `json:"cron_expression"`
	Schedule           *service.ScheduledTestSchedule        `json:"schedule"`
	Enabled            *bool                                 `json:"enabled"`
	MaxResults         int                                   `json:"max_results"`
	AutoRecover        *bool                                 `json:"auto_recover"`
	Protection         service.ScheduledTestProtectionConfig `json:"protection"`
}

type updateScheduledTestPlanRequest struct {
	PelicanConfig        *service.PelicanTestConfig             `json:"pelican_config"`
	PreserveQualityState bool                                   `json:"preserve_quality_state"`
	Name                 *string                                `json:"name"`
	SortOrder            *int                                   `json:"sort_order"`
	GroupIDs             json.RawMessage                        `json:"group_ids"`
	ExcludedAccountIDs   json.RawMessage                        `json:"excluded_account_ids"`
	TestDefinitionIDs    json.RawMessage                        `json:"test_definition_ids"`
	ModelID              string                                 `json:"model_id"`
	ModelIDs             json.RawMessage                        `json:"model_ids"`
	ReasoningEffort      json.RawMessage                        `json:"reasoning_effort"`
	CronExpression       string                                 `json:"cron_expression"`
	Schedule             json.RawMessage                        `json:"schedule"`
	Enabled              *bool                                  `json:"enabled"`
	MaxResults           int                                    `json:"max_results"`
	AutoRecover          *bool                                  `json:"auto_recover"`
	Protection           *service.ScheduledTestProtectionConfig `json:"protection"`
}

// ListByAccount GET /admin/accounts/:id/scheduled-test-plans
func (h *ScheduledTestHandler) ListByAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	plans, err := h.scheduledTestSvc.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if _, observer := service.ObserverGroupIDs(c.Request.Context()); observer {
		ownPlans := make([]*service.ScheduledTestPlan, 0, len(plans))
		for _, plan := range plans {
			if plan.TargetAccountID() == accountID {
				ownPlans = append(ownPlans, plan)
			}
		}
		plans = ownPlans
	}
	c.JSON(http.StatusOK, plans)
}

// AccountProtectionDetails GET /admin/accounts/:id/quality-protection
func (h *ScheduledTestHandler) AccountProtectionDetails(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	details, err := h.scheduledTestSvc.AccountProtectionDetails(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, details)
}

func (h *ScheduledTestHandler) ListPlans(c *gin.Context) {
	plans, err := h.scheduledTestSvc.ListPlans(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

// Create POST /admin/scheduled-test-plans
func (h *ScheduledTestHandler) Create(c *gin.Context) {
	var req createScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if _, observer := service.ObserverGroupIDs(c.Request.Context()); observer && (req.AccountID == nil || *req.AccountID <= 0) {
		response.ErrorFrom(c, service.ErrObserverScope)
		return
	}
	if !qualityJudgeAccess(c, req.PelicanConfig) {
		return
	}
	plan := &service.ScheduledTestPlan{
		AccountID:          req.AccountID,
		PelicanConfig:      req.PelicanConfig,
		Name:               req.Name,
		GroupIDs:           req.GroupIDs,
		ExcludedAccountIDs: req.ExcludedAccountIDs,
		TestDefinitionIDs:  req.TestDefinitionIDs,
		ModelID:            req.ModelID,
		ModelIDs:           req.ModelIDs,
		ReasoningEffort:    req.ReasoningEffort,
		CronExpression:     req.CronExpression,
		Schedule:           req.Schedule,
		Enabled:            true,
		MaxResults:         req.MaxResults,
		Protection:         req.Protection,
	}
	if req.SortOrder != nil {
		if *req.SortOrder < 0 {
			response.BadRequest(c, "sort_order must be non-negative")
			return
		}
		plan.SortOrder = *req.SortOrder
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}
	if req.AutoRecover != nil {
		plan.AutoRecover = *req.AutoRecover
	}

	created, err := h.scheduledTestSvc.CreatePlan(c.Request.Context(), plan)
	if err != nil {
		if infraerrors.IsForbidden(err) {
			response.ErrorFrom(c, err)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

// Update PUT /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Update(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	existing, err := h.scheduledTestSvc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}

	var req updateScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if existing.TargetAccountID() > 0 {
		h.updateAccountPlan(c, existing, req)
		return
	}
	if req.PelicanConfig != nil || req.PreserveQualityState {
		response.BadRequest(c, "account test settings require an account plan")
		return
	}
	if existing.MigrationNote != "" && (len(req.GroupIDs) == 0 || len(req.TestDefinitionIDs) == 0) {
		response.BadRequest(c, "migrated strategy requires reviewing and saving group_ids and test_definition_ids before activation")
		return
	}
	if len(req.ModelIDs) > 0 {
		var models []string
		if err := json.Unmarshal(req.ModelIDs, &models); err != nil || models == nil || len(models) == 0 {
			response.BadRequest(c, "model_ids must be a nonempty array")
			return
		}
		existing.ModelIDs = models
	} else if req.ModelID != "" {
		// Older clients explicitly replacing the scalar select one model.
		existing.ModelID = req.ModelID
		existing.ModelIDs = nil
	}
	if len(req.ReasoningEffort) > 0 {
		if bytes.Equal(bytes.TrimSpace(req.ReasoningEffort), []byte("null")) {
			existing.ReasoningEffort = ""
		} else {
			var effort string
			if err := json.Unmarshal(req.ReasoningEffort, &effort); err != nil {
				response.BadRequest(c, "reasoning_effort must be a string or null")
				return
			}
			existing.ReasoningEffort = effort
		}
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.SortOrder != nil {
		if *req.SortOrder < 0 {
			response.BadRequest(c, "sort_order must be non-negative")
			return
		}
		existing.SortOrder = *req.SortOrder
	}
	if len(req.GroupIDs) > 0 {
		var ids []int64
		if err := json.Unmarshal(req.GroupIDs, &ids); err != nil || ids == nil {
			response.BadRequest(c, "group_ids must be an array")
			return
		}
		existing.GroupIDs = ids
	}
	if len(req.ExcludedAccountIDs) > 0 {
		var ids []int64
		if err := json.Unmarshal(req.ExcludedAccountIDs, &ids); err != nil || ids == nil {
			response.BadRequest(c, "excluded_account_ids must be an array")
			return
		}
		existing.ExcludedAccountIDs = ids
	}
	if len(req.TestDefinitionIDs) > 0 {
		var ids []int64
		if err := json.Unmarshal(req.TestDefinitionIDs, &ids); err != nil || ids == nil {
			response.BadRequest(c, "test_definition_ids must be an array")
			return
		}
		existing.TestDefinitionIDs = ids
		existing.TestDefinitionID = nil
	}

	if req.CronExpression != "" {
		existing.CronExpression = req.CronExpression
	}
	// Omitted schedules survive partial updates. Explicit null switches back
	// to cron, whose value must still pass the service's validation.
	if len(req.Schedule) > 0 {
		var schedule *service.ScheduledTestSchedule
		if err := json.Unmarshal(req.Schedule, &schedule); err != nil {
			response.BadRequest(c, "invalid schedule")
			return
		}
		existing.Schedule = schedule
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.MaxResults > 0 {
		existing.MaxResults = req.MaxResults
	}
	if req.AutoRecover != nil {
		existing.AutoRecover = *req.AutoRecover
	}
	if req.Protection != nil {
		existing.Protection = *req.Protection
	}

	updated, err := h.scheduledTestSvc.UpdatePlan(c.Request.Context(), existing)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *ScheduledTestHandler) updateAccountPlan(c *gin.Context, existing *service.ScheduledTestPlan, req updateScheduledTestPlanRequest) {
	if len(req.GroupIDs) != 0 || len(req.TestDefinitionIDs) != 0 || len(req.Schedule) != 0 || req.Protection != nil {
		response.BadRequest(c, "group strategy settings require a group plan")
		return
	}
	if !qualityJudgeAccess(c, existing.PelicanConfig, req.PelicanConfig) {
		return
	}
	existing.PreserveQualityState = req.PreserveQualityState

	if req.PelicanConfig != nil {
		if existing.PelicanConfig == nil {
			response.BadRequest(c, "cannot change test type")
			return
		}
		if (existing.PelicanConfig.Quality == nil) != (req.PelicanConfig.Quality == nil) {
			response.BadRequest(c, "cannot change quality test type")
			return
		}
		existing.PelicanConfig = req.PelicanConfig
	}
	if req.ModelID != "" {
		if req.PelicanConfig == nil && existing.PelicanConfig != nil {
			for i, model := range existing.PelicanConfig.ModelIDs {
				if model == existing.ModelID {
					existing.PelicanConfig.ModelIDs[i] = req.ModelID
				}
			}
		}
		existing.ModelID = req.ModelID
	}
	if req.CronExpression != "" {
		existing.CronExpression = req.CronExpression
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.MaxResults > 0 {
		existing.MaxResults = req.MaxResults
	}
	if req.AutoRecover != nil {
		existing.AutoRecover = *req.AutoRecover
	}

	updated, err := h.scheduledTestSvc.UpdatePlan(c.Request.Context(), existing)
	if err != nil {
		if infraerrors.Reason(err) == "OBSERVER_SCOPE_FORBIDDEN" || infraerrors.Reason(err) == service.QualityScopeConflictReason || infraerrors.Reason(err) == "QUALITY_MODEL_NOT_ALLOWED" || infraerrors.Reason(err) == "QUALITY_MODEL_CATALOG_UNKNOWN" || infraerrors.Reason(err) == "OPENAI_CODEX_MODELS_CREDENTIALS_INVALID" {
			response.ErrorFrom(c, err)
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

// Delete DELETE /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Delete(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	if c.Request.ContentLength != 0 {
		var req service.QualityOwnershipRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "invalid ownership deletion confirmation")
			return
		}
		// Older clients send an empty JSON object on DELETE. Preserve their
		// guarded path; only an explicit ownership confirmation selects fencing.
		if req.Fence != "" || req.Confirm {
			req.Delete = true
			if _, err := h.scheduledTestSvc.QualityOwnership(c.Request.Context(), planID, &req); err != nil {
				response.ErrorFrom(c, err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "deleted"})
			return
		}
	}

	if err := h.scheduledTestSvc.DeletePlan(c.Request.Context(), planID); err != nil {
		if infraerrors.Reason(err) == "OBSERVER_SCOPE_FORBIDDEN" || infraerrors.Reason(err) == service.QualityScopeConflictReason {
			response.ErrorFrom(c, err)
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListResults GET /admin/scheduled-test-plans/:id/results
func (h *ScheduledTestHandler) ListResults(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	plan, err := h.scheduledTestSvc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}
	var includeContent []bool
	if plan.TargetAccountID() > 0 {
		limit = min(limit, 100)
		includeContent = []bool{c.Query("include_content") != "false"}
	}
	results, err := h.scheduledTestSvc.ListResults(c.Request.Context(), planID, limit, includeContent...)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}

// DeleteResult DELETE /admin/test-results/:id
func (h *ScheduledTestHandler) DeleteResult(c *gin.Context) {
	resultID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || resultID <= 0 {
		response.BadRequest(c, "invalid test result id")
		return
	}
	if err := h.scheduledTestSvc.DeleteResult(c.Request.Context(), resultID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "test result not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *ScheduledTestHandler) RunNow(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if e = h.scheduledTestSvc.RunNow(c.Request.Context(), id); e != nil {
		response.BadRequest(c, e.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "test queued"})
}

// RetryResult POST /admin/test-results/:id/retry
func (h *ScheduledTestHandler) RetryResult(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid test result id")
		return
	}
	result, err := h.scheduledTestSvc.RetryResult(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			response.NotFound(c, "test result or plan not found")
		case errors.Is(err, service.ErrScheduledTestAccountRunning), errors.Is(err, service.ErrScheduledTestResultNotFailed):
			response.Error(c, http.StatusConflict, err.Error())
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}
	c.JSON(http.StatusAccepted, result)
}

// Observers may manage scoped connectivity tests, but cannot opt accounts into
// shared audit credentials or enable a saved Jev rule through that older API.
func qualityJudgeAccess(c *gin.Context, configs ...*service.PelicanTestConfig) bool {
	if _, observer := service.ObserverGroupIDs(c.Request.Context()); observer {
		for _, cfg := range configs {
			if cfg != nil && cfg.Quality != nil && cfg.Quality.Judge != nil && cfg.Quality.Judge.Kind == "jev" {
				response.ErrorFrom(c, service.ErrObserverScope)
				return false
			}
		}
	}
	return true
}

func (h *ScheduledTestHandler) GetResult(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	resultID, err := strconv.ParseInt(c.Param("resultID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid result id")
		return
	}
	result, err := h.scheduledTestSvc.GetResult(c.Request.Context(), planID, resultID)
	if err != nil {
		response.NotFound(c, "result not found or expired")
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListPelicanHistory returns account-independent summaries for the record dashboard.
// Optional account_id narrows them to one account for the IQ test's records window.
func (h *ScheduledTestHandler) ListPelicanHistory(c *gin.Context) {
	beforeID, err := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	if err != nil || beforeID < 0 {
		response.BadRequest(c, "invalid before_id")
		return
	}
	accountID, err := strconv.ParseInt(c.DefaultQuery("account_id", "0"), 10, 64)
	if err != nil || accountID < 0 {
		response.BadRequest(c, "invalid account_id")
		return
	}
	page, err := h.scheduledTestSvc.ListPelicanHistory(c.Request.Context(), beforeID, 100, accountID)
	if err != nil {
		response.InternalError(c, "Failed to load pelican history")
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *ScheduledTestHandler) ListQualityPlans(c *gin.Context) {
	plans, err := h.scheduledTestSvc.ListQualityPlans(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load quality plans")
		return
	}
	if plans == nil {
		plans = []*service.ScheduledTestPlan{}
	}
	c.JSON(http.StatusOK, plans)
}
func (h *ScheduledTestHandler) TriggerQuality(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if err = h.scheduledTestSvc.TriggerQuality(c.Request.Context(), id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "queued"})
}

func (h *ScheduledTestHandler) ListQualityHistory(c *gin.Context) {
	beforeID, err := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	if err != nil || beforeID < 0 {
		response.BadRequest(c, "invalid before_id")
		return
	}
	page, err := h.scheduledTestSvc.ListQualityHistory(c.Request.Context(), beforeID)
	if err != nil {
		response.InternalError(c, "Failed to load quality operation history")
		return
	}
	c.JSON(http.StatusOK, page)
}

// ObserverGuard resolves plan IDs to accounts before any plan/result operation.
func (h *ScheduledTestHandler) ObserverGuard(accounts *AccountHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if _, scoped := service.ObserverGroupIDs(ctx); !scoped {
			c.Next()
			return
		}
		if c.Param("id") == "" {
			// The common account middleware checked the body account ID.
			c.Next()
			return
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "invalid plan id")
			c.Abort()
			return
		}
		plan, err := h.scheduledTestSvc.GetPlan(ctx, id)
		if err != nil || plan == nil || plan.TargetAccountID() <= 0 {
			response.ErrorFrom(c, service.ErrObserverScope)
			c.Abort()
			return
		}
		account, err := accounts.adminService.GetAccount(ctx, plan.TargetAccountID())
		if err != nil || !service.ObserverCanManageAccount(ctx, account) {
			response.ErrorFrom(c, service.ErrObserverScope)
			c.Abort()
			return
		}
		c.Next()
	}
}
