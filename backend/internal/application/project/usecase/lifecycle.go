package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
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
	repo        projectLifecycleRepository
	progress    ProjectProgressReader
	schedules   ProjectScheduleLifecycle
	allocations ProjectAllocationLifecycle
}

type ProjectScheduleLifecycle interface {
	PauseProjectRules(context.Context, string, string, time.Time) error
	ResumeProjectRules(context.Context, string, string, time.Time) error
}

type ProjectAllocationLifecycle interface {
	PauseProjectAllocations(context.Context, string, string, time.Time) error
	ResumeProjectAllocations(context.Context, string, string, time.Time) error
}

func NewProjectLifecycleUseCase(repo projectLifecycleRepository, progress ProjectProgressReader, schedules ProjectScheduleLifecycle, allocations ...ProjectAllocationLifecycle) *ProjectLifecycleUseCase {
	uc := &ProjectLifecycleUseCase{repo: repo, progress: progress, schedules: schedules}
	if len(allocations) > 0 {
		uc.allocations = allocations[0]
	}
	return uc
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
	return projectWorkState(status, sources), nil
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
	if err := uc.repo.SetStatusForLifecycle(ctx, domain.UserID(actorID), domain.ProjectID(projectID), status); err != nil {
		return err
	}
	if err := transitionProjectSchedules(ctx, uc.schedules, actorID, projectID, after.Status, status, asOf); err != nil {
		return err
	}
	return transitionProjectAllocations(ctx, uc.allocations, actorID, projectID, after.Status, status, asOf)
}

func transitionProjectSchedules(ctx context.Context, schedules ProjectScheduleLifecycle, actorID, projectID, from, to string, at time.Time) error {
	if from == to {
		return nil
	}
	if to == "done" {
		return schedules.PauseProjectRules(ctx, actorID, projectID, at)
	}
	if from == "done" {
		return schedules.ResumeProjectRules(ctx, actorID, projectID, at)
	}
	return nil
}

func transitionProjectAllocations(ctx context.Context, allocations ProjectAllocationLifecycle, actorID, projectID, from, to string, at time.Time) error {
	if allocations == nil || from == to {
		return nil
	}
	if to == "done" {
		return allocations.PauseProjectAllocations(ctx, actorID, projectID, at)
	}
	if from == "done" {
		return allocations.ResumeProjectAllocations(ctx, actorID, projectID, at)
	}
	return nil
}

func projectWorkState(status string, sources dao.ProjectProgressSources) shared.ProjectWorkState {
	state := shared.ProjectWorkState{Status: status}
	for _, task := range sources.Tasks {
		state.Eligible++
		if task.TaskDone {
			state.Completed++
		}
	}
	for _, schedule := range sources.Schedules {
		state.Eligible += schedule.Total
		state.Completed += schedule.Completed
	}
	return state
}
