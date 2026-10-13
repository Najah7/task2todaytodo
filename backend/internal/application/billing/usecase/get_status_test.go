package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

func TestEffectivePlanRequiresActivePaidPeriod(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	cases := []struct {
		name  string
		state dao.State
		want  shared.PlanAccess
	}{
		{name: "free", state: dao.State{Plan: "free", Status: "active", CurrentPeriodEnd: &future}, want: shared.PlanAccessFree},
		{name: "active Pro", state: dao.State{Plan: "pro", Status: "active", CurrentPeriodEnd: &future}, want: shared.PlanAccessPro},
		{name: "active Team", state: dao.State{Plan: "team", Status: "active", CurrentPeriodEnd: &future}, want: shared.PlanAccessTeam},
		{name: "period missing", state: dao.State{Plan: "team", Status: "active"}, want: shared.PlanAccessFree},
		{name: "period ended", state: dao.State{Plan: "team", Status: "active", CurrentPeriodEnd: &past}, want: shared.PlanAccessFree},
		{name: "past due", state: dao.State{Plan: "pro", Status: "past_due", CurrentPeriodEnd: &future}, want: shared.PlanAccessFree},
		{name: "canceled", state: dao.State{Plan: "team", Status: "canceled", CurrentPeriodEnd: &future}, want: shared.PlanAccessFree},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectivePlan(tc.state, now); got != tc.want {
				t.Fatalf("effectivePlan() = %q, want %q", got, tc.want)
			}
		})
	}
}
