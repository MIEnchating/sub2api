//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type ticketScheduledRequest struct {
	id           int64
	model, proxy string
	done         chan struct{}
}
type ticketScheduledUpstream struct {
	HTTPUpstream
	requests chan ticketScheduledRequest
}

func (u *ticketScheduledUpstream) Do(req *http.Request, proxy string, id int64, _ int) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	call := ticketScheduledRequest{id: id, model: extractOpenAICodexTicketModel(body), proxy: proxy, done: make(chan struct{})}
	select {
	case u.requests <- call:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	select {
	case <-call.done:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody}, nil
}
func nextTicketRequest(t *testing.T, u *ticketScheduledUpstream) ticketScheduledRequest {
	t.Helper()
	select {
	case c := <-u.requests:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("expected a ticket request")
		return ticketScheduledRequest{}
	}
}
func ticketJobIdle(s *OpenAIGatewayService, id int64, model string) bool {
	r := &s.openaiCodexTicketScheduler
	r.mu.Lock()
	defer r.mu.Unlock()
	j := r.jobs[openAICodexTicketKey(id, model)]
	return j != nil && !j.InProgress
}
func activeTicketAccounts(ids ...int64) []Account {
	var accounts []Account
	for _, id := range ids {
		a := ticketTestAccount(id)
		a.Status = StatusActive
		a.Credentials["plan_type"] = "pro"
		accounts = append(accounts, *a)
	}
	return accounts
}
func TestCodexTicketSchedulerSlowAccountDoesNotDelayAnotherRetry(t *testing.T) {
	upstream := &ticketScheduledUpstream{requests: make(chan ticketScheduledRequest, 10)}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}}, upstream)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); svc.openaiCodexTicketScheduler.workers.Wait() }()
	accounts := activeTicketAccounts(1, 2)
	svc.dispatchOpenAICodexTickets(ctx, accounts)
	first, second := nextTicketRequest(t, upstream), nextTicketRequest(t, upstream)
	slow, fast := first, second
	if slow.id != 1 {
		slow, fast = fast, slow
	}
	close(fast.done)
	require.Eventually(t, func() bool { return ticketJobIdle(svc, fast.id, fast.model) }, time.Second, time.Millisecond)
	dueTicketJobs(svc)
	svc.dispatchOpenAICodexTickets(ctx, accounts)
	retry := nextTicketRequest(t, upstream)
	require.Equal(t, fast.id, retry.id, "another account retries before the slow request completes")
	close(retry.done)
	close(slow.done)
}
func TestCodexTicketSchedulerRespectsGlobalAndAccountLimitsAndDoesNotOverlapKeys(t *testing.T) {
	upstream := &ticketScheduledUpstream{requests: make(chan ticketScheduledRequest, 10)}

	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, HarvestMaxConcurrent: 2, HarvestProxyConcurrency: 1, HarvestAccountConcurrency: 1}, upstream)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); svc.openaiCodexTicketScheduler.workers.Wait() }()
	accounts := activeTicketAccounts(1, 2, 3, 4)
	for i := range accounts {
		id := int64(i%2 + 1)
		accounts[i].ProxyID = &id
		accounts[i].Proxy = &Proxy{ID: id, Protocol: "http", Host: fmt.Sprintf("proxy%d.example", id), Port: 80}
	}
	svc.dispatchOpenAICodexTickets(ctx, accounts)
	a, b := nextTicketRequest(t, upstream), nextTicketRequest(t, upstream)
	require.NotEqual(t, a.id, b.id)
	// HarvestProxyConcurrency is enforced per configured proxy. Accounts 1 and
	// 2 use different exits, so both may be admitted under the global limit.
	require.NotEqual(t, a.proxy, b.proxy)
	svc.dispatchOpenAICodexTickets(ctx, accounts)
	select {
	case <-upstream.requests:
		t.Fatal("exceeded harvest limit")
	default:
	}
	close(a.done)
	close(b.done)
	svc.openaiCodexTicketScheduler.workers.Wait()
	// Fresh, unattempted accounts/models sort before failed jobs, preventing starvation.
	svc.dispatchOpenAICodexTickets(ctx, accounts)
	c, d := nextTicketRequest(t, upstream), nextTicketRequest(t, upstream)
	require.False(t, (c.id == a.id && c.model == a.model) || (c.id == b.id && c.model == b.model))
	require.False(t, (d.id == a.id && d.model == a.model) || (d.id == b.id && d.model == b.model))
	close(c.done)
	close(d.done)
}
func TestCodexTicketSchedulerFailureBackoffAndConfigurationRecovery(t *testing.T) {
	var mu sync.Mutex
	count := 0
	upstream := &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		count++
		return &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"model_not_found","message":"sensitive upstream information"}}`))}, nil
	}}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, upstream)
	account := activeTicketAccounts(1)[0]
	proxyID := int64(9)
	account.ProxyID = &proxyID
	account.Proxy = &Proxy{ID: proxyID, Protocol: "http", Host: "account.example", Port: 80, Username: "user", Password: "password"}
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
	statuses := OpenAICodexTicketStatuses(&account, svc.openAICodexTicketConfig(), time.Now())
	svc.EnrichOpenAICodexTicketDiagnostics(&account, statuses)
	status := statuses[0]
	require.Equal(t, 1, status.Attempts)
	require.True(t, status.Paused)
	require.Equal(t, "model_unsupported", status.LastErrorCode)
	require.WithinDuration(t, time.Now().Add(30*time.Minute), *status.NextRetryAt, 2*time.Second)
	payload, err := json.Marshal(statuses)
	require.NoError(t, err)
	for _, secret := range []string{"sensitive upstream information", "password", "tok", "account.example"} {
		require.NotContains(t, string(payload), secret)
	}
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
	require.Equal(t, 1, count)
	account.Credentials["access_token"] = "updated-token"
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
	require.Equal(t, 2, count, "updating credentials should release the stale pause")
}
func TestCodexTicketSchedulerProxyCooldownDoesNotPenalizeLengthMismatch(t *testing.T) {

	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody}, nil
	}})
	account := activeTicketAccounts(1)[0]
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")

	health := svc.openaiCodexTicketScheduler.proxies[""]
	require.Zero(t, health.failures)
	require.True(t, health.cooldown.IsZero())
	statuses := OpenAICodexTicketStatuses(&account, svc.openAICodexTicketConfig(), time.Now())
	svc.EnrichOpenAICodexTicketDiagnostics(&account, statuses)
	require.Equal(t, "length_mismatch", statuses[0].LastErrorCode)
	require.Equal(t, 0, statuses[0].LastLength)
}
func TestCodexTicketSchedulerRateLimitAndNetworkFailures(t *testing.T) {
	for _, test := range []struct {
		name, code string
		status     int
		after      string
		err        error
		minimum    time.Duration
		proxyBad   bool
	}{
		{name: "rate", code: "rate_limited", status: 429, after: "120", minimum: 120 * time.Second},
		{name: "network", code: "network", err: errors.New("connect to http://user:secret@proxy.example failed"), minimum: 6 * time.Second, proxyBad: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
				if test.err != nil {
					return nil, test.err
				}
				return &http.Response{StatusCode: test.status, Header: http.Header{"Retry-After": []string{test.after}}, Body: http.NoBody}, nil
			}})
			account := activeTicketAccounts(1)[0]
			before := time.Now()
			svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
			statuses := OpenAICodexTicketStatuses(&account, svc.openAICodexTicketConfig(), time.Now())
			svc.EnrichOpenAICodexTicketDiagnostics(&account, statuses)
			require.Equal(t, test.code, statuses[0].LastErrorCode)
			require.False(t, statuses[0].NextRetryAt.Before(before.Add(test.minimum)))
			require.Equal(t, test.proxyBad, !svc.openaiCodexTicketScheduler.proxies[""].cooldown.IsZero())
		})
	}
}
func TestCodexTicketSchedulerOffCancelsInflightAndClearsDiagnostics(t *testing.T) {
	upstream := &ticketScheduledUpstream{requests: make(chan ticketScheduledRequest, 10)}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, upstream)
	accounts := activeTicketAccounts(1)
	svc.dispatchOpenAICodexTickets(context.Background(), accounts)
	_ = nextTicketRequest(t, upstream)
	svc.cfg.Gateway.OpenAICodexTicket.Enabled = false
	svc.dispatchOpenAICodexTickets(context.Background(), accounts)
	done := make(chan struct{})
	go func() { svc.openaiCodexTicketScheduler.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disabling gateway did not cancel in-flight request")
	}
	require.Empty(t, svc.openaiCodexTicketScheduler.jobs)
	require.Zero(t, svc.openaiCodexTicketScheduler.active)
	require.Empty(t, OpenAICodexTicketStatuses(&accounts[0], svc.openAICodexTicketConfig(), time.Now()))
}
func TestCodexTicketSchedulerMissingPlanVisibleAndNoProxyDoesNotCountAttempt(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, &httpUpstreamRecorder{})
	account := ticketTestAccount(1)
	id := int64(9)
	account.ProxyID = &id // A configured proxy whose relation is missing cannot become direct.
	svc.probeOnceOpenAICodexTicket(context.Background(), account, "gpt-6-astra")
	statuses := OpenAICodexTicketStatuses(account, svc.openAICodexTicketConfig(), time.Now())
	svc.EnrichOpenAICodexTicketDiagnostics(account, statuses)
	require.False(t, statuses[0].PlanKnown)
	require.Zero(t, statuses[0].Attempts)
	require.Equal(t, "no_proxy", statuses[0].LastErrorCode)
	account.Credentials["plan_type"] = "team"
	require.True(t, codexTicketPlanKnown(account))
}
func TestCodexTicketScheduler401RefreshUsesCoordinatorAndRateLimit(t *testing.T) {
	account := activeTicketAccounts(1)[0]
	account.Credentials["refresh_token"] = "refresh"
	account.Credentials["expires_at"] = fmt.Sprint(time.Now().Add(24 * time.Hour).Unix())
	repo := &refreshAPIAccountRepo{account: &account}
	executor := &refreshAPIExecutorStub{credentials: map[string]any{"access_token": "new-token", "refresh_token": "new-refresh", "expires_at": fmt.Sprint(time.Now().Add(24 * time.Hour).Unix())}}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{}, nil)
	svc.openAITokenProvider = &OpenAITokenProvider{refreshAPI: NewOAuthRefreshAPI(repo, nil), executor: executor}
	require.True(t, svc.refreshOpenAICodexTicketAuthentication(context.Background(), &account, "tok"))
	require.Equal(t, 1, executor.refreshCalls)
	require.False(t, svc.refreshOpenAICodexTicketAuthentication(context.Background(), &account, "new-token"))
	require.Equal(t, 1, executor.refreshCalls)
}

func TestCodexTicketSchedulerSharedAccountExitDoesNotStarveLaterAccounts(t *testing.T) {
	upstream := &ticketScheduledUpstream{requests: make(chan ticketScheduledRequest, 100)}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, HarvestMaxConcurrent: 8, HarvestProxyConcurrency: 2, Models: []string{"gpt-6-astra"}}, upstream)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); svc.openaiCodexTicketScheduler.workers.Wait() }()
	var ids []int64
	for id := int64(1); id <= 40; id++ {
		ids = append(ids, id)
	}
	accounts := activeTicketAccounts(ids...)
	seen := map[int64]int{}
	for round := 0; round < 20; round++ {
		dueTicketJobs(svc) // Simulate all retry/capacity deadlines becoming eligible.
		svc.dispatchOpenAICodexTickets(ctx, accounts)
		a, b := nextTicketRequest(t, upstream), nextTicketRequest(t, upstream)
		seen[a.id]++
		seen[b.id]++
		close(a.done)
		close(b.done)
		svc.openaiCodexTicketScheduler.workers.Wait()
	}
	require.Len(t, seen, 40, "never-attempted accounts must be served before cycling old failures")
}

type ticketRefreshCache struct {
	OpenAITokenCache
	value   string
	deletes int
}

func (c *ticketRefreshCache) DeleteAccessToken(context.Context, string) error {
	c.deletes++
	c.value = ""
	return nil
}
func TestCodexTicketSchedulerRefreshClearsConcurrentStaleCacheRefill(t *testing.T) {
	account := activeTicketAccounts(1)[0]
	account.Credentials["refresh_token"] = "refresh"
	repo := &refreshAPIAccountRepo{account: &account}
	cache := &ticketRefreshCache{value: "tok"}
	executor := &refreshAPIExecutorStub{credentials: map[string]any{"access_token": "fresh", "refresh_token": "new"}, onRefresh: func() { cache.value = "tok" }}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{}, nil)
	svc.openAITokenProvider = &OpenAITokenProvider{refreshAPI: NewOAuthRefreshAPI(repo, nil), executor: executor, tokenCache: cache}
	require.True(t, svc.refreshOpenAICodexTicketAuthentication(context.Background(), &account, "tok"))
	require.Equal(t, 2, cache.deletes)
	require.Empty(t, cache.value)
}

func TestCodexTicketSchedulerReadFailurePreservesRetryAfter(t *testing.T) {
	account := activeTicketAccounts(1)[0]
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"600"}}, Body: http.NoBody}, nil
	}})
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
	before := svc.openaiCodexTicketScheduler.jobs[openAICodexTicketKey(1, "gpt-6-astra")]
	svc.accountRepo = &codexTicketLifecycleRepo{list: func(context.Context) ([]Account, error) { return nil, errors.New("database temporarily unavailable") }}
	svc.refreshOpenAICodexTickets(context.Background())
	after := svc.openaiCodexTicketScheduler.jobs[openAICodexTicketKey(1, "gpt-6-astra")]
	require.Same(t, before, after)
	require.Equal(t, "rate_limited", after.LastErrorCode)
	require.True(t, after.NextRetryAt.After(time.Now().Add(9*time.Minute)))
}

func TestCodexTicketSchedulerCoolingProxyKeepsLastFailureVisible(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") }})
	account := activeTicketAccounts(1)[0]
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
	dueTicketJobs(svc)
	svc.probeOnceOpenAICodexTicket(context.Background(), &account, "gpt-6-astra")
	statuses := OpenAICodexTicketStatuses(&account, svc.openAICodexTicketConfig(), time.Now())
	svc.EnrichOpenAICodexTicketDiagnostics(&account, statuses)
	require.Equal(t, "network", statuses[0].LastErrorCode)
	require.Equal(t, 1, statuses[0].Attempts, "waiting for cooldown is not a network attempt")
	require.True(t, statuses[0].NextRetryAt.After(time.Now()))
}

// Mirror the persisted metadata projection, including a JSON round trip.
// Repository tests separately exercise the actual Redis projection writer.
func ticketSchedulerProjection(t *testing.T, full *Account) *Account {
	t.Helper()
	projected := *full
	projected.Credentials = map[string]any{"plan_type": full.GetCredential("plan_type")}
	projected.Extra = nil
	payload, err := json.Marshal(projected)
	require.NoError(t, err)
	var decoded Account
	require.NoError(t, json.Unmarshal(payload, &decoded))
	return &decoded
}

func ticketSchedulerFixture(t *testing.T, advanced, loadBatch bool) (*OpenAIGatewayService, *openAISnapshotCacheStub, *Account, int64) {
	t.Helper()
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	groupID := int64(2)
	account := ticketTestAccount(400)
	account.GroupIDs = []int64{groupID}
	account.Concurrency = 10
	account.Credentials["email"] = "fixture@example.invalid"
	account.Credentials["plan_type"] = "pro"
	account.Extra = map[string]any{
		openAICodexTicketExtraKey("gpt-6-astra"): &openAICodexTicket{
			AccountID: account.ID, Model: "gpt-6-astra",
			State: fakeCodexTicketState(292), Length: 292,
			CapturedAt: time.Now(), IssuedAt: time.Now(),
			ExpiresAt: time.Now().Add(30 * time.Minute), Identity: ticketIdentity(account),
		},
	}
	cache := &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{ticketSchedulerProjection(t, account)},
		accountsByID:     map[int64]*Account{account.ID: account},
	}
	cfg := &config.Config{}
	cfg.Gateway.OpenAICodexTicket = config.OpenAICodexTicketConfig{
		Enabled: true, FailClosed: true, TargetLength: 292, Models: []string{"gpt-6-astra"},
	}
	cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
	cfg.Gateway.OpenAIWS.LBTopK = 1
	repo := schedulerTestOpenAIAccountRepo{accounts: []Account{*account}}
	svc := &OpenAIGatewayService{
		cfg: cfg, accountRepo: repo, cache: &schedulerTestGatewayCache{},
		schedulerSnapshot:  &SchedulerSnapshotService{cache: cache, accountRepo: repo},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	if advanced {
		svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
	}
	return svc, cache, account, groupID
}

func TestCodexTicketSchedulerProjectedCandidate(t *testing.T) {
	for _, path := range []struct {
		name                string
		advanced, loadBatch bool
	}{
		{"advanced", true, true},
		{"legacy_batch", false, true},
		{"legacy_nonbatch", false, false},
	} {
		for _, warm := range []bool{false, true} {
			name := path.name + "/cold"
			if warm {
				name = path.name + "/warm"
			}
			t.Run(name, func(t *testing.T) {
				svc, _, account, groupID := ticketSchedulerFixture(t, path.advanced, path.loadBatch)
				if warm {
					require.NotNil(t, svc.lookupOpenAICodexTicket(account, "gpt-6-astra"))
				}
				result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "",
					"gpt-6-astra", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, account.ID, result.Account.ID)
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
			})
		}
	}
}

func TestCodexTicketSchedulerAdmissionSafety(t *testing.T) {
	fixtureTicket := func(t *testing.T, a *Account) *openAICodexTicket {
		t.Helper()
		ticket, ok := a.Extra[openAICodexTicketExtraKey("gpt-6-astra")].(*openAICodexTicket)
		require.True(t, ok, "fixture must contain a typed ticket")
		require.NotNil(t, ticket)
		return ticket
	}
	for _, advanced := range []bool{false, true} {
		mode := "legacy"
		if advanced {
			mode = "advanced"
		}
		for _, tc := range []struct {
			name string
			edit func(*testing.T, *OpenAIGatewayService, *Account)
			ok   bool
		}{
			{"missing", func(_ *testing.T, _ *OpenAIGatewayService, a *Account) { a.Extra = nil }, false},
			{"expired", func(t *testing.T, _ *OpenAIGatewayService, a *Account) {
				fixtureTicket(t, a).ExpiresAt = time.Now().Add(-time.Minute)
			}, false},
			{"changed_identity", func(_ *testing.T, _ *OpenAIGatewayService, a *Account) {
				a.Credentials["chatgpt_account_id"] = "changed-fixture"
			}, false},
			{"revoked_in_memory", func(t *testing.T, s *OpenAIGatewayService, a *Account) {
				tombstone := *fixtureTicket(t, a)
				tombstone.Revoked = true
				s.openaiCodexTickets.Store(openAICodexTicketKey(a.ID, "gpt-6-astra"), &tombstone)
			}, false},
			{"valid_standby", func(t *testing.T, _ *OpenAIGatewayService, a *Account) {
				ticket := fixtureTicket(t, a)
				standby := *ticket
				ticket.ExpiresAt = time.Now().Add(-time.Minute)
				ticket.Standby = &standby
			}, true},
			{"stopped_after_snapshot", func(_ *testing.T, _ *OpenAIGatewayService, a *Account) { a.Schedulable = false }, false},
			{"removed_group_after_snapshot", func(_ *testing.T, _ *OpenAIGatewayService, a *Account) { a.GroupIDs = nil }, false},
			{"database_final_recheck_stopped", func(_ *testing.T, s *OpenAIGatewayService, a *Account) {
				latest := *a
				latest.Schedulable = false
				s.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{latest}}
			}, false},
			{"database_final_recheck_removed_group", func(_ *testing.T, s *OpenAIGatewayService, a *Account) {
				latest := *a
				latest.GroupIDs = nil
				s.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{latest}}
			}, false},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				svc, _, account, groupID := ticketSchedulerFixture(t, advanced, true)
				tc.edit(t, svc, account)
				result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "",
					"gpt-6-astra", nil, OpenAIUpstreamTransportAny, false)
				if tc.ok {
					require.NoError(t, err)
					require.NotNil(t, result)
				} else {
					require.Error(t, err)
					require.Nil(t, result)
				}
				if result != nil && result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
			})
		}
	}
}

func TestCodexTicketSchedulerTopKSkipsTicketlessAccount(t *testing.T) {
	svc, cache, valid, groupID := ticketSchedulerFixture(t, true, true)
	ticketless := *valid
	ticketless.ID = 399
	ticketless.Priority = -100
	ticketless.Extra = nil
	cache.snapshotAccounts = append([]*Account{ticketSchedulerProjection(t, &ticketless)}, cache.snapshotAccounts...)
	cache.accountsByID[ticketless.ID] = &ticketless
	svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{ticketless, *valid}}
	result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "",
		"gpt-6-astra", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, valid.ID, result.Account.ID)
	if result.ReleaseFunc != nil {
		result.ReleaseFunc()
	}
}

type ticketCountingSnapshotCache struct {
	*openAISnapshotCacheStub
	batchReads int
	err        error
}

func (c *ticketCountingSnapshotCache) GetAccounts(_ context.Context, ids []int64) (map[int64]*Account, error) {
	c.batchReads++
	if c.err != nil {
		return nil, c.err
	}
	out := make(map[int64]*Account, len(ids))
	for _, id := range ids {
		out[id] = c.accountsByID[id]
	}
	return out, nil
}

func TestCodexTicketCandidateHydrationScope(t *testing.T) {
	for _, name := range []string{"enabled", "fail_open", "disabled", "ungated", "excluded", "compact_ungated"} {
		t.Run(name, func(t *testing.T) {
			svc, snapshot, account, groupID := ticketSchedulerFixture(t, true, true)
			cache := &ticketCountingSnapshotCache{openAISnapshotCacheStub: snapshot}
			svc.schedulerSnapshot.cache = cache
			model, compact := "gpt-6-astra", false
			var excluded map[int64]struct{}
			switch name {
			case "fail_open":
				svc.cfg.Gateway.OpenAICodexTicket.FailClosed = false
			case "disabled":
				svc.cfg.Gateway.OpenAICodexTicket.Enabled = false
			case "ungated":
				model = "gpt-6-sol"
			case "excluded":
				excluded = map[int64]struct{}{account.ID: {}}
			case "compact_ungated":
				compact = true
				svc.cfg.Gateway.OpenAICompactModel = "gpt-6-sol"
			}
			_, err := svc.listSchedulableAccountsForRequest(context.Background(), &groupID,
				PlatformOpenAI, model, compact, excluded)
			require.NoError(t, err)
			if name == "enabled" {
				require.Equal(t, 1, cache.batchReads)
			} else {
				require.Zero(t, cache.batchReads)
			}
		})
	}
}

func TestCodexTicketSchedulerCompactUsesOutboundModel(t *testing.T) {
	for _, path := range []struct {
		name                string
		advanced, loadBatch bool
	}{
		{"advanced", true, true},
		{"legacy_batch", false, true},
		{"legacy_nonbatch", false, false},
	} {
		for _, scenario := range []string{"ungated_outbound", "gated_valid_ticket", "gated_missing_ticket"} {
			t.Run(path.name+"/"+scenario, func(t *testing.T) {
				svc, _, account, groupID := ticketSchedulerFixture(t, path.advanced, path.loadBatch)
				requested, outbound := "gpt-6-sol", "gpt-6-astra"
				if scenario == "ungated_outbound" {
					requested, outbound = outbound, requested
				}
				if scenario != "gated_valid_ticket" {
					account.Extra = nil
				}
				svc.cfg.Gateway.OpenAICompactModel = outbound
				result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "",
					requested, nil, OpenAIUpstreamTransportAny, true)
				if scenario == "gated_missing_ticket" {
					require.Error(t, err)
					require.Nil(t, result)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
			})
		}
	}
}

func TestCodexTicketSchedulerStorageFailureDoesNotFailOpen(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "advanced"}[advanced], func(t *testing.T) {
			svc, snapshot, _, groupID := ticketSchedulerFixture(t, advanced, true)
			cache := &ticketCountingSnapshotCache{openAISnapshotCacheStub: snapshot, err: errors.New("cache unavailable")}
			svc.schedulerSnapshot.cache = cache
			svc.schedulerSnapshot.cfg = svc.cfg // DB fallback explicitly disabled.
			result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "",
				"gpt-6-astra", nil, OpenAIUpstreamTransportAny, false)
			require.ErrorContains(t, err, "codex ticket scheduling state unavailable")
			require.Nil(t, result)
			require.Equal(t, 1, cache.batchReads)
		})
	}
}

func TestCodexTicketSchedulerAffinityCannotBypassFinalState(t *testing.T) {
	for _, binding := range []string{"session", "previous_response"} {
		for _, mutation := range []string{"stopped", "removed_group"} {
			t.Run(binding+"/"+mutation, func(t *testing.T) {
				svc, snapshot, stale, groupID := ticketSchedulerFixture(t, true, true)
				svc.cfg.Gateway.OpenAIWS.LBTopK = 2 // Include the healthy fallback in the probe budget.
				backup := *stale
				backup.ID = 401
				backup.Priority = 10
				snapshot.snapshotAccounts = append(snapshot.snapshotAccounts, ticketSchedulerProjection(t, &backup))
				snapshot.accountsByID[backup.ID] = &backup
				latest := *stale
				if mutation == "stopped" {
					latest.Schedulable = false
				} else {
					latest.GroupIDs = nil
				}
				svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{latest, backup}}
				previous, session := "", ""
				if binding == "session" {
					session = "synthetic-session"
					svc.cache = &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:" + session: stale.ID}}
				} else {
					previous = "resp_synthetic"
					svc.cfg.Gateway.OpenAIWS.Enabled = true
					svc.cfg.Gateway.OpenAIWS.OAuthEnabled = true
					svc.cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
					require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), groupID, previous, stale.ID, time.Hour))
				}
				result, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, previous, session,
					"gpt-6-astra", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, backup.ID, result.Account.ID)
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
			})
		}
	}
}
