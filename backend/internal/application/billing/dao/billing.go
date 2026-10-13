package dao

import "time"

import "github.com/Najah7/task2todaytodo/internal/application/shared"

type Plan string

const (
	PlanPro  Plan = "pro"
	PlanTeam Plan = "team"
)

type State struct {
	Plan              string            `json:"plan"`
	AccessPlan        shared.PlanAccess `json:"access_plan"`
	Status            string            `json:"status"`
	CurrentPeriodEnd  *time.Time        `json:"current_period_end"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	PendingPlan       *string           `json:"pending_plan"`
	PendingPlanAt     *time.Time        `json:"pending_plan_at"`
	CheckoutPending   bool              `json:"checkout_pending"`
}

type Subscription struct {
	ID                 string
	Created            int64
	CustomerID         string
	Status             string
	Plan               Plan
	PriceID            string
	PendingPlan        *Plan
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	CancelAtPeriodEnd  bool
	ScheduleID         string
}

type CheckoutReservation struct {
	IdempotencyKey string
	Plan           string
	SessionID      string
	URL            string
	ExpiresAt      *time.Time
}

type Account struct {
	State          State
	CustomerID     string
	SubscriptionID string
	ScheduleID     string
	PriceID        string
	CheckoutPlan   string
	Checkout       CheckoutReservation
}

type SnapshotCursor struct {
	UserID              string
	Plan                string
	PendingPlan         string
	ScheduleID          string
	PendingPlanAt       *time.Time
	SubscriptionCreated int64
	LastEventCreated    int64
	SubscriptionID      string
}
