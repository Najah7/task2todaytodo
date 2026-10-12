package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RescheduleOccurrenceUseCase struct {
	update *UpdateOccurrenceUseCase
}

func NewRescheduleOccurrenceUseCase(uow UOW, logger telemetry.Logger, issueID func() string) *RescheduleOccurrenceUseCase {
	return &RescheduleOccurrenceUseCase{update: NewUpdateOccurrenceUseCase(uow, logger, issueID)}
}

func (uc *RescheduleOccurrenceUseCase) Execute(ctx context.Context, actorID, ruleID, occurrenceDate string, startAt, endAt time.Time, expectedRevision int32) (dao.ScheduleOccurrence, error) {
	return uc.update.execute(ctx, actorID, ruleID, occurrenceDate, UpdateScheduleInput{
		StartAt: &startAt,
		EndAt:   &endAt,
	}, expectedRevision, "schedule.occurrence.reschedule")
}
