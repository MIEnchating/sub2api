package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAstraGatewayWSValidationRejectsUnconfiguredAccount(t *testing.T) {
	gateway, repo, _ := unifiedAstraFixture(t)
	svc := &AccountTestService{accountRepo: repo, cfg: gateway.cfg, openaiGatewayService: gateway}

	result, err := svc.TestAstraGateway(t.Context(), "ws", 2, 1)
	require.ErrorContains(t, err, "ws_account_not_configured")
	require.False(t, result.Success)
	require.Empty(t, gateway.codexWSAnchors.entries)
}

func TestAstraGatewayWSValidationFailureReasons(t *testing.T) {
	require.Equal(t, "ws_group_membership_changed", astraWSTestFailureReason(&OpenAITurnAdmissionError{Reason: "group_membership_changed"}))
	require.Equal(t, "ws_test_timeout", astraWSTestFailureReason(context.DeadlineExceeded))
	require.Equal(t, "ws_test_failed", astraWSTestFailureReason(errors.New("secret upstream data")))
}

func TestAstraGatewayWSValidationText(t *testing.T) {
	stream := `data: {"type":"response.output_text.delta","delta":"3"}` + "\n\n" + `data: {"type":"response.output_text.delta","delta":"23"}` + "\n\n" + `data: {"type":"response.completed","response":{"output":[]}}` + "\n\n"
	require.Equal(t, "323", astraWSValidationText(stream))
	terminal := `data: {"type":"response.completed","response":{"output":[{"content":[{"type":"output_text","text":"323"}]}]}}`
	require.Equal(t, "323", astraWSValidationText(terminal))
	require.Equal(t, "323", astraWSValidationText(stream+terminal))
	require.Empty(t, astraWSValidationText(strings.ReplaceAll(terminal, "323", "")))
}
