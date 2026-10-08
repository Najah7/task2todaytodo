package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	sharedprogress "github.com/Najah7/task2todaytodo/internal/application/shared/progress"
)

type projectLifecycleRepository interface {
	LockProjectForLifecycle(context.Context, domain.ProjectID) error
	ReadProjectStatusForLifecycle(context.Context, domain.ProjectID) (string, error)
	SetStatusForLifecycle(context.Context, domain.UserID, domain.ProjectID, string) error
}

// ProjectLifecycleUseCase owns automatic Project status transitions after a
// child operation. It runs inside the child operation's transaction, after
// the caller has locked the parent Project row.
type ProjectLifecycleUseCase struct {
	repo     projectLifecycleRepository
	progress ProjectProgressReader
}

func NewProjectLifecycleUseCase(repo projectLifecycleRepository, progress ProjectProgressReader) *ProjectLifecycleUseCase {
	return &ProjectLifecycleUseCase{repo: repo, progress: progress}
}

func (uc *ProjectLifecycleUseCase) LockParent(ctx context.Context, projectID string) error {
	err := uc.repo.LockProjectForLifecycle(ctx, domain.ProjectID(projectID))
	if errors.Is(err, domain.ErrProjectNotFound) {
		return shared.ErrProjectUnavailable
	}
	return err
}

// ReadStatus exposes only the parent state needed by child read projections.
// It deliberately avoids loading Project progress sources on ordinary reads.
func (uc *ProjectLifecycleUseCase) ReadStatus(ctx context.Context, projectID string) (string, error) {
	status, err := uc.repo.ReadProjectStatusForLifecycle(ctx, domain.ProjectID(projectID))
	if errors.Is(err, domain.ErrProjectNotFound) {
		return "", shared.ErrProjectUnavailable
	}
	return status, err
}

func (uc *ProjectLifecycleUseCase) CaptureWorkState(ctx context.Context, projectID string, asOf time.Time) (shared.ProjectWorkState, error) {
	status, err := uc.repo.ReadProjectStatusForLifecycle(ctx, domain.ProjectID(projectID))
	if err != nil {
		return shared.ProjectWorkState{}, err
	}
	sources, err := uc.progress.ReadProjectProgressSources(ctx, []string{projectID}, asOf)
	if err != nil {
		return shared.ProjectWorkState{}, err
	}
	return projectWorkState(status, sources, asOf)
}

func (uc *ProjectLifecycleUseCase) ReconcileWorkState(ctx context.Context, actorID, projectID string, before shared.ProjectWorkState, asOf time.Time) error {
	after, err := uc.CaptureWorkState(ctx, projectID, asOf)
	if err != nil {
		return err
	}
	status := after.Status
	if status == "done" && after.Unfinished() > before.Unfinished() {
		status = "open"
	} else if status != "done" &&
		(after.Eligible != before.Eligible || after.Completed != before.Completed) &&
		after.Eligible > 0 && after.Unfinished() == 0 {
		status = "done"
	}
	if status == after.Status {
		return nil
	}
	return uc.repo.SetStatusForLifecycle(ctx, domain.UserID(actorID), domain.ProjectID(projectID), status)
}

func projectWorkState(status string, sources dao.ProjectProgressSources, asOf time.Time) (shared.ProjectWorkState, error) {
	state := shared.ProjectWorkState{Status: status}
	for _, task := range sources.Tasks {
		state.Eligible++
		if task.Done {
			state.Completed++
		}
	}
	for _, schedule := range sources.Schedules {
		state.Eligible += schedule.Total
		state.Completed += schedule.Completed
		for _, root := range schedule.Roots {
			eligible, err := sharedprogress.VirtualOccurrenceOccursToday(progressRule(root), asOf)
			if err != nil {
				return shared.ProjectWorkState{}, err
			}
			if eligible {
				state.Eligible++
			}
		}
	}
	return state, nil
}
