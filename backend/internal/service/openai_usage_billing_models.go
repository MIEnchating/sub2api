package service

import "strings"

func openAIUsageBillingModels(result *OpenAIForwardResult, fields ChannelUsageFields) (requested, primary string, candidates []string) {
	requested = fields.OriginalModel
	if fields.APIKeyMappedModel != "" {
		requested = fields.APIKeyMappedModel
	}
	channelModel := fields.ChannelMappedModel
	if !fields.ChannelMappingApplied && channelModel == fields.OriginalModel {
		// An unchanged channel field is an audit alias, not a price override.
		channelModel = requested
	}
	resultModel := result.Model
	if !fields.ChannelMappingApplied && result.Model == fields.OriginalModel {
		resultModel = requested
	}
	primary = forwardResultBillingModel(resultModel, result.UpstreamModel)
	if result.BillingModel != "" {
		primary = strings.TrimSpace(result.BillingModel)
	}
	if fields.BillingModelSource == BillingModelSourceChannelMapped && fields.ChannelMappedModel != "" &&
		((fields.APIKeyMappedModel != "" && fields.ChannelMappingApplied) || fields.ChannelMappedModel != fields.OriginalModel) {
		primary = fields.ChannelMappedModel
	}
	if fields.BillingModelSource == BillingModelSourceRequested && fields.OriginalModel != "" {
		primary = requested
	}
	return requested, primary, usageBillingModelCandidates(primary, result.BillingModel, channelModel, requested, result.UpstreamModel, resultModel)
}
