package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
)

var _ usecase.UserRepository = UserRepository{}

type UserRepository struct {
	queries *sqlc.Queries
	db      sqlc.DBTX
}

func NewUserRepository(db sqlc.DBTX) *UserRepository {
	queries := sqlc.New(db)
	return &UserRepository{
		queries: queries,
		db:      db,
	}
}

func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{
		queries: r.queries.WithTx(tx),
		db:      tx,
	}
}

func (r UserRepository) GetTimezone(ctx context.Context, userID string) (string, error) {
	return r.queries.GetUserTimezone(ctx, userID)
}

func recordToUserDAO(record sqlc.User) dao.User {
	return dao.User{
		ID:        record.ID,
		FirstName: record.FirstName,
		LastName:  record.LastName,
		Email:     record.Email,
		Password:  record.Password,
		CreatedAt: pgTimeUnix(record.CreatedAt),
		UpdatedAt: pgTimeUnix(record.UpdatedAt),
	}
}

func (r UserRepository) Get(ctx context.Context, id domain.UserID) (dao.User, error) {
	u, err := r.queries.GetUser(ctx, string(id))
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	result.Timezone, err = r.GetTimezone(ctx, u.ID)
	if err != nil {
		return dao.User{}, err
	}
	return result, nil
}

func (r UserRepository) GetByEmail(ctx context.Context, email string) (dao.User, error) {
	u, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	result.Timezone, err = r.GetTimezone(ctx, u.ID)
	if err != nil {
		return dao.User{}, err
	}
	return result, nil
}

func (r UserRepository) Create(ctx context.Context, user domain.User) (dao.User, error) {
	u, err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        string(user.ID),
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
	})
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	result.Timezone, err = r.GetTimezone(ctx, u.ID)
	if err != nil {
		return dao.User{}, err
	}
	return result, nil
}

func (r UserRepository) Update(ctx context.Context, user domain.User) (dao.User, error) {
	u, err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        string(user.ID),
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
	})
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	result.Timezone, err = r.GetTimezone(ctx, u.ID)
	if err != nil {
		return dao.User{}, err
	}
	return result, nil
}

func (r UserRepository) UpdateTimezone(ctx context.Context, id domain.UserID, timezone domain.UserTimezone) error {
	var stored string
	err := r.db.QueryRow(ctx,
		"UPDATE users SET timezone = $2, updated_at = now() WHERE id = $1 RETURNING timezone",
		string(id), timezone.String(),
	).Scan(&stored)
	return err
}
