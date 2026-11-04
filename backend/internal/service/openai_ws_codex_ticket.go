package service

import (
	"context"
	"net/http"
)

// The model belongs to the outgoing response.create payload. Keep this factory
// independent of Agent Identity's per-dial authorization factory: selecting an
// existing socket must also check ticket freshness without minting credentials.
func (s *OpenAIGatewayService) openAIWSCodexTicketHeadersFactory(account *Account, model string) func(context.Context, http.Header) (http.Header, bool, error) {
	return func(ctx context.Context, headers http.Header) (http.Header, bool, error) {
		policy := s.openAICodexTicketAccountConfig(ctx, account)
		if !policy.Enabled || !s.openAICodexTicketGatedModel(model) {
			return headers, false, nil
		}
		if headers == nil {
			headers = make(http.Header)
		}
		managed, err := s.applyOpenAICodexTicketWithResult(ctx, account, model, headers, "websocket")
		if err != nil {
			return nil, false, err
		}
		// Pool selection and the actual dial must compare the same harvest
		// identity and cookies, including the removal of expired bundles.
		s.restoreBoundCodexTicketHarvestIdentity(ctx, headers, account)
		return headers, managed, nil
	}
}
