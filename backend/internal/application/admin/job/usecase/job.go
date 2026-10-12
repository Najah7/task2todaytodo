package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type jobUseCase struct {
	jobName  string
	repo     OperationRepository
	startJob func(jobName, id, leaseToken, actorID string, cutoff time.Time)
	id       IDIssuer
	clock    Clock
	lease    time.Duration
}

func newJobUseCase(jobName string, repo OperationRepository, runner JobRunner, id IDIssuer, clock Clock, lease time.Duration) *jobUseCase {
	if clock == nil {
		clock = systemClock{}
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	return &jobUseCase{jobName: jobName, repo: repo, startJob: runner.StartJob, id: id, clock: clock, lease: lease}
}

func (uc *jobUseCase) Execute(ctx context.Context, actorID string) (dao.Operation, error) {
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
		ID: uc.id.NewULID(), JobName: uc.jobName, State: "running", RequestedBy: actorID,
		Cutoff: now, CreatedAt: now, StartedAt: now,
	}, leaseToken, uc.lease)
	if err != nil {
		return dao.Operation{}, err
	}
	if created {
		operation.Name = "admin/operations/" + operation.ID
		uc.startJob(uc.jobName, operation.ID, token, actorID, operation.Cutoff)
	}
	return operation, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func NewSystemClock() Clock { return systemClock{} }
