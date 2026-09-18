package service

import (
	"context"
	"testing"
)

type accountProtectionContextTestKey struct{}

func TestAccountProtectionOutcomeExcludedContext(t *testing.T) {
	if AccountProtectionOutcomeExcluded(context.Background()) {
		t.Fatal("plain contexts must be included in outcome accounting")
	}
	ctx := WithAccountProtectionOutcomeExcluded(context.Background())
	if !AccountProtectionOutcomeExcluded(ctx) {
		t.Fatal("marked context must be excluded from outcome accounting")
	}
	if !AccountProtectionOutcomeExcluded(context.WithValue(ctx, accountProtectionContextTestKey{}, "child")) {
		t.Fatal("marker must survive derived contexts")
	}
}
