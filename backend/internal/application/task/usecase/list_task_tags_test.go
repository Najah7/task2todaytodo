package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type listTaskTagsRepository struct {
	tagsByUserID map[domain.UserID][]dao.TaskTag
	err          error
	gotUserID    domain.UserID
}

func (repo *listTaskTagsRepository) ListByUserID(_ context.Context, userID domain.UserID) ([]dao.TaskTag, error) {
	repo.gotUserID = userID
	if repo.err != nil {
		return nil, repo.err
	}
	return repo.tagsByUserID[userID], nil
}

func TestListTaskTagsUseCaseExecute(t *testing.T) {
	userID := domain.UserID("user-1")
	want := []dao.TaskTag{
		{ID: "tag-2", UserID: string(userID), Name: "Alpha"},
		{ID: "tag-1", UserID: string(userID), Name: "alpha"},
	}
	repo := &listTaskTagsRepository{tagsByUserID: map[domain.UserID][]dao.TaskTag{
		userID:                      want,
		domain.UserID("other-user"): {{ID: "tag-other", UserID: "other-user", Name: "Other"}},
	}}

	got, err := NewListTaskTagsUseCase(repo, nil).Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.gotUserID != userID {
		t.Errorf("repository user ID = %q, want authenticated user ID %q", repo.gotUserID, userID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Execute() = %#v, want %#v", got, want)
	}
}

func TestListTaskTagsUseCaseReturnsEmptyList(t *testing.T) {
	userID := domain.UserID("user-without-tags")
	repo := &listTaskTagsRepository{tagsByUserID: map[domain.UserID][]dao.TaskTag{
		userID: {},
	}}

	got, err := NewListTaskTagsUseCase(repo, nil).Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("Execute() = %#v, want empty list", got)
	}
	if repo.gotUserID != userID {
		t.Errorf("repository user ID = %q, want authenticated user ID %q", repo.gotUserID, userID)
	}
}

func TestListTaskTagsUseCasePropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &listTaskTagsRepository{err: wantErr}

	got, err := NewListTaskTagsUseCase(repo, nil).Execute(context.Background(), domain.UserID("user-1"))
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil list on error", got)
	}
}
