package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListScheduleAssigneesUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewListScheduleAssigneesUseCase(uow UOW, logger logging.Logger) *ListScheduleAssigneesUseCase {
	return &ListScheduleAssigneesUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *ListScheduleAssigneesUseCase) Execute(ctx context.Context, actorID domain.UserID, blueprintID domain.ScheduleID) (output []dao.Assignee, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListScheduleAssigneesUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var readErr error
		output, readErr = repos.Blueprints().ListBlueprintEligibleAssignees(ctx, string(actorID), string(blueprintID))
		return readErr
	})
	if err != nil {
		return nil, err
	}
	if output == nil {
		output = []dao.Assignee{}
	}
	return output, nil
}
