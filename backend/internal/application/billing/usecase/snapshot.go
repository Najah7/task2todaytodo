package usecase

import (
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

func snapshotValues(cursor dao.SnapshotCursor, eventCreated int64, sub dao.Subscription) (plan, pendingPlan, scheduleID string, pendingAt *time.Time, apply bool) {
	if (eventCreated > 0 && eventCreated < cursor.LastEventCreated) || (sub.Created > 0 && sub.ID != cursor.SubscriptionID && cursor.SubscriptionCreated > 0 && sub.Created <= cursor.SubscriptionCreated) {
		return "", "", "", nil, false
	}
	plan, pendingPlan, scheduleID = cursor.Plan, cursor.PendingPlan, cursor.ScheduleID
	if sub.Plan != "" {
		plan = string(sub.Plan)
	}
	if sub.PendingPlan != nil {
		pendingPlan = string(*sub.PendingPlan)
	} else if pendingPlan != "" && plan == pendingPlan {
		// The target price is now the active price. A schedule may still be
		// present on this snapshot during Stripe's transition, so clear the
		// local scheduled-change state before considering schedule identity.
		pendingPlan, scheduleID, pendingAt = "", "", nil
	} else if pendingPlan != "" && cursor.PendingPlanAt == nil {
		pendingPlan = ""
	} else if pendingPlan != "" && sub.ScheduleID == scheduleID && scheduleID != "" {
		pendingAt = cursor.PendingPlanAt
	} else if scheduleID != "" && sub.ScheduleID != scheduleID {
		pendingPlan, scheduleID = "", ""
	}
	if sub.Status == "canceled" || sub.Status == "incomplete_expired" {
		pendingPlan, scheduleID, pendingAt = "", "", nil
	}
	return plan, pendingPlan, scheduleID, pendingAt, true
}
