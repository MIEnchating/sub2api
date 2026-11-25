package service

import (
	"log/slog"
	"strings"
)

// OpenAIResponseTierBillingEnabledKey opts a compatible API key into billing
// the actual response tier, including upgrades. It defaults to disabled.
const OpenAIResponseTierBillingEnabledKey = "openai_response_tier_billing_enabled"

// OpenAIOAuthRelayFastBillingKey opts a trusted API-key relay of Codex OAuth
// into retaining outbound Fast billing when it echoes default. Never infer this
// contract from the provider URL or account name. Other tiers keep their rules.
const OpenAIOAuthRelayFastBillingKey = "openai_oauth_relay_fast_billing"

// ServiceTierBillingResolution describes how the billable service tier of one
// request was settled between the tier the client asked for and the tier the
// upstream reports having used.
type ServiceTierBillingResolution struct {
	Requested       string // tier carried by the request sent upstream ("" when none)
	Observed        string // tier declared by the upstream response ("" when none)
	Billing         string // tier used for billing and the usage log
	Downgraded      bool   // Billing is cheaper than Requested
	Upgraded        bool   // explicitly enabled response billing raised the tier
	ResponseBilling bool   // selected API key opts into actual response-tier billing
	OAuthRelayFast  bool   // trusted OAuth relay's default marker retained outbound Fast
}

// ResolveBillingServiceTier always bills the final outbound request tier.
// Upstream responses cannot lower or raise it, including when the request had
// no tier after request filtering or other policies had run.
func ResolveBillingServiceTier(requested, observed string) ServiceTierBillingResolution {
	requested = normalizeBillingServiceTier(requested)
	return ServiceTierBillingResolution{
		Requested: requested,
		Observed:  normalizeBillingServiceTier(observed),
		Billing:   requested,
	}
}

// serviceTierCostRank orders tiers by their cost relative to the base rate, so a
// lower rank is always cheaper. Unknown tiers rank as the base rate and report
// known=false so callers can refuse to act on them.
func serviceTierCostRank(tier string) (rank int, known bool) {
	switch normalizeBillingServiceTier(tier) {
	case "flex":
		return 0, true
	case "", "default", "standard", "auto", "scale":
		return 1, true
	case "priority", "fast":
		return 2, true
	case "ultrafast":
		return 3, true
	default:
		return 1, false
	}
}

// ResolveOpenAIServiceTierBilling uses the outbound tier by default. An
// administrator can opt a specific compatible API key into response pricing.
func ResolveOpenAIServiceTierBilling(account *Account, requested, observed string) ServiceTierBillingResolution {
	// A fixed-tier compatible provider can serve a more expensive tier than
	// the client requested. This contract must be enabled explicitly for that
	// credential; other accounts retain outbound-tier pricing.
	if account != nil && account.IsOpenAIApiKey() && account.Extra[OpenAIResponseTierBillingEnabledKey] == true {
		resolution := ServiceTierBillingResolution{
			Requested:       normalizeBillingServiceTier(requested),
			Observed:        normalizeObservedOpenAIServiceTier(observed),
			Billing:         normalizeBillingServiceTier(requested),
			ResponseBilling: true,
		}
		if resolution.Observed != "" {
			resolution.Billing = resolution.Observed
			requestedRank, _ := serviceTierCostRank(resolution.Requested)
			observedRank, _ := serviceTierCostRank(resolution.Observed)
			resolution.Downgraded = observedRank < requestedRank
			resolution.Upgraded = observedRank > requestedRank
		}
		return resolution
	}
	if account != nil && account.IsOpenAIApiKey() && account.Extra[OpenAIOAuthRelayFastBillingKey] == true &&
		codexOAuthResponseTierIsNonAuthoritative(observed) {
		tier := normalizeBillingServiceTier(requested)
		if tier == "priority" || tier == "fast" {
			return ServiceTierBillingResolution{
				Requested: tier, Observed: normalizeBillingServiceTier(observed),
				Billing: tier, OAuthRelayFast: true,
			}
		}
	}
	return ResolveBillingServiceTier(requested, observed)
}

func codexOAuthResponseTierIsNonAuthoritative(observed string) bool {
	switch normalizeBillingServiceTier(observed) {
	case "default":
		return true
	default:
		return false
	}
}

// ApplyOpenAIServiceTierBillingResolution settles the billing tier. Upward
// changes require the selected API key's explicit response-billing opt-in.
func ApplyOpenAIServiceTierBillingResolution(account *Account, result *OpenAIForwardResult) ServiceTierBillingResolution {
	if result == nil {
		return ServiceTierBillingResolution{}
	}
	resolution := ResolveOpenAIServiceTierBilling(account, optionalStringValue(result.ServiceTier), result.UpstreamResponseServiceTier)
	if resolution.Billing != resolution.Requested {
		billing := resolution.Billing
		result.ServiceTier = &billing
	}
	return resolution
}

// ApplyForwardServiceTierBillingResolution is the ForwardResult counterpart of
// ApplyOpenAIServiceTierBillingResolution.
func ApplyForwardServiceTierBillingResolution(result *ForwardResult) ServiceTierBillingResolution {
	if result == nil {
		return ServiceTierBillingResolution{}
	}
	return ResolveBillingServiceTier(optionalStringValue(result.ServiceTier), result.UpstreamResponseServiceTier)
}

// logServiceTierBillingDowngrade retains the existing downgrade audit and logs
// all explicitly enabled response-tier decisions, including missing markers.
func logServiceTierBillingDowngrade(component string, account *Account, requestID string, resolution ServiceTierBillingResolution) {
	if !resolution.Downgraded && !resolution.ResponseBilling && !resolution.OAuthRelayFast {
		return
	}
	attrs := []any{
		"component", component,
		"request_id", strings.TrimSpace(requestID),
		"requested_tier", resolution.Requested,
		"response_tier", resolution.Observed,
		"billed_tier", resolution.Billing,
	}
	if account != nil {
		attrs = append(attrs, "platform", account.Platform, "account_id", account.ID)
	}
	if resolution.OAuthRelayFast {
		attrs = append(attrs, "tier_source", "trusted_oauth_relay_request")
		slog.Info("billing.service_tier_resolved", attrs...)
		return
	}
	if resolution.ResponseBilling {
		attrs = append(attrs, "tier_source", "upstream_response", "upgraded", resolution.Upgraded)
		if resolution.Observed == "" {
			attrs = append(attrs, "tier_source_fallback", "outbound_request")
			slog.Warn("billing.service_tier_response_missing", attrs...)
			return
		}
		slog.Info("billing.service_tier_resolved", attrs...)
		return
	}
	slog.Info("billing.service_tier_downgraded", attrs...)
}
