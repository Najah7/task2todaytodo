package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type AssignScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

type SetScheduleProjectUseCase struct {
	uow    UOW
	logger logging.Logger
}

type RemoveScheduleFromProjectUseCase struct {
	uow    UOW
	logger logging.Logger
}

type ListScheduleAssigneesUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewAssignScheduleUseCase(uow UOW, logger logging.Logger) *AssignScheduleUseCase {
	return &AssignScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewSetScheduleProjectUseCase(uow UOW, logger logging.Logger) *SetScheduleProjectUseCase {
	return &SetScheduleProjectUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewRemoveScheduleFromProjectUseCase(uow UOW, logger logging.Logger) *RemoveScheduleFromProjectUseCase {
	return &RemoveScheduleFromProjectUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewListScheduleAssigneesUseCase(uow UOW, logger logging.Logger) *ListScheduleAssigneesUseCase {
	return &ListScheduleAssigneesUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *AssignScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, assigneeID domain.UserID) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "AssignScheduleUseCase.Execute", err) }()
	if assigneeID == "" {
		return domain.ErrScheduleAssigneeIDEmpty
	}
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		repository := repos.Schedules()
		return withScheduleProjectMutation(ctx, repos, actorID, scheduleID, asOf, shared.ScheduleAssignmentUpdate(), func(_ dao.Schedule, _ scheduleProjectMutationSnapshots) error {
			eligibleAssignees, err := repository.ListEligibleAssignees(ctx, actorID, scheduleID)
			if err != nil {
				return err
			}
			eligible := false
			for _, assignee := range eligibleAssignees {
				if assignee.ID == string(assigneeID) {
					eligible = true
					break
				}
			}
			if !eligible {
				return ErrScheduleAssigneeNotEligible
			}
			return repository.SetAssigneeByUserID(ctx, actorID, scheduleID, assigneeID)
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.assign", actorID, scheduleID)
	}
	return err
}

func (uc *SetScheduleProjectUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, projectID *domain.ProjectID) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "SetScheduleProjectUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMove(ctx, repos, actorID, scheduleID, projectID, nil, asOf)
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.set_project", actorID, scheduleID)
	}
	return err
}

func (uc *RemoveScheduleFromProjectUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, expectedProjectID domain.ProjectID) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "RemoveScheduleFromProjectUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMove(ctx, repos, actorID, scheduleID, nil, &expectedProjectID, asOf)
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.remove_from_project", actorID, scheduleID)
	}
	return err
}

func (uc *ListScheduleAssigneesUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID) (output []dao.Assignee, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListScheduleAssigneesUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var readErr error
		output, readErr = repos.Schedules().ListEligibleAssignees(ctx, actorID, scheduleID)
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
