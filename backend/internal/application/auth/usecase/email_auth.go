package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

const (
	EmailVerificationPurpose  = "email_verification"
	PasswordResetPurpose      = "password_reset"
	emailVerificationLifetime = time.Hour
	passwordResetLifetime     = 30 * time.Minute
	emailAttemptCooldown      = time.Minute
	emailAttemptWindow        = time.Hour
	maxEmailAttemptsPerWindow = 5
)

var (
	ErrInvalidAuthLink = errors.New("authentication link is invalid or expired")
	ErrInvalidEmail    = errors.New("email address is invalid")
)

type AuthLinkSender interface {
	SendAuthLink(ctx context.Context, to, purpose, frontendBaseURL, rawToken string, lifetime time.Duration) error
}

type authEmailDependencies struct {
	uow      UOW
	sender   AuthLinkSender
	frontend string
	now      func() time.Time
	logger   telemetry.Logger
}

func newAuthEmailDependencies(uow UOW, sender AuthLinkSender, frontendURL string, logger telemetry.Logger) authEmailDependencies {
	return authEmailDependencies{uow: uow, sender: sender, frontend: frontendURL, now: time.Now, logger: telemetry.OrNop(logger)}
}

type authEmailRequest struct {
	email          domain.Email
	purpose        string
	createUserID   string
	passwordHash   string
	createIfAbsent bool
}

// requestAuthEmailLink persists per-address throttling and a new single-use
// link in one transaction. Sending starts only after that transaction commits.
func requestAuthEmailLink(ctx context.Context, deps authEmailDependencies, request authEmailRequest) {
	now := deps.now().UTC()
	addressHash := sha256.Sum256([]byte(strings.ToLower(request.email.String())))
	rawToken := ""
	sendTo := ""
	lifetime := emailVerificationLifetime
	if request.purpose == PasswordResetPurpose {
		lifetime = passwordResetLifetime
	}
	err := deps.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		repo := repositories.EmailAuth()
		if err := repo.LockEmailDeliveryAttempts(ctx, addressHash[:]); err != nil {
			return err
		}
		if err := repo.DeleteExpiredEmailDeliveryAttempts(ctx, addressHash[:], now.Add(-emailAttemptWindow)); err != nil {
			return err
		}
		attempts, err := repo.CountEmailDeliveryAttemptsSince(ctx, addressHash[:], now.Add(-emailAttemptWindow))
		if err != nil {
			return err
		}
		recent, err := repo.CountEmailDeliveryAttemptsSince(ctx, addressHash[:], now.Add(-emailAttemptCooldown))
		if err != nil {
			return err
		}
		if attempts >= maxEmailAttemptsPerWindow || recent > 0 {
			return nil
		}
		if err := repo.CreateEmailDeliveryAttempt(ctx, addressHash[:], now); err != nil {
			return err
		}

		user, err := repo.GetUserByEmailForAuth(ctx, request.email.String())
		if err != nil && !telemetry.IsNotFound(err) {
			return err
		}
		if telemetry.IsNotFound(err) {
			if !request.createIfAbsent {
				return nil
			}
			user, err = repo.CreateUnverifiedUser(ctx, request.createUserID, request.email.String(), request.passwordHash)
			if err != nil {
				return err
			}
		}
		if request.purpose == EmailVerificationPurpose && user.EmailVerifiedAt != 0 {
			return nil
		}
		if err := repo.RevokeActiveUserAuthLinkTokensForPurpose(ctx, user.ID, request.purpose); err != nil {
			return err
		}
		rawTokenBytes := make([]byte, 32)
		if _, err := rand.Read(rawTokenBytes); err != nil {
			return err
		}
		rawToken = base64.RawURLEncoding.EncodeToString(rawTokenBytes)
		tokenHash := sha256.Sum256(rawTokenBytes)
		if err := repo.StoreAuthLinkToken(ctx, tokenHash[:], user.ID, request.purpose, now.Add(lifetime)); err != nil {
			rawToken = ""
			return err
		}
		sendTo = user.Email
		return nil
	})
	if err != nil {
		logUnexpectedFailure(deps.logger, ctx, "auth_email.prepare", err)
		return
	}
	if rawToken == "" || sendTo == "" || deps.sender == nil {
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := deps.sender.SendAuthLink(sendCtx, sendTo, request.purpose, deps.frontend, rawToken, lifetime); err != nil {
		deps.logger.Warn(ctx, "authentication email delivery failed", "operation", "auth_email.send", "reason", "provider_error")
	}
	rawToken = ""
}

func parseAuthLinkToken(raw string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return nil, ErrInvalidAuthLink
	}
	hash := sha256.Sum256(decoded)
	return hash[:], nil
}

func validAuthLink(token dao.AuthLinkToken, purpose string, now time.Time) bool {
	return token.Purpose == purpose && token.ExpiresAt.After(now) && token.UsedAt.IsZero() && token.RevokedAt.IsZero()
}
