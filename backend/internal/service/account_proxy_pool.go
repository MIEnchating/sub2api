package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// AccountProxyPoolExtraKey stores the additional egress proxy IDs in Account.Extra.
	// ProxyID remains the primary proxy for OAuth, probes, tests, and compatibility.
	AccountProxyPoolExtraKey = "proxy_pool_ids"
	MaxAccountProxyPoolSize  = 20

	AccountProxyEgressModeExtraKey       = "proxy_egress_mode"
	AccountProxyStickyTTLSecondsExtraKey = "proxy_sticky_ttl_seconds"
	AccountProxyEgressModeSessionSticky  = "session_sticky"
	AccountProxyEgressModeRoundRobin     = "round_robin"
	AccountProxyEgressModePrimary        = "primary"
	DefaultAccountProxyStickyTTLSeconds  = 2 * 60 * 60
	MinAccountProxyStickyTTLSeconds      = 60
	MaxAccountProxyStickyTTLSeconds      = 7 * 24 * 60 * 60
)

var (
	accountEgressSequence  atomic.Uint64
	accountEgressSequences sync.Map // map[accountID]*atomic.Uint64
)

type accountEgressSessionContextKey struct{}

type accountEgressCache interface {
	GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error)
	SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error
	RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error
	DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error
}

func withAccountEgressSessionHash(ctx context.Context, sessionHash string) context.Context {
	if ctx == nil || strings.TrimSpace(sessionHash) == "" {
		return ctx
	}
	return context.WithValue(ctx, accountEgressSessionContextKey{}, strings.TrimSpace(sessionHash))
}

func accountEgressSessionHash(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(accountEgressSessionContextKey{}).(string)
	return strings.TrimSpace(value)
}

func AccountProxyEgressMode(extra map[string]any) string {
	if extra != nil {
		if mode, ok := extra[AccountProxyEgressModeExtraKey].(string); ok {
			switch strings.ToLower(strings.TrimSpace(mode)) {
			case AccountProxyEgressModeRoundRobin:
				return AccountProxyEgressModeRoundRobin
			case AccountProxyEgressModePrimary:
				return AccountProxyEgressModePrimary
			case AccountProxyEgressModeSessionSticky:
				return AccountProxyEgressModeSessionSticky
			}
		}
	}
	return AccountProxyEgressModeSessionSticky
}

func AccountProxyStickyTTL(extra map[string]any) time.Duration {
	seconds := int64(DefaultAccountProxyStickyTTLSeconds)
	if extra != nil {
		if value, ok := accountProxyPoolID(extra[AccountProxyStickyTTLSecondsExtraKey]); ok {
			seconds = value
		}
	}
	if seconds < MinAccountProxyStickyTTLSeconds {
		seconds = MinAccountProxyStickyTTLSeconds
	}
	if seconds > MaxAccountProxyStickyTTLSeconds {
		seconds = MaxAccountProxyStickyTTLSeconds
	}
	return time.Duration(seconds) * time.Second
}

// ParseAccountProxyPoolIDs accepts both decoded JSON arrays and native Go slices.
// Invalid values are ignored so manually edited legacy data cannot break scheduling.
func ParseAccountProxyPoolIDs(extra map[string]any) []int64 {
	if extra == nil {
		return nil
	}
	raw, ok := extra[AccountProxyPoolExtraKey]
	if !ok || raw == nil {
		return nil
	}

	var values []any
	switch typed := raw.(type) {
	case []any:
		values = typed
	case []int64:
		out := make([]int64, len(typed))
		copy(out, typed)
		return normalizeAccountProxyPoolIDs(out, nil)
	case []int:
		values = make([]any, len(typed))
		for i, value := range typed {
			values[i] = value
		}
	default:
		return nil
	}

	ids := make([]int64, 0, len(values))
	for _, value := range values {
		if id, ok := accountProxyPoolID(value); ok {
			ids = append(ids, id)
		}
	}
	return normalizeAccountProxyPoolIDs(ids, nil)
}

func accountProxyPoolID(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), typed > 0
	case int32:
		return int64(typed), typed > 0
	case int64:
		return typed, typed > 0
	case uint:
		if uint64(typed) > math.MaxInt64 || typed == 0 {
			return 0, false
		}
		return int64(typed), true
	case uint64:
		if typed > math.MaxInt64 || typed == 0 {
			return 0, false
		}
		return int64(typed), true
	case float64:
		if typed <= 0 || typed > math.MaxInt64 || math.Trunc(typed) != typed {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		id, err := strconv.ParseInt(string(typed), 10, 64)
		return id, err == nil && id > 0
	default:
		return 0, false
	}
}

func normalizeAccountProxyPoolIDs(ids []int64, primaryProxyID *int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || (primaryProxyID != nil && id == *primaryProxyID) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func configuredAccountProxyPoolIDs(account *Account) []int64 {
	if account == nil {
		return nil
	}
	ids := account.ProxyIDs
	if len(ids) == 0 {
		ids = ParseAccountProxyPoolIDs(account.Extra)
	}
	return normalizeAccountProxyPoolIDs(ids, account.ConfiguredProxyID())
}

// ConfiguredProxyID returns the persisted primary proxy even after a
// request-local account copy has selected an additional egress proxy.
func (a *Account) ConfiguredProxyID() *int64 {
	if a == nil {
		return nil
	}
	if a.egressPrimaryProxyID != nil {
		return a.egressPrimaryProxyID
	}
	return a.ProxyID
}

func setAccountProxyPoolIDs(account *Account, ids []int64) {
	if account == nil {
		return
	}
	ids = normalizeAccountProxyPoolIDs(ids, account.ProxyID)
	account.ProxyIDs = append([]int64(nil), ids...)
	account.ProxyPool = nil
	if len(ids) == 0 {
		if account.Extra != nil {
			delete(account.Extra, AccountProxyPoolExtraKey)
		}
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra[AccountProxyPoolExtraKey] = append([]int64(nil), ids...)
}

func validateAccountProxyPool(primaryProxyID *int64, ids []int64) ([]int64, error) {
	ids = normalizeAccountProxyPoolIDs(ids, primaryProxyID)
	if len(ids) == 0 {
		return nil, nil
	}
	if primaryProxyID == nil || *primaryProxyID <= 0 {
		return nil, infraerrors.BadRequest("ACCOUNT_PROXY_POOL_PRIMARY_REQUIRED", "a primary proxy is required when additional egress proxies are configured")
	}
	if len(ids) > MaxAccountProxyPoolSize {
		return nil, infraerrors.BadRequest("ACCOUNT_PROXY_POOL_TOO_LARGE", "an account can have at most 20 additional egress proxies")
	}
	return ids, nil
}

func (s *adminServiceImpl) validateAccountProxyPoolProxies(ctx context.Context, primaryProxyID *int64, ids []int64) ([]int64, error) {
	normalized, err := validateAccountProxyPool(primaryProxyID, ids)
	if err != nil || len(normalized) == 0 {
		return normalized, err
	}
	if s.proxyRepo == nil {
		return nil, infraerrors.BadRequest("ACCOUNT_PROXY_POOL_UNAVAILABLE", "proxy repository is unavailable")
	}
	proxies, err := s.proxyRepo.ListByIDs(ctx, normalized)
	if err != nil {
		return nil, err
	}
	found := make(map[int64]struct{}, len(proxies))
	for i := range proxies {
		found[proxies[i].ID] = struct{}{}
	}
	for _, id := range normalized {
		if _, ok := found[id]; !ok {
			return nil, infraerrors.BadRequest("ACCOUNT_PROXY_POOL_PROXY_NOT_FOUND", "one or more additional egress proxies were not found")
		}
	}
	return normalized, nil
}

// selectAccountEgressProxy returns a request-local shallow copy. OAuth, probes,
// tests, and quota checks do not call this function and therefore keep using
// the configured primary proxy.
func selectAccountEgressProxy(ctx context.Context, cache accountEgressCache, account *Account) *Account {
	if account == nil || account.egressProxySelected || account.Proxy == nil || len(account.ProxyPool) == 0 {
		return account
	}

	candidates := make([]*Proxy, 0, len(account.ProxyPool)+1)
	candidates = append(candidates, account.Proxy)
	now := time.Now()
	seen := map[int64]struct{}{account.Proxy.ID: {}}
	for _, proxy := range account.ProxyPool {
		if proxy == nil || !proxy.IsActive() || proxy.IsExpired(now) {
			continue
		}
		if _, ok := seen[proxy.ID]; ok {
			continue
		}
		seen[proxy.ID] = struct{}{}
		candidates = append(candidates, proxy)
	}

	selected := candidates[0]
	mode := AccountProxyEgressMode(account.Extra)
	if mode != AccountProxyEgressModePrimary {
		sessionHash := accountEgressSessionHash(ctx)
		if mode == AccountProxyEgressModeSessionSticky && sessionHash != "" {
			selected = selectStickyAccountEgressProxy(ctx, cache, account, sessionHash, candidates)
		} else {
			selected = selectRoundRobinAccountEgressProxy(account.ID, candidates)
		}
	}

	cloned := *account
	cloned.egressPrimaryProxyID = account.ConfiguredProxyID()
	selectedID := selected.ID
	cloned.ProxyID = &selectedID
	cloned.Proxy = selected
	cloned.egressProxySelected = true
	return &cloned
}

func selectRoundRobinAccountEgressProxy(accountID int64, candidates []*Proxy) *Proxy {
	sequence := &accountEgressSequence
	if accountID > 0 {
		loaded, _ := accountEgressSequences.LoadOrStore(accountID, &atomic.Uint64{})
		if accountSequence, ok := loaded.(*atomic.Uint64); ok {
			sequence = accountSequence
		}
	}
	return candidates[(sequence.Add(1)-1)%uint64(len(candidates))]
}

func selectStickyAccountEgressProxy(ctx context.Context, cache accountEgressCache, account *Account, sessionHash string, candidates []*Proxy) *Proxy {
	cacheKey := accountProxyStickyCacheKey(sessionHash)
	ttl := AccountProxyStickyTTL(account.Extra)
	if cache != nil && account.ID > 0 {
		proxyID, err := cache.GetSessionAccountID(ctx, account.ID, cacheKey)
		if err == nil {
			for _, candidate := range candidates {
				if candidate.ID == proxyID {
					_ = cache.RefreshSessionTTL(ctx, account.ID, cacheKey, ttl)
					return candidate
				}
			}
			_ = cache.DeleteSessionAccountID(ctx, account.ID, cacheKey)
		} else if !errors.Is(err, ErrStickySessionNotFound) {
			return selectRoundRobinAccountEgressProxy(account.ID, candidates)
		}
	}

	// Deterministic selection prevents two first requests for the same session
	// from choosing different proxies on separate application instances.
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", account.ID, sessionHash)))
	index := uint64(digest[0])<<56 | uint64(digest[1])<<48 | uint64(digest[2])<<40 | uint64(digest[3])<<32 |
		uint64(digest[4])<<24 | uint64(digest[5])<<16 | uint64(digest[6])<<8 | uint64(digest[7])
	selected := candidates[index%uint64(len(candidates))]
	if cache != nil && account.ID > 0 {
		_ = cache.SetSessionAccountID(ctx, account.ID, cacheKey, selected.ID, ttl)
	}
	return selected
}

func accountProxyStickyCacheKey(sessionHash string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sessionHash)))
	return "egress_proxy:v1:" + fmt.Sprintf("%x", digest[:])
}
