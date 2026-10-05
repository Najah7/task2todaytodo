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

var _ usecase.AccessTokenRepository = AccessTokenRepository{}

type AccessTokenRepository struct {
	queries *sqlc.Queries
}

func NewAccessTokenRepository(db sqlc.DBTX) *AccessTokenRepository {
	queries := sqlc.New(db)
	return &AccessTokenRepository{
		queries: queries,
	}
}

func (r *AccessTokenRepository) WithTx(tx pgx.Tx) *AccessTokenRepository {
	return &AccessTokenRepository{
		queries: r.queries.WithTx(tx),
	}
}

func recordToAccessTokenDAO(t sqlc.AccessToken) dao.AccessToken {
	return dao.AccessToken{
		Token:     t.Token,
		UserID:    t.UserID,
		ExpiresAt: pgTimeUnix(t.ExpiresAt),
		RevokedAt: pgTimeUnix(t.RevokedAt),
		CreatedAt: pgTimeUnix(t.CreatedAt),
	}
}

func (r AccessTokenRepository) GetByToken(ctx context.Context, token string) (dao.AccessToken, error) {
	t, err := r.queries.GetAccessTokenByToken(ctx, token)
	if err != nil {
		return dao.AccessToken{}, err
	}
	return recordToAccessTokenDAO(t), nil
}

func (r AccessTokenRepository) Create(ctx context.Context, token domain.AccessToken) (dao.AccessToken, error) {
	expiresAt := pgtype.Timestamptz{}
	err := expiresAt.Scan(time.Unix(token.ExpiresAt, 0))
	if err != nil {
		return dao.AccessToken{}, err
	}

	t, err := r.queries.CreateAccessToken(ctx, sqlc.CreateAccessTokenParams{
		Token:     token.Token,
		UserID:    string(token.UserID),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return dao.AccessToken{}, err
	}
	return recordToAccessTokenDAO(t), nil
}

func (r AccessTokenRepository) Revoke(ctx context.Context, token string) error {
	revokedAt := pgtype.Timestamptz{}
	err := revokedAt.Scan(time.Now())
	if err != nil {
		return err
	}

	err = r.queries.RevokeAccessToken(ctx, sqlc.RevokeAccessTokenParams{
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
