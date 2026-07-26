package repository

import (
	"context"
	"time"

	auth "github.com/Najah7/task2todaytodo/internal/auth/domain"
	usecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/repositories/sqlc"
	"github.com/Najah7/task2todaytodo/internal/shared"
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

func recordToAccessToken(t sqlc.AccessToken) (auth.AccessToken, error) {
	ID, err := shared.NewID(t.UserID)
	if err != nil {
		return auth.NewZeroAccessToken(), err
	}
	accessToken, err := auth.NewExistingAccessToken(
		t.Token,
		auth.UserID(ID),
		pgTimeUnix(t.ExpiresAt),
		pgTimeUnix(t.RevokedAt),
		pgTimeUnix(t.CreatedAt),
	)
	if err != nil {
		return auth.NewZeroAccessToken(), err
	}

	return accessToken, nil
}

func (r AccessTokenRepository) GetByToken(ctx context.Context, token string) (auth.AccessToken, error) {
	t, err := r.queries.GetAccessTokenByToken(ctx, token)
	if err != nil {
		return auth.NewZeroAccessToken(), err
	}

	return recordToAccessToken(t)

}

func (r AccessTokenRepository) Create(ctx context.Context, token auth.AccessToken) (auth.AccessToken, error) {
	expiresAt := pgtype.Timestamptz{}
	err := expiresAt.Scan(time.Unix(token.ExpiresAt, 0))
	if err != nil {
		return auth.NewZeroAccessToken(), err
	}

	t, err := r.queries.CreateAccessToken(ctx, sqlc.CreateAccessTokenParams{
		Token:     token.Token,
		UserID:    string(token.UserID),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return auth.NewZeroAccessToken(), err
	}

	return recordToAccessToken(t)
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
