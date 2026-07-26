package repositories

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/domain/auth"
	"github.com/Najah7/task2todaytodo/internal/domain/shared"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	"github.com/jackc/pgx/v5"
)

var _ auth.UserRepository = UserRepository{}

type UserRepository struct {
	queries *sqlc.Queries
}

func NewUserRepository(db sqlc.DBTX) *UserRepository {
	queries := sqlc.New(db)
	return &UserRepository{
		queries: queries,
	}
}

func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{
		queries: r.queries.WithTx(tx),
	}
}

func recordToUser(record sqlc.User) (auth.User, error) {
	e, err := auth.NewEmail(record.Email)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	p, err := auth.NewHashedPassword(record.Password)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	ID, err := shared.NewID(record.ID)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	name := auth.NewUserName(record.FirstName, record.LastName)
	return auth.NewUser(auth.UserID(ID), e, p, name), nil
}

func (r UserRepository) Get(ctx context.Context, id auth.UserID) (auth.User, error) {
	u, err := r.queries.GetUser(ctx, string(id))
	if err != nil {
		return auth.NewZeroUser(), err
	}

	user, err := recordToUser(u)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	return user, nil
}

func (r UserRepository) GetByEmail(ctx context.Context, email string) (auth.User, error) {
	u, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	user, err := recordToUser(u)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	return user, nil
}

func (r UserRepository) Create(ctx context.Context, user auth.User) (auth.User, error) {
	u, err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        string(user.ID),
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
	})
	if err != nil {
		return auth.NewZeroUser(), err
	}

	user, err = recordToUser(u)
	if err != nil {
		return auth.NewZeroUser(), err
	}

	return user, nil
}

func (r UserRepository) Update(ctx context.Context, user auth.User) (auth.User, error) {
	u, err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        string(user.ID),
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
	})
	if err != nil {
		return auth.NewZeroUser(), err
	}

	user, err = recordToUser(u)
	if err != nil {
		return auth.NewZeroUser(), err
	}
	return user, nil
}
