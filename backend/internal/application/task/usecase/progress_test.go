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

func (taskProgressTestRepository) LockByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}}, nil
}

func (taskProgressTestRepository) SetStatusByUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus) error {
	return nil
}

func (taskProgressTestRepository) SetStatusByUserIDWithPermission(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus, int32, shared.Capability) error {
	return nil
}
