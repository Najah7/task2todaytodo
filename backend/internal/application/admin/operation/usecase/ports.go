package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var ErrOperationNotFound = errors.New("operation not found")
var ErrLeaseLost = errors.New("operation lease lost")
var ErrPermissionDenied = errors.New("permission denied")

type OperationRepository interface {
	HasPermission(context.Context, string, shared.Capability) (bool, error)
	Get(context.Context, string) (dao.Operation, error)
	List(context.Context, string, int) ([]dao.Operation, error)
}

type UseCases struct {
	ListOperations *ListOperationsUseCase
	GetOperation   *GetOperationUseCase
}
