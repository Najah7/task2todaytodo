package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
)

type GetBlueprintUseCase struct{ uow UOW }

func NewGetBlueprintUseCase(uow UOW) *GetBlueprintUseCase { return &GetBlueprintUseCase{uow: uow} }

func (uc *GetBlueprintUseCase) Execute(ctx context.Context, actorID, blueprintID string) (dao.Blueprint, error) {
	var result dao.Blueprint
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		result, err = repos.Blueprints().GetBlueprint(ctx, actorID, blueprintID)
		return err
	})
	return result, err
}
