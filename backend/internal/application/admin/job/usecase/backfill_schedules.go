package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
)

const backfillJobName = "backfill-schedules"

type BackfillSchedulesUseCase struct {
	job *jobUseCase
}

func NewBackfillSchedulesUseCase(repo OperationRepository, runner Runner, id IDIssuer, clock Clock, lease time.Duration) *BackfillSchedulesUseCase {
	jobRunner, ok := runner.(JobRunner)
	if !ok {
		// Preserve the original backfill runner port for existing callers.
		jobRunner = backfillJobRunner{Runner: runner}
	}
	return &BackfillSchedulesUseCase{job: newJobUseCase(backfillJobName, repo, jobRunner, id, clock, lease)}
}

type backfillJobRunner struct{ Runner }

func (r backfillJobRunner) StartJob(_ string, id, leaseToken, actorID string, cutoff time.Time) {
	r.StartOperation(id, leaseToken, actorID, cutoff)
}

func (uc *BackfillSchedulesUseCase) Execute(ctx context.Context, actorID string) (dao.Operation, error) {
	return uc.job.Execute(ctx, actorID)
}
