package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type updateTaskRepositoryFake struct {
	TaskRepository
	task             dao.Task
	getErr           error
	updateErr        error
	getCalls         int
	updateCalls      int
	lockCalls        int
	getUserID        domain.UserID
	getTaskID        domain.TaskID
	updateUser       domain.UserID
	expectedRevision int32
	updated          domain.Task
	callOrder        []string
}

func (repo *updateTaskRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	repo.getUserID = userID
	repo.getTaskID = taskID
	repo.callOrder = append(repo.callOrder, "get")
	if repo.getErr != nil {
		return dao.Task{}, repo.getErr
	}
	if repo.task.ID == "" || repo.task.ID != string(taskID) || repo.task.UserID != string(userID) {
		return dao.Task{}, ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *updateTaskRepositoryFake) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *updateTaskRepositoryFake) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	repo.lockCalls++
	repo.callOrder = append(repo.callOrder, "lock-task")
	return repo.GetByUserID(ctx, userID, taskID)
}

func (repo *updateTaskRepositoryFake) UpdateByUserID(_ context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error) {
	repo.updateCalls++
	repo.updateUser = userID
	repo.expectedRevision = expectedRevision
	repo.updated = task
	repo.callOrder = append(repo.callOrder, "update")
	if repo.updateErr != nil {
		return dao.Task{}, repo.updateErr
	}
	return dao.Task{ID: string(task.ID), UserID: string(task.UserID), ProjectID: string(task.ProjectID), Title: task.Title, Revision: expectedRevision + 1}, nil
}

func TestUpdateTaskUseCaseExecuteUpdatesSpecifiedFieldAndPreservesProtectedFields(t *testing.T) {
	userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
	newTitle := "Updated title"
	repo := &updateTaskRepositoryFake{task: updateTaskFixture()}

	got, err := NewUpdateTaskUseCase(&updateTaskUOWFake{repo: repo}, updateTaskProgressSource(), nil).Execute(context.Background(), userID, taskID,
		1, PatchField[string]{Present: true, Value: &newTitle}, PatchField[string]{},
		PatchField[time.Time]{}, PatchField[int]{}, PatchField[int]{},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.Title != newTitle {
		t.Errorf("updated title = %q, want %q", got.Title, newTitle)
	}
	if got.Progress != 66 {
		t.Errorf("updated progress = %d, want 66 from progress source", got.Progress)
	}
	if repo.updated.ProjectID != "project-1" || repo.updated.Status.String() != "in_progress" || repo.updated.Progress != 42 {
		t.Errorf("protected fields changed: project=%q status=%q progress=%d", repo.updated.ProjectID, repo.updated.Status.String(), repo.updated.Progress)
	}
	if repo.updated.Description != "Original description" || repo.updated.DueDate.Unix() != 1_800_000_000 || repo.updated.EstimatedMinutes == nil || *repo.updated.EstimatedMinutes != 45 || repo.updated.ActualMinutes == nil || *repo.updated.ActualMinutes != 12 {
		t.Errorf("omitted fields not preserved: %#v", repo.updated)
	}
	if repo.getUserID != userID || repo.getTaskID != taskID || repo.updateUser != userID || repo.lockCalls != 1 || !reflect.DeepEqual(repo.callOrder, []string{"get", "lock-task", "get", "update"}) {
		t.Errorf("repository scope/order = %q/%q/%q %v", repo.getUserID, repo.getTaskID, repo.updateUser, repo.callOrder)
	}
}

func TestUpdateTaskUseCaseExecuteClearsNullableFields(t *testing.T) {
	repo := &updateTaskRepositoryFake{task: updateTaskFixture()}
	_, err := NewUpdateTaskUseCase(&updateTaskUOWFake{repo: repo}, updateTaskProgressSource(), nil).Execute(context.Background(), "user-1", "task-1",
		1, PatchField[string]{}, PatchField[string]{Present: true},
		PatchField[time.Time]{Present: true}, PatchField[int]{Present: true}, PatchField[int]{Present: true},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.updated.Description != "" || !repo.updated.DueDate.IsZero() || repo.updated.EstimatedMinutes != nil || repo.updated.ActualMinutes != nil {
		t.Errorf("nullable fields not cleared: description=%q due=%v estimate=%v actual=%v", repo.updated.Description, repo.updated.DueDate, repo.updated.EstimatedMinutes, repo.updated.ActualMinutes)
	}
}

func TestUpdateTaskUseCaseExecuteRejectsInvalidInputBeforeUpdate(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*updateTaskRepositoryFake) error
		want  error
	}{
		{
			name: "null title",
			apply: func(repo *updateTaskRepositoryFake) error {
				_, err := updateTask(repo, PatchField[string]{Present: true}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[int]{}, PatchField[int]{})
				return err
			},
			want: ErrTaskPatchRequiredFieldNull,
		},
		{
			name: "negative estimate",
			apply: func(repo *updateTaskRepositoryFake) error {
				negative := -1
				_, err := updateTask(repo, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[int]{Present: true, Value: &negative}, PatchField[int]{})
				return err
			},
			want: domain.ErrTaskEstimatedMinutesInvalid,
		},
		{
			name: "negative actual",
			apply: func(repo *updateTaskRepositoryFake) error {
				negative := -1
				_, err := updateTask(repo, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[int]{}, PatchField[int]{Present: true, Value: &negative})
				return err
			},
			want: domain.ErrTaskActualMinutesInvalid,
		},
		{
			name: "blank title",
			apply: func(repo *updateTaskRepositoryFake) error {
				blank := " \t"
				_, err := updateTask(repo, PatchField[string]{Present: true, Value: &blank}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[int]{}, PatchField[int]{})
				return err
			},
			want: domain.ErrTaskTitleEmpty,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &updateTaskRepositoryFake{task: updateTaskFixture()}
			if err := test.apply(repo); !errors.Is(err, test.want) {
				t.Errorf("Execute() error = %v, want %v", err, test.want)
			}
			if repo.updateCalls != 0 {
				t.Errorf("UpdateByUserID() calls = %d, want 0", repo.updateCalls)
			}
		})
	}
}

func TestUpdateTaskUseCaseExecuteReturnsNotFoundAndRepositoryErrors(t *testing.T) {
	getErr, updateErr := errors.New("get failed"), errors.New("update failed")
	tests := []struct {
		name string
		repo *updateTaskRepositoryFake
		want error
	}{
		{name: "missing task", repo: &updateTaskRepositoryFake{}, want: ErrTaskNotFound},
		{name: "task belongs to another user", repo: &updateTaskRepositoryFake{task: updateTaskFixtureForUser("other-user")}, want: ErrTaskNotFound},
		{name: "get failure", repo: &updateTaskRepositoryFake{getErr: getErr}, want: getErr},
		{name: "update failure", repo: &updateTaskRepositoryFake{task: updateTaskFixture(), updateErr: updateErr}, want: updateErr},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := updateTask(test.repo, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}, PatchField[int]{}, PatchField[int]{})
			if !errors.Is(err, test.want) {
				t.Errorf("Execute() error = %v, want %v", err, test.want)
			}
			if test.name != "update failure" && test.repo.updateCalls != 0 {
				t.Errorf("UpdateByUserID() calls = %d, want 0", test.repo.updateCalls)
			}
		})
	}
}

func updateTask(
	repo *updateTaskRepositoryFake,
	title, description PatchField[string],
	dueDate PatchField[time.Time],
	estimatedMinutes, actualMinutes PatchField[int],
) (dao.Task, error) {
	uow := &updateTaskUOWFake{repo: repo}
	return NewUpdateTaskUseCase(uow, updateTaskProgressSource(), nil).Execute(context.Background(), "user-1", "task-1", 1, title, description, dueDate, estimatedMinutes, actualMinutes)
}

type updateTaskUOWFake struct {
	repo *updateTaskRepositoryFake
}

func (uow *updateTaskUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, updateTaskRepositoriesFake{repo: uow.repo})
}

type updateTaskRepositoriesFake struct {
	taskProgressTestRepositories
	repo *updateTaskRepositoryFake
}

func (repos updateTaskRepositoriesFake) Tasks() TaskRepository { return repos.repo }

func updateTaskProgressSource() *taskProgressSourceFake {
	return &taskProgressSourceFake{sources: dao.TaskProgressSources{
		Counts: map[string]dao.TaskProgressCounts{"task-1": {Total: 3, Completed: 2}},
	}}
}

func updateTaskFixture() dao.Task {
	return updateTaskFixtureForUser("user-1")
}

func updateTaskFixtureForUser(userID string) dao.Task {
	return dao.Task{
		ID: "task-1", UserID: userID, AssigneeID: userID, ProjectID: "project-1",
		Title: "Original title", Description: "Original description", DueDate: 1_800_000_000,
		EstimatedMinutes: updateTaskIntPointer(45), ActualMinutes: updateTaskIntPointer(12), Progress: 42,
		Priority: dao.Priority{Value: "high"}, Status: dao.TaskStatus{Value: "in_progress"},
		CreatedAt: 100, UpdatedAt: 200, Revision: 1,
	}
}

func updateTaskIntPointer(value int) *int { return &value }
