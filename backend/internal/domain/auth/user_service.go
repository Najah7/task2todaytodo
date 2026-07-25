package auth

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
)

var (
	ErrPasswordUpdateFailed   = errors.New("password update failed")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUserEmailAlreadyExists = errors.New("user with this email already exists")
	ErrUserIDAlreadyExists    = errors.New("user with this ID already exists")
)

type UserRepository interface {
	Get(ctx context.Context, id UserID) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, user User) (User, error)
	Update(ctx context.Context, user User) (User, error)
}

type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{
		repo: repo,
	}
}

func (s *UserService) GetUser(ctx context.Context, userID UserID) (User, error) {
	return s.repo.Get(ctx, userID)
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (User, error) {
	e, err := NewEmail(email)
	if err != nil {
		return NewZeroUser(), err
	}

	return s.repo.GetByEmail(ctx, e.String())
}

func (s *UserService) CreateUser(ctx context.Context, idGen func() string, email, password string) (User, error) {
	ID, err := shared.NewID(idGen())
	if err != nil {
		return NewZeroUser(), err
	}

	e, err := NewEmail(email)
	if err != nil {
		return User{}, err
	}

	p, err := NewPassword(password)
	if err != nil {
		return User{}, err
	}

	userID := UserID(ID)
	newUser := NewUser(userID, e, p, NewUserName("", ""))

	u, err := s.repo.GetByEmail(ctx, e.String())
	if err == nil && u.ID != "" {
		return NewZeroUser(), ErrUserEmailAlreadyExists
	}

	u, err = s.repo.Get(ctx, userID)
	if err == nil && u.ID != "" {
		return NewZeroUser(), ErrUserIDAlreadyExists
	}

	return s.repo.Create(ctx, newUser)
}

func (s *UserService) UpdateUserName(ctx context.Context, userID UserID, firstName, lastName string) (User, error) {
	u, err := s.repo.Get(ctx, userID)
	if err != nil {
		return NewZeroUser(), err
	}

	newUser, err := u.UpdateName(NewUserName(firstName, lastName))
	if err != nil {
		return NewZeroUser(), err
	}

	return s.repo.Update(ctx, newUser)
}

func (s *UserService) UpdateUserPassword(ctx context.Context, userID UserID, newPassword string) error {
	p, err := NewPassword(newPassword)
	if err != nil {
		return err
	}

	user, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}

	newUser := user.UpdatePassword(p)

	u, err := s.repo.Update(ctx, newUser)
	if err != nil {
		return err
	}

	if u.Password != newUser.Password {
		return ErrPasswordUpdateFailed
	}

	return nil
}

func (s *UserService) UpdateUserEmail(ctx context.Context, userID UserID, newEmail string) (User, error) {
	e, err := NewEmail(newEmail)
	if err != nil {
		return NewZeroUser(), err
	}

	u, err := s.repo.GetByEmail(ctx, newEmail)
	if err == nil && u.ID != "" && u.ID != userID {
		return NewZeroUser(), ErrUserEmailAlreadyExists
	}

	user, err := s.repo.Get(ctx, userID)
	if err != nil {
		return NewZeroUser(), err
	}

	newUser := user.UpdateEmail(e)

	return s.repo.Update(ctx, newUser)
}

func (s *UserService) Login(ctx context.Context, email, password string) (User, error) {
	e, err := NewEmail(email)
	if err != nil {
		return NewZeroUser(), err
	}

	p, err := NewPassword(password)
	if err != nil {
		return NewZeroUser(), err
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return NewZeroUser(), err
	}

	if !user.Login(e, p) {
		return NewZeroUser(), ErrInvalidCredentials
	}

	return user, nil
}
