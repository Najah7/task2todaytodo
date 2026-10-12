package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var ErrPermissionDenied = errors.New("permission denied")

// OperationRepository contains only operation lifecycle behavior needed to run jobs.
type OperationRepository interface {
	HasPermission(context.Context, string, shared.Capability) (bool, error)
	CreateOrGetActive(context.Context, dao.Operation, string, time.Duration) (operation dao.Operation, created bool, leaseToken string, err error)
	Heartbeat(context.Context, string, string, time.Duration) (bool, error)
	Progress(context.Context, string, string, int64, int64, time.Duration) (bool, error)
	Succeed(context.Context, string, string, int64, int64) error
	Fail(context.Context, string, string, string, string) error
}

// JobExecutor runs one named admin job. Progress counters use generic names so
// the shared operation runner can host independently leased jobs.
type JobExecutor interface {
	Execute(ctx context.Context, actorID string, cutoff time.Time, report func(processedRules, generatedOccurrences int) error) (processedRules, generatedOccurrences int, err error)
}

type Clock interface{ Now() time.Time }
type IDIssuer interface {
	NewULID() string
	NewUUID() string
}

type Runner interface {
	StartOperation(id, leaseToken, actorID string, cutoff time.Time)
}

type JobRunner interface {
	Runner
	StartJob(jobName, id, leaseToken, actorID string, cutoff time.Time)
}

type UseCases struct {
	RunBackfill           *BackfillSchedulesUseCase
	RunPrefillAllocations *PrefillAllocationsUseCase
}
