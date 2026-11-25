package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/tidwall/gjson"
)

// openAIModelSupportsUltrafast reports whether OpenAI offers the Ultrafast
// service tier for the model. Both model-scoped group controls and default
// billing multipliers use this gate; explicit tier overrides remain independent.
func openAIModelSupportsUltrafast(model string) bool {
	normalized := normalizeKnownOpenAICodexModel(model)
	return isOpenAIGPT6AstraModel(normalized) || normalized == "gpt-6.1-sol" || normalized == "gpt-6.2-sol" || normalized == "gpt-6.2-astra"
}

// openAIGroupForcedServiceTier returns the service_tier that the trusted request
// Group forces onto an OpenAI-account request for the given upstream model, or
// "" when the group forces nothing and the client's value is kept.
//
// An explicit ForceOpenAIServiceTier retains the custom tier override. Otherwise
// the two group flags are independent and resolve as Ultrafast > Fast > client:
//   - ForceOpenAIUltrafast applies only to models that support Ultrafast
//     (openAIModelSupportsUltrafast) and yields "ultrafast".
//   - Otherwise ForceOpenAIFast yields "priority", for every model.
//
// So a group with only ForceOpenAIUltrafast leaves unsupported models to the
// client, and a group with both flags falls back to Fast for them.
func openAIGroupForcedServiceTier(ctx context.Context, account *Account, model string) string {
	if ctx == nil || account == nil || account.Platform != PlatformOpenAI {
		return ""
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	if !IsGroupContextValid(group) || !groupSupportsOpenAIFast(group.Platform) {
		return ""
	}
	switch group.ForceOpenAIServiceTier {
	case OpenAIFastTierPriority, OpenAIFastTierUltrafast:
		return group.ForceOpenAIServiceTier
	case "":
		// The upstream boolean controls are used only without an explicit tier.
	default:
		return ""
	}
	if group.ForceOpenAIUltrafast && openAIModelSupportsUltrafast(model) {
		// Resolve the published Pro entitlement first, then account-specific grants.
		if base := openai.GPT62ModelID(model); base != "" && account.IsOpenAIOAuthLike() {
			if gpt62PlanIncludesUltrafast(account, base) {
				return OpenAIFastTierUltrafast
			}
			metadata, ok := account.GetUpstreamModelMetadata(base)
			if ok && gjson.GetBytes(metadata.CodexToolCapabilities["service_tiers"], `#(id=="ultrafast")`).Exists() {
				return OpenAIFastTierUltrafast
			}
		} else {
			return OpenAIFastTierUltrafast
		}
	}
	if group.ForceOpenAIFast {
		return OpenAIFastTierPriority
	}
	return ""
}
