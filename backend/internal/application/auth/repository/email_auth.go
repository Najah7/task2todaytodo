package repository

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.EmailAuthRepository = EmailAuthRepository{}

type EmailAuthRepository struct{ queries *sqlc.Queries }

func NewEmailAuthRepository(db sqlc.DBTX) *EmailAuthRepository {
	return &EmailAuthRepository{queries: sqlc.New(db)}
}

func (r *EmailAuthRepository) WithTx(tx pgx.Tx) *EmailAuthRepository {
	return &EmailAuthRepository{queries: r.queries.WithTx(tx)}
}

func (r EmailAuthRepository) GetUserByEmailForAuth(ctx context.Context, email string) (dao.User, error) {
	u, err := r.queries.GetUserByEmailForAuth(ctx, email)
	if err != nil {
		return dao.User{}, err
	}
	return recordToUserDAO(u), nil
}

func (r EmailAuthRepository) GetUserByIDForAuth(ctx context.Context, userID string) (dao.User, error) {
	u, err := r.queries.GetUserByIDForAuth(ctx, userID)
	if err != nil {
		return dao.User{}, err
	}
	return recordToUserDAO(u), nil
}

func (r EmailAuthRepository) CreateUnverifiedUser(ctx context.Context, id, email, passwordHash string) (dao.User, error) {
	u, err := r.queries.CreateUnverifiedUser(ctx, sqlc.CreateUnverifiedUserParams{ID: id, Email: email, PasswordHash: passwordHash})
	if err != nil {
		return dao.User{}, err
	}
	return recordToUserDAO(u), nil
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	var result pgtype.Timestamptz
	_ = result.Scan(value)
	return result
}

func (r EmailAuthRepository) StoreAuthLinkToken(ctx context.Context, hash []byte, userID, purpose string, expiresAt time.Time) error {
	return r.queries.CreateAuthLinkToken(ctx, sqlc.CreateAuthLinkTokenParams{TokenHash: hash, UserID: userID, Purpose: purpose, ExpiresAt: pgTimestamp(expiresAt)})
}

func (r EmailAuthRepository) GetAuthLinkTokenForUpdate(ctx context.Context, hash []byte) (dao.AuthLinkToken, error) {
	t, err := r.queries.GetAuthLinkTokenForUpdate(ctx, hash)
	if err != nil {
		return dao.AuthLinkToken{}, err
	}
	return authLinkDAO(t.TokenHash, t.UserID, t.Purpose, t.ExpiresAt, t.UsedAt, t.RevokedAt), nil
}

func (r EmailAuthRepository) GetAuthLinkToken(ctx context.Context, hash []byte) (dao.AuthLinkToken, error) {
	t, err := r.queries.GetAuthLinkToken(ctx, hash)
	if err != nil {
		return dao.AuthLinkToken{}, err
	}
	return authLinkDAO(t.TokenHash, t.UserID, t.Purpose, t.ExpiresAt, t.UsedAt, t.RevokedAt), nil
}

func authLinkDAO(hash []byte, userID, purpose string, expiresAt, usedAt, revokedAt pgtype.Timestamptz) dao.AuthLinkToken {
	return dao.AuthLinkToken{TokenHash: hash, UserID: userID, Purpose: purpose, ExpiresAt: expiresAt.Time, UsedAt: nullableTime(usedAt), RevokedAt: nullableTime(revokedAt)}
}

func nullableTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func (r EmailAuthRepository) ConsumeAuthLinkToken(ctx context.Context, hash []byte) (bool, error) {
	affected, err := r.queries.ConsumeAuthLinkToken(ctx, hash)
	return affected > 0, err
}

func (r EmailAuthRepository) MarkUserEmailVerified(ctx context.Context, userID string) error {
	_, err := r.queries.MarkUserEmailVerified(ctx, userID)
	return err
}

func (r EmailAuthRepository) UpdateUserPasswordForReset(ctx context.Context, userID, passwordHash string) error {
	_, err := r.queries.UpdateUserPasswordForReset(ctx, sqlc.UpdateUserPasswordForResetParams{ID: userID, PasswordHash: passwordHash})
	return err
}

func (r EmailAuthRepository) RevokeAllUserPersonalAccessTokens(ctx context.Context, userID string) error {
	return r.queries.RevokeAllUserPersonalAccessTokens(ctx, userID)
}

func (r EmailAuthRepository) RevokeUserAuthLinkTokens(ctx context.Context, userID string) error {
	return r.queries.RevokeUserAuthLinkTokens(ctx, userID)
}

func (r EmailAuthRepository) RevokeActiveUserAuthLinkTokensForPurpose(ctx context.Context, userID, purpose string) error {
	return r.queries.RevokeActiveUserAuthLinkTokensForPurpose(ctx, sqlc.RevokeActiveUserAuthLinkTokensForPurposeParams{UserID: userID, Purpose: purpose})
}

func (r EmailAuthRepository) LockEmailDeliveryAttempts(ctx context.Context, hash []byte) error {
	return r.queries.LockEmailDeliveryAttempts(ctx, hash)
}

func (r EmailAuthRepository) DeleteExpiredEmailDeliveryAttempts(ctx context.Context, hash []byte, cutoff time.Time) error {
	return r.queries.DeleteExpiredEmailDeliveryAttempts(ctx, sqlc.DeleteExpiredEmailDeliveryAttemptsParams{EmailHash: hash, AttemptedAt: pgTimestamp(cutoff)})
}

func (r EmailAuthRepository) CountEmailDeliveryAttemptsSince(ctx context.Context, hash []byte, since time.Time) (int64, error) {
	return r.queries.CountRecentEmailDeliveryAttempts(ctx, sqlc.CountRecentEmailDeliveryAttemptsParams{EmailHash: hash, AttemptedAt: pgTimestamp(since)})
}

func (r EmailAuthRepository) CreateEmailDeliveryAttempt(ctx context.Context, hash []byte, at time.Time) error {
	return r.queries.CreateEmailDeliveryAttempt(ctx, sqlc.CreateEmailDeliveryAttemptParams{EmailHash: hash, AttemptedAt: pgTimestamp(at)})
}
