package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ListBlueprintTagsUseCase struct {
	uow    UOW
	logger telemetry.Logger
}
type AddTagToBlueprintUseCase struct {
	uow    UOW
	logger telemetry.Logger
}
type RemoveTagFromBlueprintUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewListBlueprintTagsUseCase(uow UOW, logger telemetry.Logger) *ListBlueprintTagsUseCase {
	return &ListBlueprintTagsUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}
func NewAddTagToBlueprintUseCase(uow UOW, logger telemetry.Logger) *AddTagToBlueprintUseCase {
	return &AddTagToBlueprintUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}
func NewRemoveTagFromBlueprintUseCase(uow UOW, logger telemetry.Logger) *RemoveTagFromBlueprintUseCase {
	return &RemoveTagFromBlueprintUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *ListBlueprintTagsUseCase) Execute(ctx context.Context, actorID, blueprintID string) (output []dao.ScheduleTag, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "ListBlueprintTagsUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Blueprints().GetBlueprint(ctx, actorID, blueprintID); err != nil {
			return err
		}
		output, err = repos.Schedules().ListTags(ctx, domain.UserID(actorID), domain.ScheduleID(blueprintID))
		return err
	})
	if output == nil && err == nil {
		output = []dao.ScheduleTag{}
	}
	return output, err
}

func (uc *AddTagToBlueprintUseCase) Execute(ctx context.Context, actorID, blueprintID, tagID string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "AddTagToBlueprintUseCase.Execute", err) }()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Blueprints().GetBlueprint(ctx, actorID, blueprintID); err != nil {
			return err
		}
		return repos.Schedules().AddTag(ctx, domain.UserID(actorID), domain.ScheduleID(blueprintID), tagID)
	})
}

func (uc *RemoveTagFromBlueprintUseCase) Execute(ctx context.Context, actorID, blueprintID, tagID string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "RemoveTagFromBlueprintUseCase.Execute", err) }()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Blueprints().GetBlueprint(ctx, actorID, blueprintID); err != nil {
			return err
		}
		return repos.Schedules().RemoveTag(ctx, domain.UserID(actorID), domain.ScheduleID(blueprintID), tagID)
	})
}
