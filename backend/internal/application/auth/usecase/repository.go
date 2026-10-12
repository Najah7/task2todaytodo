package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

type UserRepository interface {
	Get(ctx context.Context, id domain.UserID) (dao.User, error)
	GetByEmail(ctx context.Context, email string) (dao.User, error)
	UpdateName(ctx context.Context, id domain.UserID, name domain.UserName) (dao.User, error)
	UpdateEmail(ctx context.Context, id domain.UserID, email domain.Email) (dao.User, error)
	UpdateTimezone(ctx context.Context, id domain.UserID, timezone domain.UserTimezone) error
	UpdateSleepSettings(ctx context.Context, id domain.UserID, wakeUpTime, bedtime string) error
	UpdateMinimumGapMinutes(ctx context.Context, id domain.UserID, minutes int) error
	TimezoneExists(ctx context.Context, timezone string) (bool, error)
}

type TimezoneRepository interface {
	ListTimezones(context.Context) ([]string, error)
}

type PersonalAccessTokenRepository interface {
	GetByToken(ctx context.Context, token string) (dao.PersonalAccessToken, error)
	CreateForAuth(ctx context.Context, token domain.PersonalAccessToken, authVersion int64) (dao.PersonalAccessToken, error)
	Revoke(ctx context.Context, token string) error
}

type EmailAuthRepository interface {
	GetUserByEmailForAuth(context.Context, string) (dao.User, error)
	GetUserByIDForAuth(context.Context, string) (dao.User, error)
	CreateUnverifiedUser(context.Context, string, string, string) (dao.User, error)
	StoreAuthLinkToken(context.Context, []byte, string, string, time.Time) error
	GetAuthLinkTokenForUpdate(context.Context, []byte) (dao.AuthLinkToken, error)
	GetAuthLinkToken(context.Context, []byte) (dao.AuthLinkToken, error)
	ConsumeAuthLinkToken(context.Context, []byte) (bool, error)
	MarkUserEmailVerified(context.Context, string) error
	UpdateUserPasswordForReset(context.Context, string, string) error
	RevokeAllUserPersonalAccessTokens(context.Context, string) error
	RevokeUserAuthLinkTokens(context.Context, string) error
	RevokeActiveUserAuthLinkTokensForPurpose(context.Context, string, string) error
	LockEmailDeliveryAttempts(context.Context, []byte) error
	DeleteExpiredEmailDeliveryAttempts(context.Context, []byte, time.Time) error
	CountEmailDeliveryAttemptsSince(context.Context, []byte, time.Time) (int64, error)
	CreateEmailDeliveryAttempt(context.Context, []byte, time.Time) error
}
