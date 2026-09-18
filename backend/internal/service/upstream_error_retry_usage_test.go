package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamErrorRetryHasUsageRecognizesPartialBillingEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "cached input without total", body: `{"response":{"usage":{"input_tokens_details":{"cached_tokens":2}}}}`},
		{name: "reasoning without total", body: `{"response":{"usage":{"output_tokens_details":{"reasoning_tokens":3}}}}`},
		{name: "image output without total", body: `{"response":{"usage":{"output_tokens_details":{"image_tokens":4}}}}`},
		{name: "audio output without total", body: `{"usage":{"completion_tokens_details":{"audio_tokens":5}}}`},
		{name: "cache write without total", body: `{"usage":{"cache_creation_input_tokens":6}}`},
		{name: "cache read without total", body: `{"usage":{"cache_read_input_tokens":7}}`},
		{name: "top level output", body: `{"output":[{"type":"image_generation_call","result":"image"}]}`},
		{name: "nested output", body: `{"response":{"output":[{"type":"function_call","arguments":""}]}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.True(t, upstreamErrorRetryHasUsage([]byte(test.body)))
		})
	}
}

func TestUpstreamErrorRetryHasUsageIgnoresMissingZeroAndEchoedUsage(t *testing.T) {
	for _, body := range []string{
		`{"response":{"error":{"message":"The service is busy. Please retry later."}}}`,
		`{"usage":{"input_tokens":0,"output_tokens_details":{"image_tokens":0}},"output":[]}`,
		`{"error":{"message":"usage.output_tokens_details.image_tokens=10"}}`,
	} {
		require.False(t, upstreamErrorRetryHasUsage([]byte(body)), body)
	}
}
