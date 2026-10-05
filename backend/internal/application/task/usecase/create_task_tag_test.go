package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type createTaskTagRepositoryFake struct {
	tagsByUserIDAndName map[string]dao.TaskTag
	created             domain.TaskTag
	result              dao.TaskTag
	err                 error
	calls               int
}

func (repo *createTaskTagRepositoryFake) Create(_ context.Context, tag domain.TaskTag) (dao.TaskTag, error) {
	repo.calls++
	repo.created = tag
	if repo.err != nil {
		return dao.TaskTag{}, repo.err
	}

	key := string(tag.UserID) + ":" + strings.ToLower(tag.Name)
	if _, exists := repo.tagsByUserIDAndName[key]; exists {
		return dao.TaskTag{}, ErrTaskTagNameConflict
	}
	if repo.tagsByUserIDAndName == nil {
		repo.tagsByUserIDAndName = make(map[string]dao.TaskTag)
	}
	if repo.result.ID == "" {
		repo.result = dao.TaskTag{ID: string(tag.ID), UserID: string(tag.UserID), Name: tag.Name}
	}
	repo.tagsByUserIDAndName[key] = repo.result
	return repo.result, nil
}

func validCreateTaskTagInput() CreateTaskTagInput {
	return CreateTaskTagInput{ID: "tag-1", UserID: "user-1", Name: "  Research  "}
}

func TestCreateTaskTagUseCaseCreatesTrimmedTagForOwner(t *testing.T) {
	want := dao.TaskTag{ID: "tag-1", UserID: "user-1", Name: "Research"}
	repo := &createTaskTagRepositoryFake{result: want}

	got, err := NewCreateTaskTagUseCase(repo, nil).Execute(context.Background(), validCreateTaskTagInput())
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
	if repo.calls != 1 {
		t.Fatalf("Create() calls = %d, want 1", repo.calls)
	}
	if repo.created.ID != domain.TaskTagID("tag-1") || repo.created.UserID != domain.UserID("user-1") || repo.created.Name != "Research" {
		t.Errorf("created tag = (%q, %q, %q), want (tag-1, user-1, Research)", repo.created.ID, repo.created.UserID, repo.created.Name)
	}
}

func TestCreateTaskTagUseCaseRejectsBlankNameBeforeRepository(t *testing.T) {
	input := validCreateTaskTagInput()
	input.Name = " \t\n "
	repo := &createTaskTagRepositoryFake{}

	_, err := NewCreateTaskTagUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrTaskTagNameEmpty) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrTaskTagNameEmpty)
	}
	if repo.calls != 0 {
		t.Errorf("Create() calls = %d, want 0", repo.calls)
	}
}

func TestCreateTaskTagUseCaseReturnsCaseInsensitiveNameConflict(t *testing.T) {
	repo := &createTaskTagRepositoryFake{tagsByUserIDAndName: map[string]dao.TaskTag{
		"user-1:research": {ID: "existing", UserID: "user-1", Name: "Research"},
	}}
	input := validCreateTaskTagInput()
	input.Name = "research"

	_, err := NewCreateTaskTagUseCase(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, ErrTaskTagNameConflict) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskTagNameConflict)
	}
	if repo.calls != 1 {
		t.Errorf("Create() calls = %d, want 1", repo.calls)
	}
}

func TestCreateTaskTagUseCaseAllowsSameNameForDifferentUsers(t *testing.T) {
	repo := &createTaskTagRepositoryFake{tagsByUserIDAndName: map[string]dao.TaskTag{
		"user-1:research": {ID: "existing", UserID: "user-1", Name: "Research"},
	}}
	input := validCreateTaskTagInput()
	input.ID = "tag-2"
	input.UserID = "user-2"
	input.Name = "research"
	repo.result = dao.TaskTag{ID: "tag-2", UserID: "user-2", Name: "research"}

	got, err := NewCreateTaskTagUseCase(repo, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.UserID != "user-2" || repo.created.UserID != "user-2" {
		t.Errorf("created tag owner = %q (persisted %q), want user-2", repo.created.UserID, got.UserID)
	}
	if repo.calls != 1 {
		t.Errorf("Create() calls = %d, want 1", repo.calls)
	}
}

func TestCreateTaskTagUseCasePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &createTaskTagRepositoryFake{err: wantErr}

	_, err := NewCreateTaskTagUseCase(repo, nil).Execute(context.Background(), validCreateTaskTagInput())
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.calls != 1 {
		t.Errorf("Create() calls = %d, want 1", repo.calls)
	}
}
