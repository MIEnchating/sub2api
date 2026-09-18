package handler

import "time"

const openAIWSCapacityRetryWindowDuration = 30 * time.Second

type openAIWSCapacityRetryWindow struct {
	deadline time.Time
}

// Bound when another attempt may start, while allowing a successful generation
// to finish under the upstream's normal read timeout.
func (w *openAIWSCapacityRetryWindow) allow(now time.Time, delay time.Duration) bool {
	if w.deadline.IsZero() {
		w.deadline = now.Add(openAIWSCapacityRetryWindowDuration)
	}
	return now.Add(delay).Before(w.deadline)
}

func (w *openAIWSCapacityRetryWindow) expired(now time.Time) bool {
	return !w.deadline.IsZero() && !now.Before(w.deadline)
}
