package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type assignTaskTestUOW struct{ repos Repositories }

func (uow assignTaskTestUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, uow.repos)
}

type assignTaskTestRepositories struct {
	Repositories
	tasks TaskRepository
}

func (repos assignTaskTestRepositories) Tasks() TaskRepository { return repos.tasks }

type assignTaskTestRepository struct {
	TaskRepository
	task       dao.Task
	updated    dao.Task
	updateArgs struct {
		actorID          domain.UserID
		taskID           domain.TaskID
		assigneeID       domain.UserID
		expectedRevision int32
	}
}

func (repo *assignTaskTestRepository) LockProjectForTaskAssignment(context.Context, domain.UserID, domain.TaskID) error {
	return nil
}

func (repo *assignTaskTestRepository) IsEligibleTaskAssignee(context.Context, domain.UserID, domain.TaskID, domain.UserID) (bool, error) {
	return true, nil
}

func (repo *assignTaskTestRepository) Get(_ context.Context, taskID domain.TaskID) (dao.Task, error) {
	if repo.task.ID != string(taskID) {
		return dao.Task{}, ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *assignTaskTestRepository) UpdateTaskAssigneeByActor(_ context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID, expectedRevision int32) (dao.Task, error) {
	repo.updateArgs.actorID = actorID
	repo.updateArgs.taskID = taskID
	repo.updateArgs.assigneeID = assigneeID
	repo.updateArgs.expectedRevision = expectedRevision
	repo.updated = repo.task
	repo.updated.AssigneeID = string(assigneeID)
	repo.updated.Revision = expectedRevision + 1
	// The write query returns a narrow row without a derived progress value.
	repo.updated.Progress = 0
	return repo.updated, nil
}

func (repo *assignTaskTestRepository) ReadTaskProgressSources(_ context.Context, taskIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
	counts := make(map[string]dao.TaskProgressCounts, len(taskIDs))
	statuses := make(map[string]dao.TaskStatus, len(taskIDs))
	for _, taskID := range taskIDs {
		counts[taskID] = dao.TaskProgressCounts{Total: 4, Completed: 2}
		statuses[taskID] = dao.TaskStatus{Value: "open"}
	}
	return dao.TaskProgressSources{Counts: counts, Statuses: statuses}, nil
}

func TestAssignTaskReturnsProgressAlongWithPersistedAssigneeAndRevision(t *testing.T) {
	now := time.Date(2026, time.October, 5, 8, 0, 0, 0, time.UTC).Unix()
	repository := &assignTaskTestRepository{task: dao.Task{
		ID: "task-1", UserID: "owner-1", AssigneeID: "owner-1", ProjectID: "project-1",
		Title: "Task", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"},
		Revision: 6, CreatedAt: now, UpdatedAt: now,
	}}
	uow := assignTaskTestUOW{repos: assignTaskTestRepositories{tasks: repository}}

	got, err := NewAssignTaskUseCase(uow, nil).Execute(context.Background(), "editor-1", "task-1", "viewer-1", 6)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.AssigneeID != "viewer-1" || got.Revision != 7 || got.Progress != 50 || got.Status.Value != "open" {
		t.Fatalf("Execute() result assignee=%q revision=%d progress=%d status=%q; want viewer-1/7/50/open", got.AssigneeID, got.Revision, got.Progress, got.Status.Value)
	}
	if repository.updated.Progress != 0 {
		t.Fatalf("write result fixture progress = %d, want narrow query result without derived progress", repository.updated.Progress)
	}
	if repository.updateArgs.actorID != "editor-1" || repository.updateArgs.taskID != "task-1" || repository.updateArgs.assigneeID != "viewer-1" || repository.updateArgs.expectedRevision != 6 {
		t.Fatalf("assignment repository arguments = %+v, want actual actor/task/new assignee/revision", repository.updateArgs)
	}
}
