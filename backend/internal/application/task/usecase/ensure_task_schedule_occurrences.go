package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

// EnsureTaskScheduleOccurrencesUseCase is retained for source compatibility.
// Recurring schedules are virtual and are never materialized by this type.
//
// Deprecated: callers should rely on list expansion instead.
type EnsureTaskScheduleOccurrencesUseCase struct{}

func NewEnsureTaskScheduleOccurrencesUseCase(_ shared.ID) *EnsureTaskScheduleOccurrencesUseCase {
	return &EnsureTaskScheduleOccurrencesUseCase{}
}

func (*EnsureTaskScheduleOccurrencesUseCase) Execute(context.Context, Repositories, domain.UserID, domain.TaskSchedule, time.Time) error {
	return nil
}
