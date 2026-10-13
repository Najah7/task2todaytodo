package usecase

import (
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

func TestSnapshotValuesClearsDowngradeWhenTargetPlanBecomesActive(t *testing.T) {
	pendingAt := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	cursor := dao.SnapshotCursor{
		UserID:        "user",
		Plan:          "team",
		PendingPlan:   "pro",
		ScheduleID:    "sched_downgrade",
		PendingPlanAt: &pendingAt,
	}
	sub := dao.Subscription{
		ID:         "sub",
		CustomerID: "cus",
		Plan:       dao.PlanPro,
		ScheduleID: "sched_downgrade",
	}

	plan, pendingPlan, scheduleID, actualPendingAt, apply := snapshotValues(cursor, 0, sub)
	if !apply {
		t.Fatal("snapshot was ignored")
	}
	if plan != "pro" || pendingPlan != "" || scheduleID != "" || actualPendingAt != nil {
		t.Fatalf("snapshot values = (%q, %q, %q, %v); want active Pro with no pending schedule", plan, pendingPlan, scheduleID, actualPendingAt)
	}
}
