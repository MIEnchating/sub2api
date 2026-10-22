package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type qualityBudgetHTTPUpstream struct {
	HTTPUpstream
	send func(*http.Request, string, int64, int) (*http.Response, error)
}

func (s qualityBudgetHTTPUpstream) Do(req *http.Request, proxy string, account int64, concurrency int) (*http.Response, error) {
	return s.send(req, proxy, account, concurrency)
}

func TestAntigravityQualityBudgetBeforeSingleTransportAttempt(t *testing.T) {
	for _, status := range []int{0, http.StatusOK, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			account := &Account{ID: 42, Platform: PlatformAntigravity, Extra: map[string]any{AntiDegradationExtraKey: true}}
			admissions, calls := 0, 0
			repo := &qualityOperationsStub{reserve: func(_ context.Context, id, tokens int64, _ bool, _ string) (*QualityProbeReservation, error) {
				admissions++
				require.Equal(t, account.ID, id)
				require.Positive(t, tokens)
				if status == 0 {
					return nil, &QualityDeferredError{Reason: "budget", Until: time.Now().Add(time.Hour)}
				}
				return &QualityProbeReservation{ID: 1, AccountID: id}, nil
			}}
			collector := &accountTestUsageCollector{}
			ctx := context.WithValue(context.Background(), qualityOperationsKey{}, repo)
			ctx = context.WithValue(ctx, accountTestUsageKey{}, collector)
			upstream := qualityBudgetHTTPUpstream{send: func(_ *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
				calls++
				require.Equal(t, 1, admissions, "admission must precede every paid send")
				require.Equal(t, account.ID, id)
				require.Equal(t, "http://proxy.example:8080", proxy)
				require.Equal(t, AntiDegradeConcurrencyCap, concurrency)
				return costJSONResponse(status, `{"usage":{"input_tokens":10,"output_tokens":2}}`), nil
			}}
			result, err := (&AntigravityGatewayService{}).antigravityRetryLoop(antigravityRetryLoopParams{
				ctx: ctx, account: account, accessToken: "test-token", action: "generateContent",
				body: []byte(`{"input":"test"}`), proxyURL: "http://proxy.example:8080", httpUpstream: upstream,
			})
			require.Equal(t, 1, admissions)
			if status == 0 {
				var deferred *QualityDeferredError
				require.ErrorAs(t, err, &deferred)
				require.Zero(t, calls)
				require.Nil(t, result)
				require.Empty(t, collector.snapshot().Attempts)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, calls, "scheduled retries must own their budget, including 429s")
			require.Equal(t, status, result.resp.StatusCode)
			_, err = io.ReadAll(result.resp.Body)
			require.NoError(t, err)
			require.NoError(t, result.resp.Body.Close())
			collector.finishQualitySettlements("")
			require.Len(t, collector.snapshot().Attempts, 1)
			require.Len(t, repo.outcomes, 1)
			require.EqualValues(t, 12, *repo.outcomes[0].Tokens)
		})
	}
}

func TestAntigravityCustomerRequestPreservesAccount429Retry(t *testing.T) {
	retries, calls := 1, 0
	account := &Account{ID: 42, Platform: PlatformAntigravity, Concurrency: 3, RateLimit429RetryCount: &retries}
	repo := &qualityOperationsStub{reserve: func(context.Context, int64, int64, bool, string) (*QualityProbeReservation, error) {
		t.Fatal("customer traffic must not consume the quality budget")
		return nil, nil
	}}
	ctx := context.WithValue(context.Background(), qualityOperationsKey{}, repo)
	upstream := qualityBudgetHTTPUpstream{send: func(req *http.Request, _ string, _ int64, concurrency int) (*http.Response, error) {
		calls++
		require.Equal(t, account.Concurrency, concurrency)
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"input":"test"}`, string(body))
		status := http.StatusOK
		if calls == 1 {
			status = http.StatusTooManyRequests
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}}
	result, err := (&AntigravityGatewayService{}).antigravityRetryLoop(antigravityRetryLoopParams{
		ctx: ctx, account: account, accessToken: "test-token", action: "generateContent", body: []byte(`{"input":"test"}`), httpUpstream: upstream,
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, http.StatusOK, result.resp.StatusCode)
	require.NoError(t, result.resp.Body.Close())
}
