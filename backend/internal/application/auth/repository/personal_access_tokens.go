package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.PersonalAccessTokenRepository = PersonalAccessTokenRepository{}

type PersonalAccessTokenRepository struct {
	queries *sqlc.Queries
}

func NewPersonalAccessTokenRepository(db sqlc.DBTX) *PersonalAccessTokenRepository {
	queries := sqlc.New(db)
	return &PersonalAccessTokenRepository{
		queries: queries,
	}
}

func (r *PersonalAccessTokenRepository) WithTx(tx pgx.Tx) *PersonalAccessTokenRepository {
	return &PersonalAccessTokenRepository{
		queries: r.queries.WithTx(tx),
	}
}

func recordToPersonalAccessTokenDAO(t sqlc.PersonalAccessToken) dao.PersonalAccessToken {
	return dao.PersonalAccessToken{
		Token:     t.Token,
		UserID:    t.UserID,
		ExpiresAt: pgTimeUnix(t.ExpiresAt),
		RevokedAt: pgTimeUnix(t.RevokedAt),
		CreatedAt: pgTimeUnix(t.CreatedAt),
	}
}

func (r PersonalAccessTokenRepository) GetByToken(ctx context.Context, token string) (dao.PersonalAccessToken, error) {
	t, err := r.queries.GetPersonalAccessTokenByToken(ctx, token)
	if err != nil {
		return dao.PersonalAccessToken{}, err
	}
	return recordToPersonalAccessTokenDAO(t), nil
}

func (r PersonalAccessTokenRepository) Create(ctx context.Context, token domain.PersonalAccessToken) (dao.PersonalAccessToken, error) {
	expiresAt := pgtype.Timestamptz{}
	err := expiresAt.Scan(time.Unix(token.ExpiresAt, 0))
	if err != nil {
		return dao.PersonalAccessToken{}, err
	}

	t, err := r.queries.CreatePersonalAccessToken(ctx, sqlc.CreatePersonalAccessTokenParams{
		Token:     token.Token,
		UserID:    string(token.UserID),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return dao.PersonalAccessToken{}, err
	}
	return recordToPersonalAccessTokenDAO(t), nil
}

func (r PersonalAccessTokenRepository) Revoke(ctx context.Context, token string) error {
	revokedAt := pgtype.Timestamptz{}
	err := revokedAt.Scan(time.Now())
	if err != nil {
		return err
	}

	err = r.queries.RevokePersonalAccessToken(ctx, sqlc.RevokePersonalAccessTokenParams{
		Token:     token,
		RevokedAt: revokedAt,
	})
	if err != nil {
		return err
	}

	return nil
}

func pgTimeUnix(t pgtype.Timestamptz) int64 {
	if !t.Valid {
		return 0
	}

	return t.Time.Unix()
}
