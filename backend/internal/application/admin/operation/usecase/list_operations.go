package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	sharedpagination "github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

type OperationPage struct {
	Items   []dao.Operation
	HasMore bool
}

type ListOperationsUseCase struct{ repo OperationRepository }

func NewListOperationsUseCase(repo OperationRepository) *ListOperationsUseCase {
	return &ListOperationsUseCase{repo: repo}
}
func (uc *ListOperationsUseCase) Execute(ctx context.Context, actorID, cursor string, size int) (OperationPage, error) {
	if actorID == "" {
		return OperationPage{}, ErrPermissionDenied
	}
	allowed, err := uc.repo.HasPermission(ctx, actorID, shared.AdminOperationRead())
	if err != nil {
		return OperationPage{}, err
	}
	if !allowed {
		return OperationPage{}, ErrPermissionDenied
	}
	if size <= 0 || size > sharedpagination.MaxPageSize {
		size = sharedpagination.DefaultPageSize
	}
	rows, err := uc.repo.List(ctx, cursor, size+1)
	if err != nil {
		return OperationPage{}, err
	}
	items, hasMore := sharedpagination.Window(rows, size)
	return OperationPage{Items: items, HasMore: hasMore}, nil
}
