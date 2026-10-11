package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type GetOperationUseCase struct{ repo OperationRepository }

func NewGetOperationUseCase(repo OperationRepository) *GetOperationUseCase {
	return &GetOperationUseCase{repo: repo}
}
func (uc *GetOperationUseCase) Execute(ctx context.Context, actorID, id string) (dao.Operation, error) {
	if actorID == "" {
		return dao.Operation{}, ErrPermissionDenied
	}
	allowed, err := uc.repo.HasPermission(ctx, actorID, shared.AdminOperationRead())
	if err != nil {
		return dao.Operation{}, err
	}
	if !allowed {
		return dao.Operation{}, ErrPermissionDenied
	}
	op, err := uc.repo.Get(ctx, id)
	return op, err
}
