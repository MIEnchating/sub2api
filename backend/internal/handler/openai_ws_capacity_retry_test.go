package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSCapacityRetryWindowDoesNotRestartAcrossFailures(t *testing.T) {
	start := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := &openAIWSCapacityRetryWindow{}
	require.True(t, window.allow(start, 500*time.Millisecond))
	require.True(t, window.allow(start.Add(20*time.Second), 8*time.Second))
	require.False(t, window.allow(start.Add(29*time.Second), time.Second))
	require.False(t, window.allow(start.Add(30*time.Second), 0))
}

func TestOpenAIWSCapacityRetryWindowCanResetForTheNextTurn(t *testing.T) {
	start := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := &openAIWSCapacityRetryWindow{}
	require.True(t, window.allow(start, 0))
	window.deadline = time.Time{}
	require.True(t, window.allow(start.Add(time.Hour), 500*time.Millisecond))
}

func TestOpenAIWSCapacityRetryWindowChecksTheActualAttemptStart(t *testing.T) {
	start := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := &openAIWSCapacityRetryWindow{}
	require.False(t, window.expired(start), "the initial attempt has no capacity retry budget")
	require.True(t, window.allow(start, 0))
	require.False(t, window.expired(start.Add(29*time.Second)))
	require.True(t, window.expired(start.Add(30*time.Second)), "slot acquisition cannot extend the retry window")
	window.deadline = time.Time{}
	require.False(t, window.expired(start.Add(time.Hour)), "a completed turn releases its retry window")
}
