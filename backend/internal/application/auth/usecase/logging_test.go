package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type capturedLog struct {
	level string
	msg   string
	args  []any
}

type captureLogger struct{ entries []capturedLog }

type sqlStateTestError struct{ state string }

func (e sqlStateTestError) Error() string    { return "sensitive database details" }
func (e sqlStateTestError) SQLState() string { return e.state }

func (l *captureLogger) Debug(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{level: "debug", msg: msg, args: args})
}
func (l *captureLogger) Info(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{level: "info", msg: msg, args: args})
}
func (l *captureLogger) Warn(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{level: "warn", msg: msg, args: args})
}
func (l *captureLogger) Error(_ context.Context, msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{level: "error", msg: msg, args: args})
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
	for _, entry := range l.entries {
		if entry.level == level {
			n++
		}
	}
	return n
}

func TestLoginUserUseCaseLogsSecurityEventsWithoutCredentials(t *testing.T) {
	user := existingUser(t)
	for _, tt := range []struct {
		name      string
		password  string
		wantLevel string
		wantMsg   string
	}{
		{name: "success", password: "Password1!", wantLevel: "info", wantMsg: "authentication succeeded"},
		{name: "rejected", password: "WrongPassword1!", wantLevel: "warn", wantMsg: "authentication rejected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := &captureLogger{}
			_, _ = NewLoginUserUseCase(&stubUserRepository{userByEmail: user}, logger).Execute(context.Background(), "user@example.com", tt.password)
			if logger.count(tt.wantLevel) != 1 {
				t.Fatalf("%s log count = %d, want 1", tt.wantLevel, logger.count(tt.wantLevel))
			}
			if !strings.Contains(logger.text(), tt.wantMsg) {
				t.Fatalf("logs %q do not contain %q", logger.text(), tt.wantMsg)
			}
			logs := logger.text()
			for _, secret := range []string{"user@example.com", "Password1!", "WrongPassword1!", user.Password.String()} {
				if strings.Contains(logs, secret) {
					t.Errorf("logs contain credential %q: %s", secret, logs)
				}
			}
		})
	}
}

func TestAuthUnexpectedFailureLogsSafeErrorFields(t *testing.T) {
	const privateError = "database password=secret connection refused"
	logger := &captureLogger{}
	_, err := NewLoginUserUseCase(&stubUserRepository{getByEmailErr: errors.New(privateError)}, logger).Execute(context.Background(), "person@example.com", "SecretPassword1!")
	if err == nil {
		t.Fatal("Execute() error = nil, want repository failure")
	}
	if logger.count("error") != 1 {
		t.Fatalf("error log count = %d, want 1", logger.count("error"))
	}
	logs := logger.text()
	if !strings.Contains(logs, "error_type") || !strings.Contains(logs, "operation") {
		t.Errorf("logs lack safe diagnostic fields: %s", logs)
	}
	for _, secret := range []string{privateError, "person@example.com", "SecretPassword1"} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain sensitive value %q: %s", secret, logs)
		}
	}
}

func TestAuthExpectedErrorsDoNotLogError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "missing user", err: sql.ErrNoRows},
		{name: "canceled", err: context.Canceled},
		{name: "deadline exceeded", err: context.DeadlineExceeded},
		{name: "unique conflict", err: sqlStateTestError{state: "23505"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &captureLogger{}
			ctx := context.Background()
			if errors.Is(tt.err, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if tt.name == "unique conflict" {
				repo := &stubUserRepository{createErr: tt.err}
				_, _ = NewSignUpUseCase(repo, logger).Execute(ctx, "new-user", "person@example.com", "SecretPassword1!")
			} else {
				_, _ = NewLoginUserUseCase(&stubUserRepository{getByEmailErr: tt.err}, logger).Execute(ctx, "person@example.com", "SecretPassword1!")
			}
			if got := logger.count("error"); got != 0 {
				t.Errorf("error log count = %d, want 0", got)
			}
			if tt.name == "deadline exceeded" && logger.count("warn") != 0 {
				t.Errorf("warn log count = %d, want 0 for infrastructure deadline", logger.count("warn"))
			}
		})
	}
}
