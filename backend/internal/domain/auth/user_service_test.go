package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

var (
	errGetUser        = errors.New("get user failed")
	errGetUserByEmail = errors.New("get user by email failed")
	errCreateUser     = errors.New("create user failed")
	errUpdateUser     = errors.New("update user failed")
)

func TestUserServiceGetUser(t *testing.T) {
	want := existingUser(t)
	svc := NewUserService(&stubUserRepository{user: want})

	got, err := svc.GetUser(context.Background(), want.ID)
	assertServiceErrorIs(t, err, nil)
	assertUserEqual(t, got, want)
}

func TestUserServiceGetUserByEmail(t *testing.T) {
	want := existingUser(t)
	svc := NewUserService(&stubUserRepository{userByEmail: want})

	got, err := svc.GetUserByEmail(context.Background(), want.Email.String())
	assertServiceErrorIs(t, err, nil)
	assertUserEqual(t, got, want)
}

func TestUserServiceCreateUser(t *testing.T) {
	svc := NewUserService(&stubUserRepository{})

	got, err := svc.CreateUser(context.Background(), func() string { return "new-user" }, "new@example.com", "Password1!")
	assertServiceErrorIs(t, err, nil)
	if got.ID != "new-user" || got.Email.String() != "new@example.com" || !got.Password.IsHashed {
		t.Errorf("created user = %+v, want a persisted, hashed user", got)
	}
}

func TestUserServiceCreateUserRejectsDuplicates(t *testing.T) {
	tests := []struct {
		name string
		repo *stubUserRepository
		want error
	}{
		{name: "email", repo: &stubUserRepository{userByEmail: existingUser(t)}, want: ErrUserEmailAlreadyExists},
		{name: "ID", repo: &stubUserRepository{user: existingUser(t)}, want: ErrUserIDAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUserService(tt.repo).CreateUser(context.Background(), func() string { return "user-1" }, "new@example.com", "Password1!")
			assertServiceErrorIs(t, err, tt.want)
			assertUserEqual(t, got, NewZeroUser())
		})
	}
}

func TestUserServicePropagatesRepositoryErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(*UserService) error
		repo *stubUserRepository
		want error
	}{
		{
			name: "get user",
			run:  func(s *UserService) error { _, err := s.GetUser(context.Background(), "user-1"); return err },
			repo: &stubUserRepository{getErr: errGetUser}, want: errGetUser,
		},
		{
			name: "get user by email",
			run: func(s *UserService) error {
				_, err := s.GetUserByEmail(context.Background(), "user@example.com")
				return err
			},
			repo: &stubUserRepository{getByEmailErr: errGetUserByEmail}, want: errGetUserByEmail,
		},
		{
			name: "create user",
			run: func(s *UserService) error {
				_, err := s.CreateUser(context.Background(), func() string { return "new-user" }, "new@example.com", "Password1!")
				return err
			},
			repo: &stubUserRepository{createErr: errCreateUser}, want: errCreateUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertServiceErrorIs(t, tt.run(NewUserService(tt.repo)), tt.want)
		})
	}
}

func TestUserServiceUpdateUserName(t *testing.T) {
	user := existingUser(t)
	got, err := NewUserService(&stubUserRepository{user: user}).UpdateUserName(context.Background(), user.ID, "Jane", "Doe")
	assertServiceErrorIs(t, err, nil)
	if got.FullName() != "Jane Doe" {
		t.Errorf("FullName() = %q, want %q", got.FullName(), "Jane Doe")
	}
}

func TestUserServiceUpdateUserPassword(t *testing.T) {
	user := existingUser(t)
	err := NewUserService(&stubUserRepository{user: user}).UpdateUserPassword(context.Background(), user.ID, "NewPassword1!")
	assertServiceErrorIs(t, err, nil)
}

func TestUserServiceUpdateUserPasswordDetectsUnexpectedPersistedValue(t *testing.T) {
	user := existingUser(t)
	err := NewUserService(&stubUserRepository{user: user, updateResult: user}).UpdateUserPassword(context.Background(), user.ID, "NewPassword1!")
	assertServiceErrorIs(t, err, ErrPasswordUpdateFailed)
}

func TestUserServiceUpdateUserEmail(t *testing.T) {
	user := existingUser(t)
	got, err := NewUserService(&stubUserRepository{user: user}).UpdateUserEmail(context.Background(), user.ID, "new@example.com")
	assertServiceErrorIs(t, err, nil)
	if got.Email.String() != "new@example.com" {
		t.Errorf("email = %q, want %q", got.Email, "new@example.com")
	}
}

func TestUserServiceUpdateUserEmailRejectsAnotherUsersEmail(t *testing.T) {
	user := existingUser(t)
	got, err := NewUserService(&stubUserRepository{user: user, userByEmail: user}).UpdateUserEmail(context.Background(), "other-user", "user@example.com")
	assertServiceErrorIs(t, err, ErrUserEmailAlreadyExists)
	assertUserEqual(t, got, NewZeroUser())
}

func TestUserServiceLogin(t *testing.T) {
	user := existingUser(t)
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "success", password: "Password1!"},
		{name: "wrong password", password: "WrongPassword1!", wantErr: ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUserService(&stubUserRepository{userByEmail: user}).Login(context.Background(), "user@example.com", tt.password)
			assertServiceErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil {
				assertUserEqual(t, got, user)
			}
		})
	}
}

var _ UserRepository = (*stubUserRepository)(nil)

type stubUserRepository struct {
	user          User
	userByEmail   User
	updateResult  User
	getErr        error
	getByEmailErr error
	createErr     error
	updateErr     error
}

func (r *stubUserRepository) Get(_ context.Context, _ UserID) (User, error) {
	if r.getErr != nil {
		return NewZeroUser(), r.getErr
	}
	return r.user, nil
}

func (r *stubUserRepository) GetByEmail(_ context.Context, _ string) (User, error) {
	if r.getByEmailErr != nil {
		return NewZeroUser(), r.getByEmailErr
	}
	return r.userByEmail, nil
}

func (r *stubUserRepository) Create(_ context.Context, user User) (User, error) {
	if r.createErr != nil {
		return NewZeroUser(), r.createErr
	}
	return user, nil
}

func (r *stubUserRepository) Update(_ context.Context, user User) (User, error) {
	if r.updateErr != nil {
		return NewZeroUser(), r.updateErr
	}
	if !r.updateResult.IsZero() {
		return r.updateResult, nil
	}
	return user, nil
}

func existingUser(t *testing.T) User {
	t.Helper()
	id, err := shared.NewID("user-1")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	email, err := NewEmail("user@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error = %v", err)
	}
	password, err := NewPassword("Password1!")
	if err != nil {
		t.Fatalf("NewPassword() error = %v", err)
	}
	return NewUser(UserID(id), email, password, NewUserName("John", "Doe"))
}

func assertUserEqual(t *testing.T, got, want User) {
	t.Helper()
	if got != want {
		t.Errorf("user = %+v, want %+v", got, want)
	}
}

func assertServiceErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}
