package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var ErrPaidPlanRequired = errors.New("Pro or Team plan required")

func requirePaid(ctx context.Context, access shared.PlanAccessReader, userID string) error {
	if access == nil {
		return ErrPaidPlanRequired
	}
	plan, err := access.GetPlanAccess(ctx, userID)
	if err != nil {
		return err
	}
	if !plan.IsPaid() {
		return ErrPaidPlanRequired
	}
	return nil
}
