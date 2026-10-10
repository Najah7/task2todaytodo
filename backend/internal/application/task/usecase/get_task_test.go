package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type getTaskUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *getTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type getTaskRepositoriesFake struct {
	taskProgressTestRepositories
	tasks    TaskRepository
	tags     TaskTagRepository
	items    TaskListProjectionReader
	accesses *[]string
}

func (repos getTaskRepositoriesFake) ActionItems() ActionItemRepository {
	if items, ok := repos.items.(ActionItemRepository); ok {
		return items
	}
	return taskProgressTestActionItems{}
}

func (repos getTaskRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos getTaskRepositoriesFake) TaskTags() TaskTagRepository {
	*repos.accesses = append(*repos.accesses, "tags")
	return repos.tags
}

type getTaskTaskRepositoryFake struct {
	taskProgressTestRepository
	taskProgressSourceFake
	task     dao.Task
	err      error
	userID   domain.UserID
	taskID   domain.TaskID
	calls    int
	accesses *[]string
}

func (repo *getTaskTaskRepositoryFake) ReadTaskProgressSources(ctx context.Context, taskIDs []string, asOf time.Time) (dao.TaskProgressSources, error) {
	return repo.taskProgressSourceFake.ReadTaskProgressSources(ctx, taskIDs, asOf)
}

func (repo *getTaskTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.err != nil {
		return dao.Task{}, repo.err
	}
	if repo.task.ID == "" || repo.task.UserID != string(userID) {
		return dao.Task{}, ErrTaskNotFound
	}
	return repo.task, nil
}

type getTaskTagRepositoryFake struct {
	TaskTagRepository
	tags     []dao.TaskTag
	err      error
	userID   domain.UserID
	taskID   domain.TaskID
	calls    int
	accesses *[]string
}

type getTaskProjectionFake struct{ rows []dao.ActionItem }

type getTaskActionItemsFake struct {
	taskProgressTestActionItems
	getTaskProjectionFake
}

func (reader getTaskActionItemsFake) ReadTaskListProjection(ctx context.Context, userID domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	return reader.getTaskProjectionFake.ReadTaskListProjection(ctx, userID, taskIDs)
}

func (reader getTaskProjectionFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = reader.rows
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}

func (repo *getTaskTagRepositoryFake) ListByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskTag, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "list-tags")
	return repo.tags, repo.err
}

func TestGetTaskUseCaseExecuteReturnsOwnedTaskAndTags(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantTask := dao.Task{ID: string(taskID), UserID: string(userID), Title: "Owned task", Progress: 66}
	wantTags := []dao.TaskTag{{ID: "tag-1", UserID: string(userID), Name: "urgent"}}
	var accesses []string
	taskRepo := &getTaskTaskRepositoryFake{
		task: wantTask, accesses: &accesses,
		taskProgressSourceFake: taskProgressSourceFake{sources: dao.TaskProgressSources{
			Counts: map[string]dao.TaskProgressCounts{string(taskID): {Total: 3, Completed: 2}},
		}},
	}
	tagRepo := &getTaskTagRepositoryFake{tags: wantTags, accesses: &accesses}
	rows := []dao.ActionItem{
		{ID: "item-1", TaskID: string(taskID), SeriesID: "item-1", RepeatState: repeatStateOneOff, OccurrenceDate: "2026-10-10", Completed: true},
		{ID: "item-2", TaskID: string(taskID), SeriesID: "item-2", RepeatState: repeatStateOneOff, OccurrenceDate: "2026-10-10", Completed: true},
		{ID: "item-3", TaskID: string(taskID), SeriesID: "item-3", RepeatState: repeatStateOneOff, OccurrenceDate: "2026-10-10"},
	}
	wantTask.Progress, wantTask.ActionItemCount, wantTask.ActionItemCompletedCount = 66, 3, 2
	wantTask.EstimateSource = "action_items"
	wantTask.CanUpdate = true
	uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{
		tasks: taskRepo, tags: tagRepo, items: getTaskActionItemsFake{getTaskProjectionFake: getTaskProjectionFake{rows: rows}}, accesses: &accesses,
	}}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, TaskWithTags{Task: wantTask, Tags: wantTags}) {
		t.Errorf("Execute() = %#v, want task and tags", got)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || tagRepo.calls != 1 {
		t.Errorf("calls = UOW:%d task:%d tags:%d, want 1 each", uow.calls, taskRepo.calls, tagRepo.calls)
	}
	if taskRepo.userID != userID || taskRepo.taskID != taskID || tagRepo.userID != userID || tagRepo.taskID != taskID {
		t.Errorf("repository scope = task(%q,%q), tags(%q,%q), want user %q and task %q", taskRepo.userID, taskRepo.taskID, tagRepo.userID, tagRepo.taskID, userID, taskID)
	}
	if !reflect.DeepEqual(accesses, []string{"tasks", "get-task", "tags", "list-tags"}) {
		t.Errorf("repository access order = %v, want task lookup before tag listing", accesses)
	}
}

func TestGetTaskUseCaseExecuteReturnsNotFoundForMissingOrUnownedTask(t *testing.T) {
	userID := domain.UserID("requesting-user")
	taskID := domain.TaskID("task-1")
	for _, testCase := range []struct {
		name string
		task dao.Task
		err  error
	}{
		{name: "missing task"},
		{name: "task owned by another user", task: dao.Task{ID: string(taskID), UserID: "other-user"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var accesses []string
			taskRepo := &getTaskTaskRepositoryFake{task: testCase.task, err: testCase.err, accesses: &accesses}
			tagRepo := &getTaskTagRepositoryFake{tags: []dao.TaskTag{{ID: "must-not-be-read"}}, accesses: &accesses}
			uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{
				tasks: taskRepo, tags: tagRepo, accesses: &accesses,
			}}

			got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), userID, taskID)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
			}
			if !reflect.DeepEqual(got, TaskWithTags{}) {
				t.Errorf("Execute() = %#v, want zero result on error", got)
			}
			if tagRepo.calls != 0 {
				t.Errorf("ListByTaskAndUserID() calls = %d, want 0 for missing or unowned task", tagRepo.calls)
			}
			if taskRepo.calls != 1 || taskRepo.userID != userID || taskRepo.taskID != taskID {
				t.Errorf("GetByUserID() = calls %d, user %q, task %q; want 1, %q, %q", taskRepo.calls, taskRepo.userID, taskRepo.taskID, userID, taskID)
			}
		})
	}
}

func TestGetTaskUseCaseExecutePropagatesTagRepositoryError(t *testing.T) {
	wantErr := errors.New("tag query failed")
	var accesses []string
	taskRepo := &getTaskTaskRepositoryFake{task: dao.Task{ID: "task-1", UserID: "user-1"}, accesses: &accesses}
	tagRepo := &getTaskTagRepositoryFake{err: wantErr, accesses: &accesses}
	uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{
		tasks: taskRepo, tags: tagRepo, accesses: &accesses,
	}}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, TaskWithTags{}) {
		t.Errorf("Execute() = %#v, want zero result on error", got)
	}
	if taskRepo.calls != 1 || tagRepo.calls != 1 {
		t.Errorf("repository calls = task:%d tags:%d, want 1 each", taskRepo.calls, tagRepo.calls)
	}
}

func TestGetTaskUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &getTaskUOWFake{err: wantErr}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, TaskWithTags{}) {
		t.Errorf("Execute() = %#v, want zero result on error", got)
	}
	if uow.calls != 1 {
		t.Errorf("UOW calls = %d, want 1", uow.calls)
	}
}

func TestGetTaskUseCaseExecutePropagatesTaskRepositoryError(t *testing.T) {
	wantErr := errors.New("task query failed")
	var accesses []string
	taskRepo := &getTaskTaskRepositoryFake{err: wantErr, accesses: &accesses}
	tagRepo := &getTaskTagRepositoryFake{accesses: &accesses}
	uow := &getTaskUOWFake{repos: getTaskRepositoriesFake{
		tasks: taskRepo, tags: tagRepo, accesses: &accesses,
	}}

	got, err := NewGetTaskUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, TaskWithTags{}) {
		t.Errorf("Execute() = %#v, want zero result on error", got)
	}
	if tagRepo.calls != 0 {
		t.Errorf("ListByTaskAndUserID() calls = %d, want 0 after task lookup failure", tagRepo.calls)
	}
}
