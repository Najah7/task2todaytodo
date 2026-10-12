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

func (uc *RescheduleOccurrenceUseCase) Execute(ctx context.Context, actorID, ruleID, occurrenceDate string, startAt, endAt time.Time, expectedRevision int32, confirmTodoRegeneration ...bool) (dao.ScheduleOccurrence, error) {
	confirmed := len(confirmTodoRegeneration) > 0 && confirmTodoRegeneration[0]
	return uc.update.execute(ctx, actorID, ruleID, occurrenceDate, UpdateScheduleInput{
		StartAt:                 &startAt,
		EndAt:                   &endAt,
		ConfirmTodoRegeneration: confirmed,
	}, expectedRevision, "schedule.occurrence.reschedule")
}
