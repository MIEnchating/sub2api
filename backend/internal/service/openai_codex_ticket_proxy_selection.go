package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// Bindings only drain in-flight attempts when an account's configured exit is
// changed. The scheduler never chooses an alternate proxy after a failure.
type codexTicketProxyBinding struct {
	source string
	proxy  string
}

type codexTicketProxyRoute struct {
	source string
	proxy  string
	valid  bool
}

type codexTicketProxySelection struct {
	source string
	proxy  string
}

func (s *OpenAIGatewayService) openAICodexTicketHarvestConfig(ctx context.Context) config.OpenAICodexTicketConfig {
	cfg := s.openAICodexTicketConfig()
	if s != nil && s.settingService != nil {
		if proxy := s.settingService.GetOpenAICodexTicketHarvestProxyURL(ctx); proxy != "" {
			cfg.HarvestProxyURL = proxy
		}
	}
	return cfg
}

func openAICodexTicketProxyRoute(account *Account, harvestProxy ...string) codexTicketProxyRoute {
	if account == nil {
		return codexTicketProxyRoute{source: "account"}
	}
	if len(harvestProxy) > 0 {
		if proxy := strings.TrimSpace(harvestProxy[0]); proxy != "" {
			// An explicitly configured dedicated exit wins over account routing.
			// Invalid settings fail closed instead of leaking onto another exit.
			return codexTicketProxyRoute{source: "pool", proxy: proxy, valid: ValidateOpenAICodexTicketHarvestProxyURL(proxy) == nil}
		}
	}
	if account.ProxyID == nil {
		// An unbound account uses the same direct route as normal business traffic.
		return codexTicketProxyRoute{source: "direct", valid: true}
	}
	// Missing/invalid loaded proxies must never silently change the account exit.
	if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
		return codexTicketProxyRoute{source: "account"}
	}
	proxy := account.Proxy.URL()
	if proxy == "" || ValidateOpenAICodexTicketHarvestProxyURL(proxy) != nil {
		return codexTicketProxyRoute{source: "account"}
	}
	return codexTicketProxyRoute{source: "account", proxy: proxy, valid: true}
}

// Caller holds r.mu. Account proxy edits wait for older requests to drain;
// capacity waits and connection failures keep the configured business exit.
func (r *codexTicketScheduler) acquireProxy(job *codexTicketJob, route codexTicketProxyRoute, limit int, now time.Time) (codexTicketProxySelection, time.Time, bool) {
	if !route.valid {
		return codexTicketProxySelection{}, now.Add(time.Second), false
	}
	binding := r.bindings[job.accountID]
	if binding == nil || binding.source != route.source || binding.proxy != route.proxy {
		if r.accountActive[job.accountID] > 0 {
			return codexTicketProxySelection{}, now.Add(time.Second), false
		}
		r.bindings[job.accountID] = &codexTicketProxyBinding{source: route.source, proxy: route.proxy}
	}
	health := r.proxies[route.proxy]
	if health == nil {
		health = &codexTicketProxyHealth{}
		r.proxies[route.proxy] = health
	}
	availableAt := health.cooldown
	if health.active >= limit && availableAt.Before(now.Add(time.Second)) {
		availableAt = now.Add(time.Second)
	}
	if availableAt.After(now) {
		return codexTicketProxySelection{}, availableAt, false
	}
	health.active++
	return codexTicketProxySelection{source: route.source, proxy: route.proxy}, time.Time{}, true
}
