package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type deleteTaskTagRepositoryFake struct {
	err         error
	userID      domain.UserID
	tagID       domain.TaskTagID
	calls       int
	tasks       map[domain.TaskID]bool
	assignments map[domain.TaskID]map[domain.TaskTagID]bool
}

func (repo *deleteTaskTagRepositoryFake) DeleteByUserID(_ context.Context, userID domain.UserID, tagID domain.TaskTagID) error {
	repo.calls++
	repo.userID = userID
	repo.tagID = tagID
	if repo.err != nil {
		return repo.err
	}

	for taskID, tags := range repo.assignments {
		delete(tags, tagID)
		if len(tags) == 0 {
			delete(repo.assignments, taskID)
		}
	}
	return nil
}

func TestDeleteTaskTagUseCaseDeletesOwnedTagAndOnlyClearsAssignments(t *testing.T) {
	userID := domain.UserID("user-1")
	tagID := domain.TaskTagID("tag-1")
	taskID := domain.TaskID("task-1")
	otherTaskID := domain.TaskID("task-2")
	otherTagID := domain.TaskTagID("tag-2")
	repo := &deleteTaskTagRepositoryFake{
		tasks: map[domain.TaskID]bool{taskID: true, otherTaskID: true},
		assignments: map[domain.TaskID]map[domain.TaskTagID]bool{
			taskID:      {tagID: true, otherTagID: true},
			otherTaskID: {tagID: true},
		},
	}

	err := NewDeleteTaskTagUseCase(repo, nil).Execute(context.Background(), userID, tagID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.calls != 1 || repo.userID != userID || repo.tagID != tagID {
		t.Fatalf("DeleteByUserID() = calls %d, user %q, tag %q; want 1, %q, %q", repo.calls, repo.userID, repo.tagID, userID, tagID)
	}
	if !repo.tasks[taskID] || !repo.tasks[otherTaskID] {
		t.Errorf("tasks after deleting tag = %#v, want both tasks preserved", repo.tasks)
	}
	if got := repo.assignments[taskID]; len(got) != 1 || !got[otherTagID] {
		t.Errorf("first task assignments = %#v, want only tag-2", got)
	}
	if got := repo.assignments[otherTaskID]; len(got) != 0 {
		t.Errorf("second task assignments = %#v, want none", got)
	}
}

func TestDeleteTaskTagUseCaseReturnsNotFoundForMissingOrUnownedTag(t *testing.T) {
	for _, name := range []string{"missing tag", "tag belongs to another user"} {
		t.Run(name, func(t *testing.T) {
			repo := &deleteTaskTagRepositoryFake{err: ErrTaskTagNotFound}
			err := NewDeleteTaskTagUseCase(repo, nil).Execute(context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"))
			if !errors.Is(err, ErrTaskTagNotFound) {
				t.Errorf("Execute() error = %v, want %v", err, ErrTaskTagNotFound)
			}
			if repo.calls != 1 {
				t.Errorf("DeleteByUserID() calls = %d, want 1", repo.calls)
			}
		})
	}
}

func TestDeleteTaskTagUseCasePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &deleteTaskTagRepositoryFake{err: wantErr}

	err := NewDeleteTaskTagUseCase(repo, nil).Execute(context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"))
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.calls != 1 {
		t.Errorf("DeleteByUserID() calls = %d, want 1", repo.calls)
	}
}
