package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newCyberBlockTestCtx(headers map[string]string, body string) (*gin.Context, []byte) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/openai/v1/responses", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c, []byte(body)
}

func TestCyberSessionExplicitBlockKey(t *testing.T) {
	c1, b1 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	k1 := CyberSessionExplicitBlockKey(101, c1, b1)
	require.NotEmpty(t, k1)

	// Same session, different apiKey → different key (isolation).
	c2, b2 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	require.NotEqual(t, k1, CyberSessionExplicitBlockKey(202, c2, b2))

	// Same session + same apiKey → stable key.
	c3, b3 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	require.Equal(t, k1, CyberSessionExplicitBlockKey(101, c3, b3))

	// A cache hint does not establish a conversation identity.
	c4, b4 := newCyberBlockTestCtx(nil, `{"prompt_cache_key":"pck-1"}`)
	require.Empty(t, CyberSessionExplicitBlockKey(101, c4, b4))

	// No explicit signal → empty key → caller must skip blocking entirely.
	c5, b5 := newCyberBlockTestCtx(nil, `{"input":"hello world"}`)
	require.Empty(t, CyberSessionExplicitBlockKey(101, c5, b5))

	// conversation_id header counts as explicit; key is stable and non-empty.
	c6, b6 := newCyberBlockTestCtx(map[string]string{"conversation_id": "conv-xyz"}, `{}`)
	k6 := CyberSessionExplicitBlockKey(101, c6, b6)
	require.NotEmpty(t, k6)
	c6b, b6b := newCyberBlockTestCtx(map[string]string{"conversation_id": "conv-xyz"}, `{}`)
	require.Equal(t, k6, CyberSessionExplicitBlockKey(101, c6b, b6b), "conversation_id key must be stable")
}

func TestCyberTranscriptBlocksRequireModelGeneratedHistory(t *testing.T) {
	first := []byte(`{"instructions":"shared","input":[{"role":"user","content":"fixed environment"},{"role":"user","content":"question one"}]}`)
	second := []byte(`{"instructions":"shared","input":[{"role":"user","content":"fixed environment"},{"role":"user","content":"question two"}]}`)
	firstBlocks := CyberSessionTranscriptBlocks(77, first)
	secondBlocks := CyberSessionTranscriptBlocks(77, second)
	require.Len(t, firstBlocks, 1)
	require.Len(t, secondBlocks, 1)
	require.NotEqual(t, firstBlocks[0].Key, secondBlocks[0].Key)

	hit := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}`)
	continuation := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"different trigger"},{"role":"assistant","content":"blocked"},{"role":"user","content":"continue"}]}`)
	hitBlocks := CyberSessionTranscriptBlocks(77, hit)
	require.Equal(t, []int{3, 2}, []int{hitBlocks[0].Length, hitBlocks[1].Length})
	require.Equal(t, []string{hitBlocks[1].Key}, cyberSessionTranscriptKeysAt(77, continuation, []int{hitBlocks[1].Length}))
	require.NotEqual(t, []string{hitBlocks[0].Key}, cyberSessionTranscriptKeysAt(77, continuation, []int{hitBlocks[0].Length}))
}

func TestCyberTranscriptBlocksWebSocketResponseCreate(t *testing.T) {
	body := []byte(`{"type":"response.create","response":{"prompt_cache_key":"ws-session","input":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}}`)
	c, _ := newCyberBlockTestCtx(nil, string(body))
	require.Empty(t, CyberSessionExplicitBlockKey(88, c, body))
	require.Len(t, CyberSessionTranscriptBlocks(88, body), 2)
}

func TestCyberTranscriptBlocksStringInputMatchesSingleItemHistory(t *testing.T) {
	blocks := CyberSessionTranscriptBlocks(77, []byte(`{"input":"hello"}`))
	require.Equal(t, []CyberSessionTranscriptBlock{{Key: blocks[0].Key, Length: 1}}, blocks)
	require.Equal(t, []string{blocks[0].Key}, cyberSessionTranscriptKeysAt(77, []byte(`{"input":["hello","more"]}`), []int{1}))
	require.Empty(t, CyberSessionTranscriptBlocks(77, []byte(`{"input":"  "}`)))
}

func TestCyberTranscriptKeysAtRecomputesRecordedLengthsInOrder(t *testing.T) {
	messages := make([]map[string]string, 300)
	for i := range messages {
		messages[i] = map[string]string{"role": "user", "content": "message-" + strconv.Itoa(i)}
	}
	body, err := json.Marshal(map[string]any{"messages": messages})
	require.NoError(t, err)
	prefixKey := func(length int) string {
		prefix, err := json.Marshal(map[string]any{"messages": messages[:length]})
		require.NoError(t, err)
		return CyberSessionTranscriptBlocks(77, prefix)[0].Key
	}

	require.Equal(t, []string{prefixKey(1), prefixKey(45), prefixKey(300)},
		cyberSessionTranscriptKeysAt(77, body, []int{300, 45, 1, 45, 301, 0}))
	require.Empty(t, cyberSessionTranscriptKeysAt(77, body, nil))
	require.Equal(t, CyberSessionTranscriptBlocks(77, body)[0], CyberSessionTranscriptBlock{Key: prefixKey(300), Length: 300})
}

// --- fakes ---

type fakeCyberBlockStore struct {
	findCalls int
	findErr   error
	blocked   map[string]bool
	scopes    map[string]bool
	lengths   map[int64][]int
	findKeys  [][]string
}

var _ CyberSessionBlockStore = (*fakeCyberBlockStore)(nil)

func (f *fakeCyberBlockStore) SetCyberSessionBlocked(_ context.Context, block CyberSessionBlock, _ time.Duration) error {
	if f.blocked == nil {
		f.blocked = map[string]bool{}
	}
	for _, key := range block.Keys {
		f.blocked[key] = true
	}
	if block.ScopeKey != "" {
		if f.scopes == nil {
			f.scopes = map[string]bool{}
			f.lengths = map[int64][]int{}
		}
		f.scopes[block.ScopeKey] = true
		f.lengths[block.APIKeyID] = append(f.lengths[block.APIKeyID], block.TranscriptLengths...)
	}
	return nil
}

func (f *fakeCyberBlockStore) CyberSessionTranscriptLengths(_ context.Context, apiKeyID int64, scopeKey string) ([]int, error) {
	if !f.scopes[scopeKey] {
		return nil, nil
	}
	return f.lengths[apiKeyID], nil
}

func (f *fakeCyberBlockStore) FindCyberSessionBlocked(_ context.Context, keys []string) (string, error) {
	f.findCalls++
	if f.findErr != nil {
		return "", f.findErr
	}
	f.findKeys = append(f.findKeys, keys)
	for _, key := range keys {
		if f.blocked[key] {
			return key, nil
		}
	}
	return "", nil
}

// fakeSettingRepo is a minimal SettingRepository stub for unit tests.
// Only GetValue is exercised by GetCyberSessionBlockRuntime; all other methods
// panic so accidental calls are caught immediately.
type fakeSettingRepo struct {
	vals map[string]string
}

func (r *fakeSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	v, ok := r.vals[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (r *fakeSettingRepo) Get(_ context.Context, _ string) (*Setting, error) {
	panic("fakeSettingRepo.Get not implemented")
}
func (r *fakeSettingRepo) Set(_ context.Context, _, _ string) error {
	panic("fakeSettingRepo.Set not implemented")
}
func (r *fakeSettingRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	panic("fakeSettingRepo.GetMultiple not implemented")
}
func (r *fakeSettingRepo) SetMultiple(_ context.Context, _ map[string]string) error {
	panic("fakeSettingRepo.SetMultiple not implemented")
}
func (r *fakeSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	panic("fakeSettingRepo.GetAll not implemented")
}
func (r *fakeSettingRepo) Delete(_ context.Context, _ string) error {
	panic("fakeSettingRepo.Delete not implemented")
}

var _ SettingRepository = (*fakeSettingRepo)(nil)

// comboCacheAndStore implements both GatewayCache (no-op stubs) and
// CyberSessionBlockStore (delegates to fakeCyberBlockStore) so it can be
// injected as s.cache and successfully type-asserted to CyberSessionBlockStore.
type comboCacheAndStore struct {
	store fakeCyberBlockStore
}

var _ GatewayCache = (*comboCacheAndStore)(nil)
var _ CyberSessionBlockStore = (*comboCacheAndStore)(nil)

func (c *comboCacheAndStore) GetSessionAccountID(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, errors.New("stub")
}
func (c *comboCacheAndStore) SetSessionAccountID(_ context.Context, _ int64, _ string, _ int64, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) RefreshSessionTTL(_ context.Context, _ int64, _ string, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) DeleteSessionAccountID(_ context.Context, _ int64, _ string) error {
	return nil
}

func (c *comboCacheAndStore) SetGrokVideoPendingBilling(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) GetGrokVideoPendingBilling(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}
func (c *comboCacheAndStore) ClaimGrokVideoBilled(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (c *comboCacheAndStore) ReleaseGrokVideoBilled(_ context.Context, _ string) error {
	return nil
}

func (c *comboCacheAndStore) SetReasoningContent(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) GetReasoningContent(_ context.Context, _ string) (string, error) {
	return "", ErrReasoningContentNotFound
}

func (c *comboCacheAndStore) SetCyberSessionBlocked(ctx context.Context, block CyberSessionBlock, ttl time.Duration) error {
	return c.store.SetCyberSessionBlocked(ctx, block, ttl)
}
func (c *comboCacheAndStore) CyberSessionTranscriptLengths(ctx context.Context, apiKeyID int64, scopeKey string) ([]int, error) {
	return c.store.CyberSessionTranscriptLengths(ctx, apiKeyID, scopeKey)
}
func (c *comboCacheAndStore) FindCyberSessionBlocked(ctx context.Context, keys []string) (string, error) {
	return c.store.FindCyberSessionBlocked(ctx, keys)
}

// --- tests ---

// TestIsCyberSessionBlocked_EmptyKeyAndNilService covers the fail-open paths:
// empty key, nil service, store missing → always false / no panic.
func TestFindCyberSessionBlocked_EmptyAndNilService(t *testing.T) {
	var nilSvc *OpenAIGatewayService
	require.Empty(t, nilSvc.FindCyberSessionBlockedForRequest(context.Background(), 1, nil, nil, "", ""))
	require.NotPanics(t, func() { nilSvc.MarkCyberSessionBlocked(context.Background(), CyberSessionBlock{Keys: []string{"k"}}) })

	svc := &OpenAIGatewayService{}
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(context.Background(), 1, nil, nil, "", ""))
}

// TestCyberSessionBlock_RoundTrip exercises the type-assertion success path:
// mark a session blocked via a combo cache+store, then confirm IsCyberSessionBlocked
// returns true, and an unrelated key returns false.
func TestCyberSessionBlock_RoundTrip(t *testing.T) {
	// SettingService with only settingRepo set — GetCyberSessionBlockRuntime needs
	// nothing else (cfg/proxyRepo/etc. are not touched by this code path).
	settingSvc := &SettingService{
		settingRepo: &fakeSettingRepo{
			vals: map[string]string{
				SettingKeyCyberSessionBlockEnabled:    "true",
				SettingKeyCyberSessionBlockTTLSeconds: "60",
			},
		},
	}

	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{
		cache:          combo,
		settingService: settingSvc,
	}

	ctx := context.Background()
	const testKey = "deadbeef1234"

	c, body := newCyberBlockTestCtx(map[string]string{"session_id": "sess-roundtrip"}, `{}`)
	explicitKey := CyberSessionExplicitBlockKey(1, c, body)
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 1, c, body, "203.0.113.1", "client/1.0"))

	svc.MarkCyberSessionBlocked(ctx, CyberSessionBlock{APIKeyID: 1, Keys: []string{explicitKey, testKey}})

	require.Equal(t, explicitKey, svc.FindCyberSessionBlockedForRequest(ctx, 1, c, body, "203.0.113.1", "client/1.0"))
}

func TestFindCyberSessionBlockedForRequestUsesScopeForTranscript(t *testing.T) {
	settingSvc := &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
		SettingKeyCyberSessionBlockEnabled:    "true",
		SettingKeyCyberSessionBlockTTLSeconds: "60",
	}}}
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: settingSvc}
	ctx := context.Background()

	hitBody := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}`)
	nextBody := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"different trigger"},{"role":"assistant","content":"blocked"},{"role":"user","content":"continue"}]}`)
	nextCtx, _ := newCyberBlockTestCtx(nil, string(nextBody))
	const clientIP = "203.0.113.20"
	const userAgent = "Codex CLI 1.2.3"
	block := CyberSessionTranscriptBlocks(9, hitBody)[1]

	// Without an active source scope, transcript candidates are never blocks.
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 9, nextCtx, nextBody, clientIP, userAgent))
	scopeKey := CyberSessionScopeKey(9, clientIP, userAgent)
	svc.MarkCyberSessionBlocked(ctx, CyberSessionBlock{APIKeyID: 9, ScopeKey: scopeKey, Keys: []string{block.Key}, TranscriptLengths: []int{block.Length}})
	require.Equal(t, block.Key, svc.FindCyberSessionBlockedForRequest(ctx, 9, nextCtx, nextBody, clientIP, "Codex CLI 1.2.4"))
}

// Issue #8519: once a source scope is active, unrelated sessions with long
// histories must pass, while a blocked transcript padded with any number of
// appended items must still match its recorded prefix.
func TestFindCyberSessionBlockedForRequestMatchesLongHistoriesExactly(t *testing.T) {
	settingSvc := &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
		SettingKeyCyberSessionBlockEnabled:    "true",
		SettingKeyCyberSessionBlockTTLSeconds: "60",
	}}}
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: settingSvc}
	ctx := context.Background()
	const apiKeyID = int64(9)
	const clientIP = "203.0.113.20"
	const userAgent = "Codex CLI 1.2.3"

	history := func(prefix []map[string]string, padding int, tag string) []byte {
		items := append([]map[string]string{}, prefix...)
		for i := 0; i < padding; i++ {
			role := "user"
			if i%2 == 0 {
				role = "assistant"
			}
			items = append(items, map[string]string{"role": role, "content": tag + "-" + strconv.Itoa(i)})
		}
		body, err := json.Marshal(map[string]any{"input": items})
		require.NoError(t, err)
		return body
	}
	hitPrefix := []map[string]string{{"role": "user", "content": "setup"}, {"role": "assistant", "content": "ready"}, {"role": "user", "content": "trigger"}}
	hitBody := history(hitPrefix, 0, "")
	hitCtx, _ := newCyberBlockTestCtx(map[string]string{"session_id": "sess-hit"}, string(hitBody))
	block := CyberSessionBlock{
		APIKeyID: apiKeyID,
		ScopeKey: CyberSessionScopeKey(apiKeyID, clientIP, userAgent),
		Keys:     []string{CyberSessionExplicitBlockKey(apiKeyID, hitCtx, hitBody)},
	}
	transcripts := CyberSessionTranscriptBlocks(apiKeyID, hitBody)
	for _, transcript := range transcripts {
		block.Keys = append(block.Keys, transcript.Key)
		block.TranscriptLengths = append(block.TranscriptLengths, transcript.Length)
	}
	svc.MarkCyberSessionBlocked(ctx, block)

	for _, items := range []int{256, 257, 1000} {
		unrelated := history(nil, items, "unrelated")
		c, _ := newCyberBlockTestCtx(map[string]string{"session_id": "sess-other"}, string(unrelated))
		require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, apiKeyID, c, unrelated, clientIP, userAgent), "items=%d", items)
	}

	padded := history(hitPrefix, 1000, "padding")
	c, _ := newCyberBlockTestCtx(nil, string(padded))
	// The shorter rewrite-tolerant prefix is checked first (earliest match).
	require.Equal(t, transcripts[1].Key, svc.FindCyberSessionBlockedForRequest(ctx, apiKeyID, c, padded, clientIP, userAgent))
	require.Len(t, combo.store.findKeys[len(combo.store.findKeys)-1], len(block.TranscriptLengths),
		"lookup must only recompute the recorded prefix lengths")
}

func TestCyberSessionScopeKeyNormalizesUserAgentVersion(t *testing.T) {
	base := CyberSessionScopeKey(7, "203.0.113.10", "Codex CLI 1.2.3")
	require.NotEmpty(t, base)
	require.Equal(t, base, CyberSessionScopeKey(7, "203.0.113.10", "Codex CLI 1.2.4"))
	require.NotEqual(t, base, CyberSessionScopeKey(8, "203.0.113.10", "Codex CLI 1.2.3"))
	require.NotEqual(t, base, CyberSessionScopeKey(7, "203.0.113.11", "Codex CLI 1.2.3"))
}

func TestCyberSessionIdentityStrictEnabledDefaultsOffAndReadsSetting(t *testing.T) {
	t.Run("missing setting", func(t *testing.T) {
		svc := &OpenAIGatewayService{settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{}}}}
		require.False(t, svc.CyberSessionIdentityStrictEnabled(context.Background()))
	})

	t.Run("explicitly enabled", func(t *testing.T) {
		svc := &OpenAIGatewayService{settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
			SettingKeyCyberSessionBlockEnabled:          "true",
			SettingKeyCyberSessionIdentityStrictEnabled: "true",
		}}}}
		require.True(t, svc.CyberSessionIdentityStrictEnabled(context.Background()))
	})
}

func TestCyberSessionExplicitBlockKeyUsesTypedExplicitIdentity(t *testing.T) {
	var threadKey string
	for _, header := range []string{"conversation_id", "thread_id", "thread-id", "X-Conversation-ID"} {
		c, body := newCyberBlockTestCtx(map[string]string{header: " conversation-a "}, `{"prompt_cache_key":"shared-cache"}`)
		key := CyberSessionExplicitBlockKey(7, c, body)
		require.NotEmpty(t, key, header)
		if threadKey == "" {
			threadKey = key
		}
		require.Equal(t, threadKey, key, "thread header aliases must share one typed identity")
		require.NotEqual(t, key, CyberSessionExplicitBlockKey(8, c, body))
		require.Empty(t, CyberSessionExplicitBlockKey(0, c, body))
		require.Empty(t, CyberSessionExplicitBlockKey(-1, c, body))
	}

	var sessionKey string
	for _, header := range []string{"session-id", "session_id", "X-Session-Id", "X-OpenCode-Session"} {
		c, body := newCyberBlockTestCtx(map[string]string{header: " conversation-a "}, `{"prompt_cache_key":"shared-cache"}`)
		key := CyberSessionExplicitBlockKey(7, c, body)
		require.NotEmpty(t, key, header)
		if sessionKey == "" {
			sessionKey = key
		}
		require.Equal(t, sessionKey, key, "session header aliases must share one typed identity")
	}
	require.NotEqual(t, threadKey, sessionKey, "thread and session namespaces must not collide")

	c, body := newCyberBlockTestCtx(nil, `{"client_metadata":{"thread_id":"conversation-a"}}`)
	require.Equal(t, threadKey, CyberSessionExplicitBlockKey(7, c, body))
	c, body = newCyberBlockTestCtx(nil, `{"client_metadata":{"session_id":"conversation-a"}}`)
	require.Equal(t, sessionKey, CyberSessionExplicitBlockKey(7, c, body))
	c, body = newCyberBlockTestCtx(nil, `{"type":"response.create","response":{"client_metadata":{"thread_id":"conversation-a"}}}`)
	require.Equal(t, threadKey, CyberSessionExplicitBlockKey(7, c, body))

	c, body = newCyberBlockTestCtx(map[string]string{"thread_id": "thread-header"}, `{"client_metadata":{"thread_id":"thread-body"}}`)
	require.Empty(t, CyberSessionExplicitBlockKey(7, c, body))

	for _, body := range []string{`{}`, `{"prompt_cache_key":"conversation-a"}`, `{"type":"response.create","response":{"prompt_cache_key":"conversation-a","input":"hello"}}`} {
		c, b := newCyberBlockTestCtx(map[string]string{"X-Session-Affinity": "conversation-a"}, body)
		require.Empty(t, CyberSessionExplicitBlockKey(7, c, b))
	}
	require.Empty(t, CyberSessionExplicitBlockKey(7, nil, nil))
	require.Empty(t, CyberSessionExplicitBlockKey(7, &gin.Context{}, nil))
}

func TestResolveCyberSessionIdentityExposesOnlyHashedIdentity(t *testing.T) {
	c, body := newCyberBlockTestCtx(nil, `{"client_metadata":{"thread_id":"private-thread-value"}}`)
	resolution := ResolveCyberSessionIdentity(7, c, body)

	require.True(t, resolution.Resolved())
	require.Equal(t, OpenAIClientSessionIdentityMetadata{
		Status: OpenAIClientSessionIdentityResolved,
		Kind:   openAIClientSessionKindThread,
		Source: OpenAIClientSessionIdentitySourceBody,
	}, resolution.Metadata)
	require.NotEmpty(t, resolution.BlockKey)
	require.NotContains(t, resolution.BlockKey, "private-thread-value")
	require.Len(t, resolution.LookupKeys, 2)

	inherited := InheritCyberSessionIdentity(resolution)
	require.True(t, inherited.Resolved())
	require.True(t, inherited.Inherited)
	require.Equal(t, OpenAIClientSessionIdentitySourceConnection, inherited.Metadata.Source)
	require.Equal(t, resolution.BlockKey, inherited.BlockKey)
	require.Equal(t, resolution.LookupKeys, inherited.LookupKeys)

	missingCtx, missingBody := newCyberBlockTestCtx(nil, `{}`)
	missing := ResolveCyberSessionIdentity(7, missingCtx, missingBody)
	require.False(t, missing.Resolved())
	require.Equal(t, OpenAIClientSessionIdentityMissing, missing.Metadata.Status)
	require.Empty(t, missing.BlockKey)
	require.Empty(t, missing.LookupKeys)
}

func TestCyberSessionBlockReadsLegacyV2ExplicitKey(t *testing.T) {
	ctx := context.Background()
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
		SettingKeyCyberSessionBlockEnabled: "true",
	}}}}

	c, body := newCyberBlockTestCtx(map[string]string{"session_id": "legacy-session"}, `{}`)
	legacyRaw := "cyber-explicit-session:v2|api_key=7|session=legacy-session"
	legacySum := sha256.Sum256([]byte(legacyRaw))
	legacyKey := hex.EncodeToString(legacySum[:])
	combo.store.blocked = map[string]bool{legacyKey: true}

	require.Equal(t, legacyKey, svc.FindCyberSessionBlockedForRequest(ctx, 7, c, body, "", ""))
	require.NotEqual(t, legacyKey, CyberSessionExplicitBlockKey(7, c, body), "new writes must use typed v3")
}

func TestCyberSessionIsolationAcrossSharedKeyAndLongHistory(t *testing.T) {
	ctx := context.Background()
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
		SettingKeyCyberSessionBlockEnabled: "true", SettingKeyCyberSessionBlockTTLSeconds: "60",
	}}}}
	// Same input, cache hint and source cannot merge distinct session IDs.
	items := make([]map[string]string, 600)
	for i := range items {
		items[i] = map[string]string{"type": "function_call_output", "call_id": "call-" + strconv.Itoa(i), "output": "ordinary tool result"}
	}
	payload, err := json.Marshal(map[string]any{"input": items, "prompt_cache_key": "shared-cache"})
	require.NoError(t, err)
	a, _ := newCyberBlockTestCtx(map[string]string{"session_id": "conversation-a"}, string(payload))
	aKey := CyberSessionExplicitBlockKey(7, a, payload)
	legacyScope := sha256.Sum256([]byte("cyber-scope:v1|api_key=7|ip=203.0.113.20|ua=" + NormalizeSessionUserAgent("Codex CLI 1.2.3")))
	svc.MarkCyberSessionBlocked(ctx, CyberSessionBlock{APIKeyID: 7, ScopeKey: hex.EncodeToString(legacyScope[:]), Keys: []string{aKey}})

	for _, tc := range []struct {
		name, session string
		apiKey        int64
		blocked       bool
	}{
		{"original session remains blocked", "conversation-a", 7, true},
		{"another session shares key and history", "conversation-b", 7, false},
		{"another authenticated key reuses session name", "conversation-a", 8, false},
		{"no explicit identity with shared cache", "", 7, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newCyberBlockTestCtx(map[string]string{"session_id": tc.session}, string(payload))
			got := svc.FindCyberSessionBlockedForRequest(ctx, tc.apiKey, c, payload, "203.0.113.20", "Codex CLI 1.2.3")
			if tc.blocked {
				require.Equal(t, aKey, got)
			} else {
				require.Empty(t, got)
			}
		})
	}
	// A reconnect/model change and a different network must not evade an exact block.
	require.Equal(t, aKey, svc.FindCyberSessionBlockedForRequest(ctx, 7, a, []byte(`{"model":"another-model","input":"next turn"}`), "203.0.113.99", "AnotherClient/2"))
	require.Empty(t, combo.store.lengths[7], "storing an explicit block must not seed a transcript block")
}

func TestCyberSessionIgnoresLegacyCacheBlocks(t *testing.T) {
	ctx := context.Background()
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{SettingKeyCyberSessionBlockEnabled: "true"}}}}
	// Before v2, a prompt_cache_key used the same digest as a session header.
	legacy := sha256.Sum256([]byte(isolateOpenAISessionID(7, "shared-value")))
	svc.MarkCyberSessionBlocked(ctx, CyberSessionBlock{Keys: []string{hex.EncodeToString(legacy[:])}})
	c, body := newCyberBlockTestCtx(map[string]string{"session_id": "shared-value"}, `{}`)
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 7, c, body, "", ""))
}

func TestCyberSessionLookupFailOpen(t *testing.T) {
	c, body := newCyberBlockTestCtx(map[string]string{"session_id": "session-a"}, `{}`)
	for _, enabled := range []string{"false", "true"} {
		t.Run(enabled, func(t *testing.T) {
			combo := &comboCacheAndStore{store: fakeCyberBlockStore{findErr: errors.New("storage unavailable")}}
			svc := &OpenAIGatewayService{cache: combo, settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{SettingKeyCyberSessionBlockEnabled: enabled}}}}
			require.Empty(t, svc.FindCyberSessionBlockedForRequest(context.Background(), 7, c, body, "", ""))
			if enabled == "false" {
				require.Zero(t, combo.store.findCalls)
			} else {
				require.Equal(t, 1, combo.store.findCalls)
			}
		})
	}
}

func TestCyberRequestFingerprintFallbackBlocksIdenticalRetry(t *testing.T) {
	svc := &OpenAIGatewayService{
		cache: &comboCacheAndStore{},
		settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
			SettingKeyCyberSessionBlockEnabled:    "true",
			SettingKeyCyberSessionBlockTTLSeconds: "60",
		}}},
	}
	ctx := context.Background()
	bodyA := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"synthetic same content"}],"prompt_cache_key":"cache-a","stream":true}`)
	bodyB := []byte(`{"stream":false,"prompt_cache_key":"cache-b","messages":[{"role":"user","content":"synthetic same content"}],"model":"another-model"}`)
	c, _ := newCyberBlockTestCtx(nil, string(bodyA))
	identity := ResolveCyberSessionIdentity(7, c, bodyA)
	require.False(t, identity.Resolved(), "fixture must not have an explicit session ID")
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 7, c, bodyA, "", ""))

	svc.MarkCyberSessionBlockedForRequest(ctx, 7, identity, bodyA)
	require.NotEmpty(t, svc.FindCyberSessionBlockedForRequest(ctx, 7, c, bodyB, "", ""), "identical content must match across cache/stream/model changes")
	retryWithIdentity, _ := newCyberBlockTestCtx(map[string]string{"session_id": "retry-session"}, string(bodyA))
	require.NotEmpty(t, svc.FindCyberSessionBlockedForRequest(ctx, 7, retryWithIdentity, bodyA, "", ""), "a retry that adds a session ID must still hit the fallback block")
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 7, c, []byte(`{"messages":[{"role":"user","content":"different content"}]}`), "", ""))
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 8, c, bodyA, "", ""), "fingerprint is API-key scoped")
}

func TestCyberRequestFingerprintFallbackDoesNotReplaceExplicitSession(t *testing.T) {
	svc := &OpenAIGatewayService{
		cache: &comboCacheAndStore{},
		settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
			SettingKeyCyberSessionBlockEnabled: "true",
		}}},
	}
	ctx := context.Background()
	body := []byte(`{"messages":[{"role":"user","content":"synthetic explicit content"}]}`)
	explicitCtx, _ := newCyberBlockTestCtx(map[string]string{"session_id": "session-explicit"}, string(body))
	identity := ResolveCyberSessionIdentity(9, explicitCtx, body)
	require.True(t, identity.Resolved())
	svc.MarkCyberSessionBlockedForRequest(ctx, 9, identity, body)

	require.Equal(t, identity.BlockKey, svc.FindCyberSessionBlockedForRequest(ctx, 9, explicitCtx, body, "", ""))
	noIdentityCtx, _ := newCyberBlockTestCtx(nil, string(body))
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 9, noIdentityCtx, body, "", ""), "explicit block must not silently become a content-wide block")
}

func TestCyberRequestFingerprintFallbackDisabledWithSessionBlock(t *testing.T) {
	svc := &OpenAIGatewayService{
		cache:          &comboCacheAndStore{},
		settingService: &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{SettingKeyCyberSessionBlockEnabled: "false"}}},
	}
	ctx := context.Background()
	body := []byte(`{"messages":[{"role":"user","content":"synthetic disabled content"}]}`)
	c, _ := newCyberBlockTestCtx(nil, string(body))
	identity := ResolveCyberSessionIdentity(10, c, body)
	svc.MarkCyberSessionBlockedForRequest(ctx, 10, identity, body)
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 10, c, body, "", ""))
}

func TestCyberRequestFingerprintKeyNormalizesTransportFields(t *testing.T) {
	a := []byte(`{"messages":[{"role":"user","content":"same"}],"model":"gpt-5","stream":true,"prompt_cache_key":"a"}`)
	b := []byte(`{"prompt_cache_key":"b","stream":false,"model":"other","messages":[{"role":"user","content":"same"}]}`)
	require.NotEmpty(t, CyberRequestFingerprintKey(1, a))
	require.Equal(t, CyberRequestFingerprintKey(1, a), CyberRequestFingerprintKey(1, b))
	require.NotEqual(t, CyberRequestFingerprintKey(1, a), CyberRequestFingerprintKey(2, a))
	require.Empty(t, CyberRequestFingerprintKey(1, []byte(`{"model":"gpt-5"}`)))
}
