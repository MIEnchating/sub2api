package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// Public runtime DTOs intentionally contain no cookie, credential, response ID,
// client session identity or generated content.
type AstraRouteStatus struct {
	ProxyNode        string     `json:"proxy_node,omitempty"`
	ProxyCountry     string     `json:"proxy_country,omitempty"`
	Answer           string     `json:"answer,omitempty"`
	AccountID        int64      `json:"account_id"`
	State            string     `json:"state"`
	Reason           string     `json:"reason"`
	CheckedAt        *time.Time `json:"checked_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	RemainingSeconds int64      `json:"remaining_seconds"`
	Gateway          string     `json:"gateway,omitempty"`
	Active           bool       `json:"active"`
}
type AstraWSStatus struct {
	AccountID        int64      `json:"account_id"`
	Ready            bool       `json:"ready"`
	Reason           string     `json:"reason"`
	ActiveSessions   int        `json:"active_sessions"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	RemainingSeconds int64      `json:"remaining_seconds"`
}
type AstraRotationCooldown struct {
	AccountID        int64     `json:"account_id"`
	Gateway          string    `json:"gateway"`
	RetryAt          time.Time `json:"retry_at"`
	RemainingSeconds int64     `json:"remaining_seconds"`
}
type AstraGatewayRuntime struct {
	SchedulingRecords     []AstraSchedulingRecord   `json:"scheduling_records"`
	Cooldowns             []AstraRotationCooldown   `json:"cooldowns"`
	Gateways              []AstraGatewayObservation `json:"gateways"`
	UnknownGatewaySamples int64                     `json:"unknown_gateway_samples"`
	Setup                 AstraSetupStatus          `json:"setup"`
	GeneratedAt           time.Time                 `json:"generated_at"`
	Revision              string                    `json:"revision"`
	Sources               []AstraRouteStatus        `json:"sources"`
	Targets               []AstraRouteStatus        `json:"targets"`
	WS                    []AstraWSStatus           `json:"ws"`
	// RouteExpiresAt describes transport availability, independent of diagnostic results.
	RouteExpiresAt *time.Time              `json:"route_expires_at,omitempty"`
	ReadyRoutes    int                     `json:"ready_routes"`
	Preparing      bool                    `json:"preparing"`
	LastTest       *AstraGatewayTestResult `json:"last_test,omitempty"`
}
type AstraGatewayObservation struct {
	Gateway          string    `json:"gateway"`
	SourceAccountIDs []int64   `json:"source_account_ids"`
	Samples          int64     `json:"samples"`
	RepeatedHits     int64     `json:"repeated_hits"`
	SourcePasses     int64     `json:"source_passes"`
	SourceFailures   int64     `json:"source_failures"`
	TargetPasses     int64     `json:"target_passes"`
	TargetFailures   int64     `json:"target_failures"`
	LastSeen         time.Time `json:"last_seen"`
}
type AstraGatewayTestResult struct {
	TestKind    string    `json:"test_kind"`
	Answer      string    `json:"answer,omitempty"`
	Expected    string    `json:"expected,omitempty"`
	HTML        string    `json:"html,omitempty"`
	Action      string    `json:"action"`
	AccountID   int64     `json:"account_id"`
	Success     bool      `json:"success"`
	Reason      string    `json:"reason"`
	DurationMS  int64     `json:"duration_ms"`
	CheckedAt   time.Time `json:"checked_at"`
	OutputChars int       `json:"output_chars"`
}
type AstraGatewayRuntimeProvider interface {
	AstraGatewaySnapshot(context.Context) AstraGatewayRuntime
	SetAstraGatewayPreparer(func(context.Context, int64) error)
	PrepareAstraGateway(context.Context) error
}

func (s *AccountTestService) prepareAstraGatewaySource(ctx context.Context, id int64) error {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil || account == nil || !account.IsOpenAIOAuthLike() || account.Status != StatusActive {
		return errors.New("source_account_unavailable")
	}
	question := astraQuestion{Prompt: "Reply with OK only."}
	_, err = s.runAstraQuestionHTTP(WithAstraSourceAcquisition(ctx), id, question)
	return err
}

// Runtime readiness comes from the same persisted account tickets used by forwarding.
func (s *AccountTestService) AstraGatewayStatus(ctx context.Context) AstraGatewayRuntime {
	settings := s.cfg.AstraRouting(ctx)
	result := AstraGatewayRuntime{GeneratedAt: time.Now(), Revision: settings.Revision, Sources: []AstraRouteStatus{}, Targets: []AstraRouteStatus{}, WS: []AstraWSStatus{}, Gateways: []AstraGatewayObservation{}}
	if s.openaiGatewayService != nil {
		result = s.openaiGatewayService.StoredAstraSnapshot(ctx)
	}
	result.SchedulingRecords = astraRecentScheduling.snapshot()
	for i := range result.Targets {
		if !settings.CookiePool.Enabled {
			result.Targets[i].State, result.Targets[i].Reason = "disabled", "disabled"
		}
	}
	for _, id := range settings.WSSession.AccountIDs {
		row := AstraWSStatus{AccountID: id, Reason: "disabled"}
		a, err := s.accountRepo.GetByID(ctx, id)
		if settings.WSSession.Enabled {
			if err != nil || a == nil || !a.IsOpenAIOAuthLike() {
				row.Reason = "account_unavailable"
			} else if d := NewOpenAIWSProtocolResolver(s.cfg).Resolve(a); d.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
				row.Reason = d.Reason
			} else if !a.IsModelSupported("gpt-6-astra") {
				row.Reason = "astra_not_in_allowlist"
			} else {
				row.Ready = true
				row.Reason = "ready"
			}
		}
		if s.openaiGatewayService != nil {
			store := &s.openaiGatewayService.codexWSAnchors
			store.mu.Lock()
			for k, e := range store.entries {
				if k.account == id && time.Now().Before(e.expires) && strings.HasSuffix(k.scope, ":"+settings.Revision) {
					row.ActiveSessions++
					expiry := e.expires
					if row.ExpiresAt == nil || expiry.Before(*row.ExpiresAt) {
						row.ExpiresAt = &expiry
					}
				}
			}
			store.mu.Unlock()
			if row.ExpiresAt != nil {
				row.RemainingSeconds = int64(max(0, int(time.Until(*row.ExpiresAt).Seconds())))
			}
		}
		result.WS = append(result.WS, row)
	}
	s.astraGatewayTestMu.Lock()
	if s.astraGatewayLastTest != nil {
		copy := *s.astraGatewayLastTest
		result.LastTest = &copy
	}
	s.astraGatewayTestMu.Unlock()
	s.astraSetupMu.Lock()
	result.Setup = s.astraSetupStatus
	s.astraSetupMu.Unlock()
	result.Preparing = result.Setup.State == "queued" || result.Setup.State == "running"
	return result
}

// Explicit preparation is bounded and serialized. It uses the donor exit with
// each target's own credentials; only a complete two-shot validation is stored.
func (s *AccountTestService) TestAstraGateway(ctx context.Context, action string, id, adminID int64, _ ...string) (AstraGatewayTestResult, error) {
	result := AstraGatewayTestResult{Action: action, AccountID: id, CheckedAt: time.Now(), TestKind: "state_probe"}
	if !s.astraGatewayActionMu.TryLock() {
		return result, errors.New("test_already_running")
	}
	defer s.astraGatewayActionMu.Unlock()
	if s.openaiGatewayService == nil {
		return result, errors.New("runtime_unavailable")
	}
	settings := s.cfg.AstraRouting(ctx)
	if action != "ws" && !settings.CookiePool.Enabled {
		return result, errors.New("cookie_pool_disabled")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var err error
	switch action {
	case "prepare":
		err = s.openaiGatewayService.PrepareStoredAstraSources(ctx)
		if err == nil {
			for _, target := range settings.CookiePool.TargetAccountIDs {
				if s.cfg.AstraRouting(ctx).Revision != settings.Revision {
					err = errors.New("configuration_changed")
					break
				}
				if err = s.openaiGatewayService.VerifyStoredAstraTarget(ctx, target); err != nil {
					break
				}
			}
		}
		if err == nil {
			if _, ready := astraTargetsReady(ctx, s.openaiGatewayService, settings.CookiePool.TargetAccountIDs); !ready {
				err = errors.New("target_not_verified")
			}
		}
	case "verify":
		if !slices.Contains(settings.CookiePool.TargetAccountIDs, id) {
			return result, errors.New("target_not_configured")
		}
		err = s.openaiGatewayService.VerifyStoredAstraTarget(ctx, id)
	case "ws":
		if !settings.WSSession.Enabled || !slices.Contains(settings.WSSession.AccountIDs, id) {
			return result, errors.New("ws_account_not_configured")
		}
		result.Answer, err = s.testAstraWS(ctx, id, adminID, astraQuestion{Prompt: "Reply with OK only."})
		result.OutputChars = len(result.Answer)
	default:
		return result, errors.New("invalid_action")
	}
	result.Success = err == nil
	result.Reason = "success"
	if err != nil {
		result.Reason = astraSetupError(err)
		if action == "ws" && astraGatewayWSReasonAllowed(err.Error()) {
			result.Reason = err.Error()
		}
	}
	result.DurationMS = time.Since(result.CheckedAt).Milliseconds()
	s.astraGatewayTestMu.Lock()
	s.astraGatewayLastTest = &result
	s.astraGatewayTestMu.Unlock()
	return result, nil
}

func astraGatewayWSReasonAllowed(reason string) bool {
	switch reason {
	case "runtime_unavailable", "account_unavailable", "account_ws_disabled", "astra_not_in_allowlist",
		"ws_empty_result", "ws_transport_not_selected", "ws_response_model_mismatch", "ws_response_id_missing",
		"ws_group_membership_changed", "ws_latest_state_unavailable", "ws_account_ineligible", "ws_admission_denied",
		"ws_test_timeout", "ws_test_cancelled", "ws_upstream_failed", "ws_test_failed":
		return true
	}
	return false
}

func (s *AccountTestService) testAstraWS(ctx context.Context, id, adminID int64, question astraQuestion) (string, error) {
	if s.openaiGatewayService == nil || s.accountRepo == nil || adminID <= 0 {
		return "", errors.New("runtime_unavailable")
	}
	a, err := s.accountRepo.GetByID(ctx, id)
	if err != nil || a == nil {
		return "", errors.New("account_unavailable")
	}
	if NewOpenAIWSProtocolResolver(s.cfg).Resolve(a).Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		return "", errors.New("account_ws_disabled")
	}
	if !a.IsModelSupported("gpt-6-astra") {
		return "", errors.New("astra_not_in_allowlist")
	}
	ctx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	session := "astra-admin-test-" + uuid.NewString()
	defer s.openaiGatewayService.clearAstraAdminTestSession(adminID, session, id)
	previous := ""
	output := ""
	for range 2 {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
		c.Request.Header.Set("session_id", session)
		// Admin tests explicitly select this account; use one of its actual
		// groups so the unchanged primary-database admission check can verify it.
		// This admin probe uses its own two-turn continuation, not a customer key.
		testKey := &APIKey{ID: adminID, AllowLongContext: true}
		if len(a.GroupIDs) > 0 {
			groupID := a.GroupIDs[0]
			testKey.GroupID = &groupID
		}
		c.Set("api_key", testKey)
		SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
		raw, _ := json.Marshal(map[string]any{"model": "gpt-6-astra", "stream": true, "reasoning": map[string]string{"effort": "medium"}, "input": []map[string]string{{"role": "user", "content": question.Prompt}}})
		body := string(raw)
		if previous != "" {
			encoded, _ := json.Marshal(previous)
			body = strings.TrimSuffix(body, "}") + `,"previous_response_id":` + string(encoded) + `}`
		}
		r, err := s.openaiGatewayService.Forward(ctx, c, a, []byte(body))
		if err != nil {
			return "", errors.New(astraWSTestFailureReason(err))
		}
		if r == nil {
			return "", errors.New("ws_empty_result")
		}
		if !r.OpenAIWSMode {
			return "", errors.New("ws_transport_not_selected")
		}
		if r.UpstreamResponseModel != "gpt-6-astra" {
			return "", errors.New("ws_response_model_mismatch")
		}
		if r.ResponseID == "" {
			return "", errors.New("ws_response_id_missing")
		}
		answer := astraWSValidationText(w.Body.String())
		output, err = evaluateAstraAnswer(question, answer)
		if err != nil {
			return output, err
		}
		previous = r.ResponseID
	}
	return output, nil
}

// Admin validation connections are deliberately short lived; regular user
// sessions remain visible in the status panel until their configured deadline.
func (s *OpenAIGatewayService) clearAstraAdminTestSession(adminID int64, session string, accountID int64) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Header.Set("session_id", session)
	scope, _ := resolveOpenAIWSExecutionScope(c, nil, adminID)
	store := &s.codexWSAnchors
	store.mu.Lock()
	defer store.mu.Unlock()
	for key, entry := range store.entries {
		if key.account == accountID && key.apiKey == adminID && strings.HasPrefix(key.scope, scope+":") {
			delete(store.entries, key)
			if entry.connID != "" {
				s.getOpenAIWSConnPool().evictConn(accountID, entry.connID)
			}
		}
	}
}

func (s *AccountTestService) runAstraQuestionHTTP(ctx context.Context, id int64, q astraQuestion) (string, error) {
	a, err := s.accountRepo.GetByID(ctx, id)
	if err != nil || a == nil || !a.IsOpenAIOAuthLike() {
		return "", errors.New("account_unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	w := &pelicanRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/astra-test", nil).WithContext(ctx)
	err = s.TestPelicanAccountConnection(c, id, "gpt-6-astra", q.Prompt, "medium")
	output, msg := parseTestSSEOutput(w.Body.String())
	if err != nil || msg != "" {
		for _, code := range []string{"astra_rotation_cooling", "astra_rotation_unavailable", "astra_rotation_use_once", "astra_rotation_no_nodes", "astra_rotation_node_unavailable", "astra_rotation_exhausted", "target_probe_degraded", "target_route_changed", "target_probe_failed", "target_validation_in_progress", "preparation_in_progress", "source_probe_cooldown", "no_qualified_source_route", "configuration_changed"} {
			if strings.Contains(msg, code) || (err != nil && strings.Contains(err.Error(), code)) {
				answer := ""
				if provider, ok := s.httpUpstream.(AstraGatewayRuntimeProvider); ok {
					for _, row := range provider.AstraGatewaySnapshot(ctx).Targets {
						if row.AccountID == id {
							answer = row.Answer
							break
						}
					}
				}
				return answer, errors.New(code)
			}
		}
		return "", errors.New("upstream_test_failed")
	}
	if w.overflow {
		return "", errors.New("test_output_too_large")
	}
	return evaluateAstraAnswer(q, output)
}

// Keep diagnostics actionable without exposing upstream error bodies or headers.
func astraWSTestFailureReason(err error) string {
	var admission *OpenAITurnAdmissionError
	if errors.As(err, &admission) {
		switch admission.Reason {
		case "group_membership_changed":
			return "ws_group_membership_changed"
		case "latest_state_unavailable":
			return "ws_latest_state_unavailable"
		case "account_ineligible", "account_runtime_blocked", "model_runtime_blocked", "model_rate_limited":
			return "ws_account_ineligible"
		default:
			return "ws_admission_denied"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "ws_test_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "ws_test_cancelled"
	}
	var fallback *openAIWSFallbackError
	if errors.As(err, &fallback) {
		return "ws_upstream_failed"
	}
	return "ws_test_failed"
}

// Read delta text from the complete forwarded SSE stream. Some Codex terminals
// legitimately have an empty output array, so non-streaming JSON loses text.
func astraWSValidationText(stream string) string {
	var delta strings.Builder
	terminalText := ""
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		event := gjson.Parse(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		switch event.Get("type").String() {
		case "response.output_text.delta":
			_, _ = delta.WriteString(event.Get("delta").String())
		case "response.completed", "response.done":
			var text strings.Builder
			for _, item := range event.Get("response.output").Array() {
				for _, part := range item.Get("content").Array() {
					if part.Get("type").String() == "output_text" {
						_, _ = text.WriteString(part.Get("text").String())
					}
				}
			}
			terminalText = text.String()
		}
	}
	if delta.Len() > 0 {
		return delta.String()
	}
	return terminalText
}
