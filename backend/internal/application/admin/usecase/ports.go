package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var ErrOperationNotFound = errors.New("operation not found")
var ErrPermissionDenied = errors.New("permission denied")

type OperationRepository interface {
	HasPermission(context.Context, string, shared.Capability) (bool, error)
	CreateOrGetActive(context.Context, dao.Operation, string, time.Duration) (operation dao.Operation, created bool, leaseToken string, err error)
	Get(context.Context, string) (dao.Operation, error)
	List(context.Context, string, int) ([]dao.Operation, error)
	Heartbeat(context.Context, string, string, time.Duration) (bool, error)
	Progress(context.Context, string, string, int64, int64, time.Duration) (bool, error)
	Succeed(context.Context, string, string, int64, int64) error
	Fail(context.Context, string, string, string, string) error
}

// BackfillExecutor is a consumer-owned primitive port implemented by root wiring.
type BackfillExecutor interface {
	Execute(ctx context.Context, actorID string, cutoff time.Time, report func(processedBlueprints, generatedSchedules int) error) (BackfillResult, error)
}

type BackfillResult struct {
	ProcessedBlueprints int64
	GeneratedSchedules  int64
}

type Clock interface{ Now() time.Time }
type IDGenerator interface{ Generate() string }

type Runner interface {
	StartOperation(id, leaseToken, actorID string, cutoff time.Time)
}

type UseCases struct {
	RunBackfill    *RunBackfillSchedulesUseCase
	ListOperations *ListOperationsUseCase
	GetOperation   *GetOperationUseCase
}
