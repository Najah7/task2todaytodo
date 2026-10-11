package application

import (
	"context"
	"time"

	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
)

type scheduleBackfillAdapter struct {
	usecase scheduleusecase.ScheduleBackfill
}

func (adapter scheduleBackfillAdapter) Execute(
	ctx context.Context,
	actorID string,
	cutoff time.Time,
	report func(processedBlueprints, generatedSchedules int) error,
) (adminusecase.BackfillResult, error) {
	result, err := adapter.usecase.Execute(ctx, actorID, cutoff, report)
	if err != nil {
		return adminusecase.BackfillResult{}, err
	}
	return adminusecase.BackfillResult{
		ProcessedBlueprints: int64(result.ProcessedBlueprints),
		GeneratedSchedules:  int64(result.GeneratedSchedules),
	}, nil
}

var _ adminusecase.BackfillExecutor = scheduleBackfillAdapter{}
