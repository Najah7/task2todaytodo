package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
)

const prefillAllocationsJobName = "prefill-allocations"

type PrefillAllocationsUseCase struct{ job *jobUseCase }

func NewPrefillAllocationsUseCase(repo OperationRepository, runner JobRunner, id IDIssuer, clock Clock, lease time.Duration) *PrefillAllocationsUseCase {
	return &PrefillAllocationsUseCase{job: newJobUseCase(prefillAllocationsJobName, repo, runner, id, clock, lease)}
}

func (uc *PrefillAllocationsUseCase) Execute(ctx context.Context, actorID string) (dao.Operation, error) {
	return uc.job.Execute(ctx, actorID)
}
