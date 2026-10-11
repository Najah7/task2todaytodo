package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

const backfillJobName = "backfill-schedules"

type RunBackfillSchedulesUseCase struct {
	repo   OperationRepository
	runner Runner
	id     IDIssuer
	clock  Clock
	lease  time.Duration
}

func NewRunBackfillSchedulesUseCase(repo OperationRepository, runner Runner, id IDIssuer, clock Clock, lease time.Duration) *RunBackfillSchedulesUseCase {
	if clock == nil {
		clock = systemClock{}
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	return &RunBackfillSchedulesUseCase{repo: repo, runner: runner, id: id, clock: clock, lease: lease}
}

func (uc *RunBackfillSchedulesUseCase) Execute(ctx context.Context, actorID string) (dao.Operation, error) {
	if actorID == "" {
		return dao.Operation{}, ErrPermissionDenied
	}
	allowed, err := uc.repo.HasPermission(ctx, actorID, shared.AdminJobRun())
	if err != nil {
		return dao.Operation{}, err
	}
	if !allowed {
		return dao.Operation{}, ErrPermissionDenied
	}
	now := uc.clock.Now().UTC()
	leaseToken := uc.id.NewUUID()
	operation, created, token, err := uc.repo.CreateOrGetActive(ctx, dao.Operation{
		ID: uc.id.NewULID(), JobName: backfillJobName,
		State: "running", RequestedBy: actorID, Cutoff: now, CreatedAt: now, StartedAt: now,
	}, leaseToken, uc.lease)
	if err != nil {
		return dao.Operation{}, err
	}
	if created {
		operation.Name = "admin/operations/" + operation.ID
		uc.runner.StartOperation(operation.ID, token, actorID, operation.Cutoff)
	}
	return operation, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func NewSystemClock() Clock { return systemClock{} }
