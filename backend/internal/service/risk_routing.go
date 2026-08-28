package service

import "context"

const (
	RiskHitActionBlock        = "block"
	RiskHitActionRouteGroup   = "route_group"
	RiskHitActionRouteAccount = "route_account"
)

// RiskRoutingTarget is scoped to one audited request. It changes only upstream
// account selection; billing and API key authorization keep the original group.
type RiskRoutingTarget struct {
	Action    string
	GroupID   int64
	AccountID int64
	Platform  string
}

type riskRoutingTargetContextKey struct{}

func WithRiskRoutingTarget(ctx context.Context, target RiskRoutingTarget) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, riskRoutingTargetContextKey{}, target)
}

func RiskRoutingTargetFromContext(ctx context.Context) (RiskRoutingTarget, bool) {
	if ctx == nil {
		return RiskRoutingTarget{}, false
	}
	target, ok := ctx.Value(riskRoutingTargetContextKey{}).(RiskRoutingTarget)
	if !ok {
		return RiskRoutingTarget{}, false
	}
	switch target.Action {
	case RiskHitActionRouteGroup:
		return target, target.GroupID > 0
	case RiskHitActionRouteAccount:
		return target, target.AccountID > 0
	default:
		return RiskRoutingTarget{}, false
	}
}

func riskRoutingGroupID(ctx context.Context, fallback *int64) *int64 {
	target, ok := RiskRoutingTargetFromContext(ctx)
	if !ok || target.Action != RiskHitActionRouteGroup {
		return fallback
	}
	id := target.GroupID
	return &id
}

func riskRoutingAccountID(ctx context.Context) (int64, bool) {
	target, ok := RiskRoutingTargetFromContext(ctx)
	if !ok || target.Action != RiskHitActionRouteAccount {
		return 0, false
	}
	return target.AccountID, true
}

func riskRoutingPlatform(ctx context.Context, fallback string) string {
	target, ok := RiskRoutingTargetFromContext(ctx)
	if !ok || target.Action != RiskHitActionRouteGroup || target.Platform == "" {
		return fallback
	}
	return target.Platform
}
