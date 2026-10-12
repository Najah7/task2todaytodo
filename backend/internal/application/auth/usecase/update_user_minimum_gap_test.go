package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

type minimumGapTestRepository struct {
	UserRepository
	user    dao.User
	updated int
	writes  int
}

func (r *minimumGapTestRepository) Get(context.Context, domain.UserID) (dao.User, error) {
	return r.user, nil
}

func (r *minimumGapTestRepository) UpdateMinimumGapMinutes(_ context.Context, _ domain.UserID, minutes int) error {
	r.updated = minutes
	r.writes++
	r.user.MinimumGapMinutes = minutes
	return nil
}

func TestUpdateUserMinimumGapAcceptsBoundsAndReturnsSavedValue(t *testing.T) {
	for _, minutes := range []int{0, 5, 1440} {
		t.Run(fmt.Sprint(minutes), func(t *testing.T) {
			repo := &minimumGapTestRepository{user: dao.User{ID: "user-1", MinimumGapMinutes: 5}}
			result, err := NewUpdateUserMinimumGapUseCase(repo, nil).Execute(context.Background(), "user-1", minutes)
			if err != nil {
				t.Fatal(err)
			}
			if repo.updated != minutes || result.MinimumGapMinutes != minutes {
				t.Fatalf("saved/returned gap = %d/%d, want %d", repo.updated, result.MinimumGapMinutes, minutes)
			}
		})
	}
}

func TestUpdateUserMinimumGapRejectsOutOfRangeBeforeWriting(t *testing.T) {
	for _, minutes := range []int{-1, 1441} {
		repo := &minimumGapTestRepository{}
		if _, err := NewUpdateUserMinimumGapUseCase(repo, nil).Execute(context.Background(), "user-1", minutes); err != ErrInvalidMinimumGapMinutes {
			t.Fatalf("Execute(%d) error = %v", minutes, err)
		}
		if repo.writes != 0 {
			t.Fatalf("invalid gap %d was written", minutes)
		}
	}
}
