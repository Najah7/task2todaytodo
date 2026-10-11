package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type GetOccurrenceUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewGetOccurrenceUseCase(uow UOW, loggers ...telemetry.Logger) *GetOccurrenceUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &GetOccurrenceUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *GetOccurrenceUseCase) Execute(ctx context.Context, actorID, blueprintID, occurrenceDate string) (output dao.ScheduleOccurrence, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "GetOccurrenceUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		value, err := occurrenceForDate(ctx, repos.Blueprints(), actorID, blueprintID, occurrenceDate)
		if err != nil {
			return err
		}
		output = value
		return nil
	})
	return output, err
}
