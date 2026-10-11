package application

import (
	"context"
	"time"

	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

func NewAdminUseCases(store AdminStore, backfill adminusecase.BackfillExecutor, ids shared.ID, logger logging.Logger) (adminusecase.UseCases, func(context.Context) error) {
	runner := adminusecase.NewAsyncRunner(store.Operations, backfill, context.Background(), 30*time.Second, 10*time.Second, logger)
	return adminusecase.UseCases{
		RunBackfill:    adminusecase.NewRunBackfillSchedulesUseCase(store.Operations, runner, ids, adminusecase.NewSystemClock(), 30*time.Second),
		ListOperations: adminusecase.NewListOperationsUseCase(store.Operations),
		GetOperation:   adminusecase.NewGetOperationUseCase(store.Operations),
	}, runner.Shutdown
}
