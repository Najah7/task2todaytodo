package shared

import (
	"context"
	"time"
)

type PlanAccess string

const (
	PlanAccessFree PlanAccess = "free"
	PlanAccessPro  PlanAccess = "pro"
	PlanAccessTeam PlanAccess = "team"
)

type PlanAccessReader interface {
	GetPlanAccess(context.Context, string) (PlanAccess, error)
}

// EffectivePlanAccess returns the paid feature tier only while Stripe's
// subscription snapshot is active and inside its current paid period.
func EffectivePlanAccess(plan, status string, currentPeriodEnd *time.Time, now time.Time) PlanAccess {
	if status != "active" || currentPeriodEnd == nil || !currentPeriodEnd.After(now) {
		return PlanAccessFree
	}
	switch plan {
	case string(PlanAccessPro):
		return PlanAccessPro
	case string(PlanAccessTeam):
		return PlanAccessTeam
	default:
		return PlanAccessFree
	}
}

func (access PlanAccess) IsPaid() bool {
	return access == PlanAccessPro || access == PlanAccessTeam
}

func (access PlanAccess) IsTeam() bool { return access == PlanAccessTeam }
