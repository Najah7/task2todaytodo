package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

// taskProgressTestRepository provides explicit inert defaults for unrelated
// usecase fixtures while preserving the required production repository port.
// Behavior-focused tests provide their own counts and status mutations.
type taskProgressTestRepository struct{ TaskRepository }

type taskProgressTestRepositories struct{ Repositories }

func (taskProgressTestRepositories) Tasks() TaskRepository { return taskProgressTestRepository{} }
func (taskProgressTestRepositories) ActionItems() ActionItemRepository {
	return taskProgressTestActionItems{}
}

type taskProgressTestActionItems struct{ ActionItemRepository }

func (taskProgressTestActionItems) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}
func (taskProgressTestRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle {
	return inertProjectWorkLifecycle{}
}

type inertProjectWorkLifecycle struct{}

func (inertProjectWorkLifecycle) LockParent(context.Context, string) error { return nil }
func (inertProjectWorkLifecycle) ReadStatus(context.Context, string) (string, error) {
	return "open", nil
}
func (inertProjectWorkLifecycle) CaptureWorkState(context.Context, string, time.Time) (shared.ProjectWorkState, error) {
	return shared.ProjectWorkState{Status: "open"}, nil
}
func (inertProjectWorkLifecycle) ReconcileWorkState(context.Context, string, string, shared.ProjectWorkState, time.Time) error {
	return nil
}

func (taskProgressTestRepository) ReadTaskProgressSources(_ context.Context, taskIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
	sources := dao.TaskProgressSources{Counts: map[string]dao.TaskProgressCounts{}, Statuses: map[string]dao.TaskStatus{}}
	for _, id := range taskIDs {
		sources.Statuses[id] = dao.TaskStatus{Value: "open"}
	}
	return sources, nil
}

func (taskProgressTestRepository) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}}, nil
}

func (taskProgressTestRepository) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}}, nil
}

func (taskProgressTestRepository) LockByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}}, nil
}

func (taskProgressTestRepository) SetStatusByUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus) error {
	return nil
}

func (taskProgressTestRepository) SetStatusByUserIDWithPermission(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus, int32, shared.Capability) error {
	return nil
}

func (taskProgressTestRepository) BumpRevisionByUserID(context.Context, domain.UserID, domain.TaskID, int32, shared.Capability) error {
	return nil
}

func (taskProgressTestRepository) HasPermission(context.Context, domain.UserID, domain.TaskID, shared.Capability) (bool, error) {
	return true, nil
}
