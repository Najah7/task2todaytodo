package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type addTaskToProjectUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type AddTaskToProjectUseCase struct {
	uow      addTaskToProjectUOW
	progress taskProgressSource
	logger   logging.Logger
}

func NewAddTaskToProjectUseCase(uow addTaskToProjectUOW, progress taskProgressSource, logger logging.Logger) *AddTaskToProjectUseCase {
	return &AddTaskToProjectUseCase{logger: logging.OrNop(logger), uow: uow, progress: progress}
}

func (uc *AddTaskToProjectUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, taskID domain.TaskID, expectedRevision int32) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "AddTaskToProjectUseCase.Execute", err) }()

	var result dao.Task
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		tasks := repos.Tasks()
		task, err := tasks.GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
		if err != nil {
			return err
		}
		if task.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		initialProjectID := task.ProjectID
		locks := []struct {
			id         domain.ProjectID
			permission shared.Capability
		}{{id: projectID, permission: shared.TaskCreate()}}
		if task.ProjectID != "" && task.ProjectID != string(projectID) {
			locks = append(locks, struct {
				id         domain.ProjectID
				permission shared.Capability
			}{id: domain.ProjectID(task.ProjectID), permission: shared.TaskUpdate()})
		}
		sort.Slice(locks, func(i, j int) bool { return locks[i].id < locks[j].id })
		var targetProject TaskProject
		for _, lock := range locks {
			project, err := repos.TaskProjects().LockProjectByUserIDWithPermission(ctx, string(userID), string(lock.id), lock.permission)
			if err != nil {
				return err
			}
			if lock.id == projectID {
				targetProject = project
			}
		}
		if task.UserID != targetProject.OwnerID {
			return ErrTaskNotFound
		}
		// Re-read after the ordered project locks so a concurrent move cannot
		// leave this command acting on a stale project relationship.
		task, err = tasks.GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
		if err != nil {
			return err
		}
		if task.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if task.ProjectID != initialProjectID {
			return ErrRevisionConflict
		}

		lifecycle := repos.ProjectLifecycle()
		if lifecycle == nil {
			return ErrProjectLifecycleUnavailable
		}
		beforeByProject := make(map[string]shared.ProjectWorkState, 2)
		for _, projectID := range []string{initialProjectID, string(projectID)} {
			if projectID == "" {
				continue
			}
			if _, captured := beforeByProject[projectID]; captured {
				continue
			}
			before, err := lifecycle.CaptureWorkState(ctx, projectID, asOf)
			if err != nil {
				return err
			}
			beforeByProject[projectID] = before
		}

		changedAssociation := task.ProjectID != string(projectID)
		if task.ProjectID == string(projectID) {
			result = task
		} else {
			result, err = tasks.AssignToProjectByUserID(ctx, userID, taskID, projectID, expectedRevision)
			if err != nil {
				return err
			}
		}
		if changedAssociation {
			if initialProjectID != "" {
				if err := lifecycle.ReconcileWorkState(ctx, string(userID), initialProjectID, beforeByProject[initialProjectID], asOf); err != nil {
					return err
				}
			}
			if err := lifecycle.ReconcileWorkState(ctx, string(userID), string(projectID), beforeByProject[string(projectID)], asOf); err != nil {
				return err
			}
		}
		rows, err := applyTaskProgress(ctx, tasks, []dao.Task{result}, time.Now())
		if err != nil {
			return err
		}
		result = rows[0]
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	return result, nil

}
