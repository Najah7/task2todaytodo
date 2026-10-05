package usecase

import (
	"context"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

type timezoneUpdateRepository struct {
	*stubUserRepository
	updatedID       domain.UserID
	updatedTimezone domain.UserTimezone
	updateErr       error
}

func (r *timezoneUpdateRepository) UpdateTimezone(_ context.Context, id domain.UserID, timezone domain.UserTimezone) error {
	r.updatedID = id
	r.updatedTimezone = timezone
	return r.updateErr
}

func TestUpdateUserTimezoneUseCaseExecute(t *testing.T) {
	user := existingUser(t)
	repo := &timezoneUpdateRepository{stubUserRepository: &stubUserRepository{user: user}}
	got, err := NewUpdateUserTimezoneUseCase(repo, nil).Execute(context.Background(), user.ID, "America/Los_Angeles")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Timezone != "America/Los_Angeles" || got.Password != "" {
		t.Errorf("user = %+v, want timezone set and password omitted", got)
	}
	if repo.updatedID != user.ID || repo.updatedTimezone.String() != "America/Los_Angeles" {
		t.Errorf("UpdateTimezone() = (%q, %q), want (%q, %q)", repo.updatedID, repo.updatedTimezone, user.ID, "America/Los_Angeles")
	}
}

func TestUpdateUserTimezoneUseCaseRejectsInvalidTimezone(t *testing.T) {
	user := existingUser(t)
	repo := &timezoneUpdateRepository{stubUserRepository: &stubUserRepository{user: user}}
	_, err := NewUpdateUserTimezoneUseCase(repo, nil).Execute(context.Background(), user.ID, "Mars/Olympus")
	if err != domain.ErrInvalidUserTimezone {
		t.Errorf("Execute() error = %v, want %v", err, domain.ErrInvalidUserTimezone)
	}
	if repo.updatedTimezone != "" {
		t.Errorf("UpdateTimezone() called with %q for invalid input", repo.updatedTimezone)
	}
}

func TestUpdateUserTimezoneUseCasePropagatesRepositoryErrors(t *testing.T) {
	user := existingUser(t)
	repo := &timezoneUpdateRepository{stubUserRepository: &stubUserRepository{user: user}, updateErr: errUpdateUser}
	_, err := NewUpdateUserTimezoneUseCase(repo, nil).Execute(context.Background(), user.ID, "America/Los_Angeles")
	assertServiceErrorIs(t, err, errUpdateUser)
}
