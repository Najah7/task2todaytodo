package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListScheduleTagsUseCase struct {
	uow    UOW
	logger logging.Logger
}

type ListScheduleTagsForSchedulesUseCase struct {
	uow    UOW
	logger logging.Logger
}

type AddTagToScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

type RemoveTagFromScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewListScheduleTagsUseCase(uow UOW, logger logging.Logger) *ListScheduleTagsUseCase {
	return &ListScheduleTagsUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewListScheduleTagsForSchedulesUseCase(uow UOW, logger logging.Logger) *ListScheduleTagsForSchedulesUseCase {
	return &ListScheduleTagsForSchedulesUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewAddTagToScheduleUseCase(uow UOW, logger logging.Logger) *AddTagToScheduleUseCase {
	return &AddTagToScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewRemoveTagFromScheduleUseCase(uow UOW, logger logging.Logger) *RemoveTagFromScheduleUseCase {
	return &RemoveTagFromScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *ListScheduleTagsUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID) (output []dao.ScheduleTag, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListScheduleTagsUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Schedules().GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleRead()); err != nil {
			return err
		}
		output, err = repos.Schedules().ListTags(ctx, actorID, scheduleID)
		return err
	})
	if output == nil && err == nil {
		output = []dao.ScheduleTag{}
	}
	return output, err
}

// Execute batches tag projection for authorized Schedule rows. The repository
// resolves each requested row's series root while preserving row-level read ACL.
func (uc *ListScheduleTagsForSchedulesUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleIDs []domain.ScheduleID) (output map[string][]dao.ScheduleTag, err error) {
	defer func() {
		logUnexpectedScheduleFailure(uc.logger, ctx, "ListScheduleTagsForSchedulesUseCase.Execute", err)
	}()
	output = make(map[string][]dao.ScheduleTag, len(scheduleIDs))
	ids := make([]domain.ScheduleID, 0, len(scheduleIDs))
	seen := make(map[domain.ScheduleID]struct{}, len(scheduleIDs))
	for _, id := range scheduleIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		output[string(id)] = []dao.ScheduleTag{}
	}
	if len(ids) == 0 {
		return output, nil
	}
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var readErr error
		output, readErr = repos.Schedules().ListTagsForSchedules(ctx, actorID, ids)
		return readErr
	})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if output[string(id)] == nil {
			output[string(id)] = []dao.ScheduleTag{}
		}
	}
	return output, nil
}

func (uc *AddTagToScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, tagID string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "AddTagToScheduleUseCase.Execute", err) }()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		_, err := repos.Schedules().GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleUpdate())
		if err != nil {
			return err
		}
		return repos.Schedules().AddTag(ctx, actorID, scheduleID, tagID)
	})
}

func (uc *RemoveTagFromScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, tagID string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "RemoveTagFromScheduleUseCase.Execute", err) }()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		_, err := repos.Schedules().GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleUpdate())
		if err != nil {
			return err
		}
		return repos.Schedules().RemoveTag(ctx, actorID, scheduleID, tagID)
	})
}
