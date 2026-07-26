package usecase

import (
	"context"
	"errors"

	domain "github.com/Najah7/task2todaytodo/internal/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/shared"
)

var (
	ErrPasswordUpdateFailed   = errors.New("password update failed")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUserEmailAlreadyExists = errors.New("user with this email already exists")
	ErrUserIDAlreadyExists    = errors.New("user with this ID already exists")
)

type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{
		repo: repo,
	}
}

func (s *UserService) GetUser(ctx context.Context, userID domain.UserID) (domain.User, error) {
	return s.repo.Get(ctx, userID)
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	e, err := domain.NewEmail(email)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	return s.repo.GetByEmail(ctx, e.String())
}

func (s *UserService) CreateUser(ctx context.Context, idGen func() string, email, password string) (domain.User, error) {
	ID, err := shared.NewID(idGen())
	if err != nil {
		return domain.NewZeroUser(), err
	}

	e, err := domain.NewEmail(email)
	if err != nil {
		return domain.User{}, err
	}

	p, err := domain.NewPassword(password)
	if err != nil {
		return domain.User{}, err
	}

	userID := domain.UserID(ID)
	newUser := domain.NewUser(userID, e, p, domain.NewUserName("", ""))

	u, err := s.repo.GetByEmail(ctx, e.String())
	if err == nil && u.ID != "" {
		return domain.NewZeroUser(), ErrUserEmailAlreadyExists
	}

	u, err = s.repo.Get(ctx, userID)
	if err == nil && u.ID != "" {
		return domain.NewZeroUser(), ErrUserIDAlreadyExists
	}

	return s.repo.Create(ctx, newUser)
}

func (s *UserService) UpdateUserName(ctx context.Context, userID domain.UserID, firstName, lastName string) (domain.User, error) {
	u, err := s.repo.Get(ctx, userID)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	newUser, err := u.UpdateName(domain.NewUserName(firstName, lastName))
	if err != nil {
		return domain.NewZeroUser(), err
	}

	return s.repo.Update(ctx, newUser)
}

func (s *UserService) UpdateUserPassword(ctx context.Context, userID domain.UserID, newPassword string) error {
	p, err := domain.NewPassword(newPassword)
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

func (s *UserService) UpdateUserEmail(ctx context.Context, userID domain.UserID, newEmail string) (domain.User, error) {
	e, err := domain.NewEmail(newEmail)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	u, err := s.repo.GetByEmail(ctx, newEmail)
	if err == nil && u.ID != "" && u.ID != userID {
		return domain.NewZeroUser(), ErrUserEmailAlreadyExists
	}

	user, err := s.repo.Get(ctx, userID)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	newUser := user.UpdateEmail(e)

	return s.repo.Update(ctx, newUser)
}

func (s *UserService) Login(ctx context.Context, email, password string) (domain.User, error) {
	e, err := domain.NewEmail(email)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	p, err := domain.NewPassword(password)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	if !user.Login(e, p) {
		return domain.NewZeroUser(), ErrInvalidCredentials
	}

	return user, nil
}
