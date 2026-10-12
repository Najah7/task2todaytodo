package application

import (
	"context"
	"time"

	jobusecase "github.com/Najah7/task2todaytodo/internal/application/admin/job/usecase"
	operationusecase "github.com/Najah7/task2todaytodo/internal/application/admin/operation/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type AdminUseCases struct {
	Job       jobusecase.UseCases
	Operation operationusecase.UseCases
}

func NewAdminUseCases(store AdminStore, backfill jobusecase.JobExecutor, id IDIssuer, logger telemetry.Logger, allocations ...jobusecase.JobExecutor) (AdminUseCases, func(context.Context) error) {
	executors := map[string]jobusecase.JobExecutor{"backfill-schedules": backfill}
	if len(allocations) > 0 {
		executors["prefill-allocations"] = allocations[0]
	}
	runner := jobusecase.NewAsyncJobRunner(store.Operations, executors, context.Background(), 30*time.Second, 10*time.Second, logger)
	usecases := AdminUseCases{
		Job: jobusecase.UseCases{
			RunBackfill: jobusecase.NewBackfillSchedulesUseCase(store.Operations, runner, id, jobusecase.NewSystemClock(), 30*time.Second),
		},
		Operation: operationusecase.UseCases{
			ListOperations: operationusecase.NewListOperationsUseCase(store.Operations),
			GetOperation:   operationusecase.NewGetOperationUseCase(store.Operations),
		},
	}
	if len(allocations) > 0 {
		usecases.Job.RunPrefillAllocations = jobusecase.NewPrefillAllocationsUseCase(store.Operations, runner, id, jobusecase.NewSystemClock(), 30*time.Second)
	}
	return usecases, runner.Shutdown
}
