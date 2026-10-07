package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type GetScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewGetScheduleUseCase(uow UOW, logger logging.Logger) *GetScheduleUseCase {
	return &GetScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *GetScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID) (output dao.Schedule, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "GetScheduleUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var readErr error
		output, readErr = repos.Schedules().GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleRead())
		return readErr
	})
	return output, err
}
