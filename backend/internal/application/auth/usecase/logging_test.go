package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/jackc/pgx/v5"
)

type capturedLog struct {
	level, msg string
	args       []any
}
type captureLogger struct{ entries []capturedLog }

func (l *captureLogger) Debug(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{"debug", msg, args})
}
func (l *captureLogger) Info(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{"info", msg, args})
}
func (l *captureLogger) Warn(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{"warn", msg, args})
}
func (l *captureLogger) Error(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{"error", msg, args})
}
func (l *captureLogger) text() string {
	var b strings.Builder
	for _, entry := range l.entries {
		fmt.Fprintf(&b, "%s %s %v ", entry.level, entry.msg, entry.args)
	}
	return b.String()
}
func (l *captureLogger) count(level string) int {
	n := 0
	for _, e := range l.entries {
		if e.level == level {
			n++
		}
	}
	return n
}

type loginTestUOW struct {
	user                 dao.User
	lookupErr, createErr error
}

func (u loginTestUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, loginTestRepositories{uow: u})
}

type loginTestRepositories struct{ uow loginTestUOW }

func (loginTestRepositories) Users() UserRepository { return nil }
func (r loginTestRepositories) EmailAuth() EmailAuthRepository {
	return loginTestEmailAuth{user: r.uow.user, err: r.uow.lookupErr}
}
func (loginTestRepositories) Availability() UserAvailabilityValidator { return nil }
func (r loginTestRepositories) PersonalAccessTokens() PersonalAccessTokenRepository {
	return loginTestPAT{err: r.uow.createErr}
}
func (loginTestRepositories) Roles() RoleCatalogRepository { return nil }

type loginTestEmailAuth struct {
	EmailAuthRepository
	user dao.User
	err  error
}

func (r loginTestEmailAuth) GetUserByEmailForAuth(context.Context, string) (dao.User, error) {
	return r.user, r.err
}

type loginTestPAT struct {
	PersonalAccessTokenRepository
	err error
}

func (r loginTestPAT) CreateForAuth(_ context.Context, token domain.PersonalAccessToken, _ int64) (dao.PersonalAccessToken, error) {
	if r.err != nil {
		return dao.PersonalAccessToken{}, r.err
	}
	return personalAccessTokenDAOFromDomain(token), nil
}

func TestLoginAndIssueLogsSecurityEventsWithoutCredentials(t *testing.T) {
	user := existingUser(t)
	userRecord := userDAOFromDomain(user)
	for _, tt := range []struct{ name, password, level, msg string }{
		{"success", "Password1!", "info", "authentication succeeded"},
		{"rejected", "WrongPassword1!", "warn", "authentication rejected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := &captureLogger{}
			_, _ = NewLoginAndIssueTokenUseCase(loginTestUOW{user: userRecord}, logger).Execute(context.Background(), "USER@example.com", tt.password)
			if logger.count(tt.level) != 1 || !strings.Contains(logger.text(), tt.msg) {
				t.Fatalf("logs = %q", logger.text())
			}
			for _, secret := range []string{"USER@example.com", "Password1!", "WrongPassword1!", user.Password.String()} {
				if strings.Contains(logger.text(), secret) {
					t.Errorf("logs contain credential %q", secret)
				}
			}
		})
	}
}

func TestLoginAndIssueMissingEmailIsSafeRejection(t *testing.T) {
	logger := &captureLogger{}
	const email, password = "unknown@example.com", "SecretPassword1!"
	driverErr := fmt.Errorf("lookup failed: %w", pgx.ErrNoRows)
	_, err := NewLoginAndIssueTokenUseCase(loginTestUOW{lookupErr: driverErr}, logger).Execute(context.Background(), email, password)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Execute() error = %v, want invalid credentials", err)
	}
	if logger.count("warn") != 1 || logger.count("error") != 0 {
		t.Fatalf("warn=%d error=%d", logger.count("warn"), logger.count("error"))
	}
	for _, secret := range []string{email, password} {
		if strings.Contains(logger.text(), secret) {
			t.Errorf("logs contain %q", secret)
		}
	}
}

func TestLoginAndIssueMalformedEmailIsSafeRejectionWithoutLookup(t *testing.T) {
	logger := &captureLogger{}
	_, err := NewLoginAndIssueTokenUseCase(loginTestUOW{}, logger).Execute(context.Background(), "not-an-email", "SecretPassword1!")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Execute() error = %v, want invalid credentials", err)
	}
	if logger.count("warn") != 1 || logger.count("error") != 0 {
		t.Fatalf("warn=%d error=%d, want a safe rejection without lookup", logger.count("warn"), logger.count("error"))
	}
	if strings.Contains(logger.text(), "not-an-email") || strings.Contains(logger.text(), "SecretPassword1!") {
		t.Errorf("logs contain login input: %s", logger.text())
	}
}

func TestLoginAndIssueInfrastructureErrorIsSanitized(t *testing.T) {
	const privateError = "database password=secret connection refused"
	logger := &captureLogger{}
	_, err := NewLoginAndIssueTokenUseCase(loginTestUOW{lookupErr: errors.New(privateError)}, logger).Execute(context.Background(), "person@example.com", "SecretPassword1!")
	if err == nil || logger.count("error") != 1 {
		t.Fatalf("err=%v errorLogs=%d", err, logger.count("error"))
	}
	for _, secret := range []string{privateError, "person@example.com", "SecretPassword1!"} {
		if strings.Contains(logger.text(), secret) {
			t.Errorf("logs contain sensitive value %q", secret)
		}
	}
}

func TestLoginAndIssueDoesNotTreatPATWriteFailureAsSuccess(t *testing.T) {
	user := userDAOFromDomain(existingUser(t))
	logger := &captureLogger{}
	_, err := NewLoginAndIssueTokenUseCase(loginTestUOW{user: user, createErr: errors.New("insert failed")}, logger).Execute(context.Background(), user.Email, "Password1!")
	if err == nil {
		t.Fatal("Execute() returned nil error after PAT insert failure")
	}
	if logger.count("error") != 1 {
		t.Fatalf("error log count = %d", logger.count("error"))
	}
}
