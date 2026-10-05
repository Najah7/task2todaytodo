package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type renameTaskTagRepositoryFake struct {
	tag          dao.TaskTag
	getErr       error
	renameErr    error
	getCalls     int
	renameCalls  int
	getUserID    domain.UserID
	getTagID     domain.TaskTagID
	renameUserID domain.UserID
	renamed      domain.TaskTag
	callOrder    []string
}

func (repo *renameTaskTagRepositoryFake) GetByUserID(_ context.Context, userID domain.UserID, tagID domain.TaskTagID) (dao.TaskTag, error) {
	repo.getCalls++
	repo.getUserID = userID
	repo.getTagID = tagID
	repo.callOrder = append(repo.callOrder, "get")
	if repo.getErr != nil {
		return dao.TaskTag{}, repo.getErr
	}
	if repo.tag.ID == "" || repo.tag.ID != string(tagID) || repo.tag.UserID != string(userID) {
		return dao.TaskTag{}, ErrTaskTagNotFound
	}
	return repo.tag, nil
}

func (repo *renameTaskTagRepositoryFake) RenameByUserID(_ context.Context, userID domain.UserID, tag domain.TaskTag) (dao.TaskTag, error) {
	repo.renameCalls++
	repo.renameUserID = userID
	repo.renamed = tag
	repo.callOrder = append(repo.callOrder, "rename")
	if repo.renameErr != nil {
		return dao.TaskTag{}, repo.renameErr
	}
	return dao.TaskTag{
		ID: string(tag.ID), UserID: string(tag.UserID), Name: tag.Name,
		CreatedAt: tag.CreatedAt.Unix(), UpdatedAt: tag.UpdatedAt.Unix(),
	}, nil
}

func TestRenameTaskTagUseCaseRenamesOwnedTagAndTrimsName(t *testing.T) {
	repo := &renameTaskTagRepositoryFake{tag: taskTagFixture()}
	got, err := NewRenameTaskTagUseCase(repo, nil).Execute(
		context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"), "  planning  ",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.ID != "tag-1" || got.UserID != "user-1" || got.Name != "planning" {
		t.Errorf("Execute() = %#v, want owned tag renamed to planning", got)
	}
	if repo.getUserID != "user-1" || repo.getTagID != "tag-1" || repo.renameUserID != "user-1" {
		t.Errorf("repository IDs = get(%q, %q), rename(%q)", repo.getUserID, repo.getTagID, repo.renameUserID)
	}
	if repo.renamed.Name != "planning" || repo.renamed.CreatedAt.Unix() != 100 || repo.renamed.UpdatedAt.Unix() != 200 {
		t.Errorf("renamed domain tag = %#v, want normalized name and persisted timestamps", repo.renamed)
	}
	if len(repo.callOrder) != 2 || repo.callOrder[0] != "get" || repo.callOrder[1] != "rename" {
		t.Errorf("repository call order = %v, want [get rename]", repo.callOrder)
	}
}

func TestRenameTaskTagUseCaseRejectsWhitespaceName(t *testing.T) {
	repo := &renameTaskTagRepositoryFake{tag: taskTagFixture()}
	_, err := NewRenameTaskTagUseCase(repo, nil).Execute(
		context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"), " \t ",
	)
	if !errors.Is(err, domain.ErrTaskTagNameEmpty) {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrTaskTagNameEmpty)
	}
	if repo.renameCalls != 0 {
		t.Errorf("RenameByUserID() calls = %d, want 0", repo.renameCalls)
	}
}

func TestRenameTaskTagUseCaseReturnsNotFoundForTagOwnedByAnotherUser(t *testing.T) {
	repo := &renameTaskTagRepositoryFake{tag: taskTagFixtureForUser("other-user")}
	_, err := NewRenameTaskTagUseCase(repo, nil).Execute(
		context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"), "planning",
	)
	if !errors.Is(err, ErrTaskTagNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskTagNotFound)
	}
	if repo.renameCalls != 0 {
		t.Errorf("RenameByUserID() calls = %d, want 0", repo.renameCalls)
	}
}

func TestRenameTaskTagUseCaseReturnsNameConflict(t *testing.T) {
	repo := &renameTaskTagRepositoryFake{tag: taskTagFixture(), renameErr: ErrTaskTagNameConflict}
	_, err := NewRenameTaskTagUseCase(repo, nil).Execute(
		context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"), "planning",
	)
	if !errors.Is(err, ErrTaskTagNameConflict) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskTagNameConflict)
	}
}

func TestRenameTaskTagUseCasePropagatesRepositoryFailures(t *testing.T) {
	getErr := errors.New("get failed")
	renameErr := errors.New("rename failed")
	for _, test := range []struct {
		name string
		repo *renameTaskTagRepositoryFake
		want error
	}{
		{name: "get", repo: &renameTaskTagRepositoryFake{getErr: getErr}, want: getErr},
		{name: "rename", repo: &renameTaskTagRepositoryFake{tag: taskTagFixture(), renameErr: renameErr}, want: renameErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRenameTaskTagUseCase(test.repo, nil).Execute(
				context.Background(), domain.UserID("user-1"), domain.TaskTagID("tag-1"), "planning",
			)
			if !errors.Is(err, test.want) {
				t.Errorf("Execute() error = %v, want %v", err, test.want)
			}
		})
	}
}

func taskTagFixture() dao.TaskTag { return taskTagFixtureForUser("user-1") }

func taskTagFixtureForUser(userID string) dao.TaskTag {
	return dao.TaskTag{ID: "tag-1", UserID: userID, Name: "work", CreatedAt: 100, UpdatedAt: 200}
}
