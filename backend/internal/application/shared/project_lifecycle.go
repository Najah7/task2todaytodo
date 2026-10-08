package shared

import (
	"context"
	"errors"
	"time"
)

var ErrProjectUnavailable = errors.New("project is no longer available")

// ProjectWorkState is the primitive status/count snapshot that child contexts
// use to request Project-owned automatic lifecycle reconciliation.
type ProjectWorkState struct {
	Status    string
	Eligible  int
	Completed int
}

func (s ProjectWorkState) Unfinished() int { return s.Eligible - s.Completed }

// ProjectWorkLifecycle is implemented by the Project context. Callers must
// lock the parent Project before capturing counts or mutating a child.
type ProjectWorkLifecycle interface {
	LockParent(context.Context, string) error
	ReadStatus(context.Context, string) (string, error)
	CaptureWorkState(context.Context, string, time.Time) (ProjectWorkState, error)
	ReconcileWorkState(context.Context, string, string, ProjectWorkState, time.Time) error
}
