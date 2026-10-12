//go:build integration

package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	"github.com/Najah7/task2todaytodo/internal/port/adapter/email"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	oldPassword = "InitialPassword1!"
	newPassword = "NewPassword1!"
)

var tokenPattern = regexp.MustCompile(`#token=([A-Za-z0-9_-]+)`)

type testMailer struct {
	mu       sync.Mutex
	messages []email.Message
	fail     bool
}

func (m *testMailer) Send(_ context.Context, message email.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
	if m.fail {
		return errors.New("test provider failure")
	}
	return nil
}

func (m *testMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *testMailer) tokenAt(t *testing.T, index int) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.messages) {
		t.Fatalf("mail message index %d is unavailable", index)
	}
	matches := tokenPattern.FindStringSubmatch(m.messages[index].TextBody)
	if len(matches) != 2 {
		t.Fatal("captured authentication email did not contain a link token")
	}
	return matches[1]
}

type fixture struct {
	t      *testing.T
	pool   *pgxpool.Pool
	uc     application.UseCase
	mailer *testMailer
	email  string
	userID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testdb.Open(t)
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	mailer := &testMailer{}
	id := adapter.NewIDIssuer()
	uc, stop := application.NewUseCase(store, uow, id, telemetry.Nop(), application.AuthEmailSettings{
		Sender: email.AuthLinkSender{Provider: mailer}, FrontendURL: "https://app.example.test",
	})
	testdb.RegisterShutdown(t, stop)
	userID := id.NewULID()
	return &fixture{
		t: t, pool: pool, uc: uc, mailer: mailer,
		email: "auth-" + strings.ToLower(userID) + "@example.test", userID: userID,
	}
}

func (f *fixture) cleanup() {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := f.pool.Exec(ctx, `DELETE FROM personal_access_tokens WHERE user_id = $1`, f.userID); err != nil {
		f.t.Errorf("clean up fixture access tokens: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, f.userID); err != nil {
		f.t.Errorf("clean up fixture user: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM email_delivery_attempts WHERE email_hash = $1`, emailHash(f.email)); err != nil {
		f.t.Errorf("clean up fixture email attempts: %v", err)
	}
}

func (f *fixture) register(password string) {
	f.t.Helper()
	f.t.Cleanup(f.cleanup)
	if err := f.uc.EmailAuth.RegisterUser.Execute(f.t.Context(), f.userID, f.email, password); err != nil {
		f.t.Fatalf("register user: %v", err)
	}
}

func emailHash(address string) []byte {
	sum := sha256.Sum256([]byte(strings.ToLower(address)))
	return sum[:]
}

func ageFixtureEmailAttempts(t *testing.T, f *fixture) {
	t.Helper()
	if _, err := f.pool.Exec(t.Context(), `UPDATE email_delivery_attempts SET attempted_at = attempted_at - interval '61 seconds' WHERE email_hash = $1`, emailHash(f.email)); err != nil {
		t.Fatalf("age fixture cooldown row: %v", err)
	}
}

func TestSignupVerificationRequiresOriginalPasswordAndConsumesLinkOnce(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	if got := f.mailer.count(); got != 1 {
		t.Fatalf("signup sent %d messages, want one", got)
	}
	var verifiedAt *time.Time
	var storedHash string
	if err := f.pool.QueryRow(t.Context(), `SELECT email_verified_at, password_hash FROM users WHERE id = $1`, f.userID).Scan(&verifiedAt, &storedHash); err != nil {
		t.Fatalf("read new account: %v", err)
	}
	if verifiedAt != nil {
		t.Fatal("new account was verified before link confirmation")
	}
	if !strings.HasPrefix(storedHash, "$argon2id$") {
		t.Fatal("signup did not persist an Argon2id password hash")
	}
	if _, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword); !errors.Is(err, usecase.ErrEmailUnverified) {
		t.Fatalf("login before verification returned unexpected error: %v", err)
	}
	link := f.mailer.tokenAt(t, 0)
	assertLinkLifetime(t, f, link, time.Hour)
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), link, "WrongPassword1!"); !errors.Is(err, usecase.ErrInvalidAuthLink) {
		t.Fatalf("wrong-password verification returned unexpected error: %v", err)
	}
	var usedAt *time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT used_at FROM auth_link_tokens WHERE token_hash = $1`, tokenHash(link)).Scan(&usedAt); err != nil {
		t.Fatalf("read verification token state: %v", err)
	}
	if usedAt != nil {
		t.Fatal("wrong password consumed the verification link")
	}
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), link, oldPassword); err != nil {
		t.Fatalf("confirm verification: %v", err)
	}
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), link, oldPassword); !errors.Is(err, usecase.ErrInvalidAuthLink) {
		t.Fatalf("reused verification link returned unexpected error: %v", err)
	}
	pat, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword)
	if err != nil {
		t.Fatalf("login after verification: %v", err)
	}
	if got, err := f.uc.PersonalAccessToken.Authenticate.Execute(t.Context(), pat.Token); err != nil || string(got) != f.userID {
		t.Fatalf("issued access token was not accepted for its account (error=%v)", err)
	}
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), "malformed", oldPassword); !errors.Is(err, usecase.ErrInvalidAuthLink) {
		t.Fatalf("malformed verification token returned unexpected error: %v", err)
	}
}

func TestPasswordResetVerifiesPendingAccountAndInvalidatesOldCredentials(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	verifyToken := f.mailer.tokenAt(t, 0)
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if f.mailer.count() != 1 {
		t.Fatal("combined email cooldown did not suppress immediate reset delivery")
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE email_delivery_attempts SET attempted_at = attempted_at - interval '61 seconds' WHERE email_hash = $1`, emailHash(f.email)); err != nil {
		t.Fatalf("age fixture cooldown row: %v", err)
	}
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
		t.Fatalf("request reset after cooldown: %v", err)
	}
	resetToken := f.mailer.tokenAt(t, 1)
	assertLinkLifetime(t, f, resetToken, 30*time.Minute)
	if err := f.uc.EmailAuth.ConfirmPasswordReset.Execute(t.Context(), resetToken, newPassword); err != nil {
		t.Fatalf("confirm reset for pending account: %v", err)
	}
	var verifiedAt *time.Time
	var hash string
	if err := f.pool.QueryRow(t.Context(), `SELECT email_verified_at, password_hash FROM users WHERE id = $1`, f.userID).Scan(&verifiedAt, &hash); err != nil {
		t.Fatalf("read reset account: %v", err)
	}
	if verifiedAt == nil || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("password reset did not verify the pending account and retain an Argon2id hash")
	}
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), verifyToken, oldPassword); !errors.Is(err, usecase.ErrInvalidAuthLink) {
		t.Fatalf("pre-reset verification link remained usable: %v", err)
	}
	if _, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword); !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("old password remained valid after reset: %v", err)
	}
	if _, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, newPassword); err != nil {
		t.Fatalf("new password was rejected after reset: %v", err)
	}
}

func TestPasswordResetRevokesExistingPATAndAllActiveAuthLinks(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), f.mailer.tokenAt(t, 0), oldPassword); err != nil {
		t.Fatalf("confirm verification: %v", err)
	}
	pat, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword)
	if err != nil {
		t.Fatalf("issue PAT before reset: %v", err)
	}
	ageFixtureEmailAttempts(t, f)
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	resetToken := f.mailer.tokenAt(t, 1)
	if err := f.uc.EmailAuth.ConfirmPasswordReset.Execute(t.Context(), resetToken, newPassword); err != nil {
		t.Fatalf("confirm reset: %v", err)
	}
	if _, err := f.uc.PersonalAccessToken.Authenticate.Execute(t.Context(), pat.Token); err == nil {
		t.Fatal("pre-reset personal access token remained usable")
	}
	var activeLinks int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM auth_link_tokens WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, f.userID).Scan(&activeLinks); err != nil {
		t.Fatalf("count active auth links after reset: %v", err)
	}
	if activeLinks != 0 {
		t.Fatalf("password reset left %d active authentication links", activeLinks)
	}
}

func TestPasswordResetDatabaseFailureRollsBackConsumptionAndCredentialChanges(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), f.mailer.tokenAt(t, 0), oldPassword); err != nil {
		t.Fatalf("confirm verification: %v", err)
	}
	pat, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword)
	if err != nil {
		t.Fatalf("issue PAT before reset: %v", err)
	}
	var verifiedBefore *time.Time
	var hashBefore string
	if err := f.pool.QueryRow(t.Context(), `SELECT email_verified_at, password_hash FROM users WHERE id = $1`, f.userID).Scan(&verifiedBefore, &hashBefore); err != nil {
		t.Fatalf("read account before reset: %v", err)
	}
	ageFixtureEmailAttempts(t, f)
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	resetToken := f.mailer.tokenAt(t, 1)
	removeTrigger := installResetFailureTrigger(t, f)
	if err := f.uc.EmailAuth.ConfirmPasswordReset.Execute(t.Context(), resetToken, newPassword); err == nil {
		t.Fatal("reset unexpectedly succeeded while the fixture trigger rejected password updates")
	}
	var verifiedAt *time.Time
	var storedHash string
	if err := f.pool.QueryRow(t.Context(), `SELECT email_verified_at, password_hash FROM users WHERE id = $1`, f.userID).Scan(&verifiedAt, &storedHash); err != nil {
		t.Fatalf("read account after failed reset: %v", err)
	}
	if verifiedAt == nil || verifiedBefore == nil || !verifiedAt.Equal(*verifiedBefore) || storedHash != hashBefore {
		t.Fatal("failed reset changed verified-account state or removed its password hash")
	}
	var usedAt *time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT used_at FROM auth_link_tokens WHERE token_hash = $1`, tokenHash(resetToken)).Scan(&usedAt); err != nil {
		t.Fatalf("read reset token after failed transaction: %v", err)
	}
	if usedAt != nil {
		t.Fatal("failed reset consumed its link despite rolling back")
	}
	if _, err := f.uc.PersonalAccessToken.Authenticate.Execute(t.Context(), pat.Token); err != nil {
		t.Fatalf("failed reset revoked the existing PAT: %v", err)
	}
	if _, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword); err != nil {
		t.Fatalf("failed reset changed the existing password: %v", err)
	}
	removeTrigger()
	if err := f.uc.EmailAuth.ConfirmPasswordReset.Execute(t.Context(), resetToken, newPassword); err != nil {
		t.Fatalf("retry reset after removing injected database failure: %v", err)
	}
	if _, err := f.uc.PersonalAccessToken.Authenticate.Execute(t.Context(), pat.Token); err == nil {
		t.Fatal("successful reset retry left the pre-reset PAT active")
	}
}

func TestLoginReturnsPATInsertFailureWithoutEmptySuccessOrPersistedToken(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), f.mailer.tokenAt(t, 0), oldPassword); err != nil {
		t.Fatalf("confirm verification: %v", err)
	}
	removeTrigger := installPATInsertFailureTrigger(t, f)
	pat, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword)
	if err == nil {
		t.Fatal("login reported success while the fixture trigger rejected PAT insertion")
	}
	if pat.Token != "" || pat.UserID != "" {
		t.Fatal("failed PAT insertion returned a partially populated token")
	}
	var count int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM personal_access_tokens WHERE user_id = $1`, f.userID).Scan(&count); err != nil {
		t.Fatalf("count access tokens after failed login: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed login persisted %d access tokens", count)
	}
	removeTrigger()
	pat, err = f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), f.email, oldPassword)
	if err != nil {
		t.Fatalf("login after removing injected failure: %v", err)
	}
	if pat.Token == "" || pat.UserID != f.userID {
		t.Fatal("successful login did not return a complete access token")
	}
}

func installResetFailureTrigger(t *testing.T, f *fixture) func() {
	t.Helper()
	// User IDs are ULIDs generated locally and contain only ASCII alphanumerics.
	functionName := "auth_test_reset_fail_" + f.userID
	triggerName := "auth_test_reset_fail_" + f.userID
	functionSQL := fmt.Sprintf(`CREATE FUNCTION public.%s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.id = '%s' AND NEW.password_hash IS DISTINCT FROM OLD.password_hash THEN
		RAISE EXCEPTION 'injected integration-test password update failure';
	END IF;
	RETURN NEW;
END;
$$`, functionName, f.userID)
	if _, err := f.pool.Exec(t.Context(), functionSQL); err != nil {
		t.Fatalf("install scoped reset failure function: %v", err)
	}
	triggerSQL := fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION public.%s()`, triggerName, functionName)
	if _, err := f.pool.Exec(t.Context(), triggerSQL); err != nil {
		_, _ = f.pool.Exec(t.Context(), fmt.Sprintf(`DROP FUNCTION IF EXISTS public.%s()`, functionName))
		t.Fatalf("install scoped reset failure trigger: %v", err)
	}
	remove := func() {
		if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON users`, triggerName)); err != nil {
			t.Errorf("remove scoped reset failure trigger: %v", err)
		}
		if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS public.%s()`, functionName)); err != nil {
			t.Errorf("remove scoped reset failure function: %v", err)
		}
	}
	t.Cleanup(remove)
	return remove
}

func installPATInsertFailureTrigger(t *testing.T, f *fixture) func() {
	t.Helper()
	functionName := "auth_test_pat_fail_" + f.userID
	triggerName := "auth_test_pat_fail_" + f.userID
	functionSQL := fmt.Sprintf(`CREATE FUNCTION public.%s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.user_id = '%s' THEN
		RAISE EXCEPTION 'injected integration-test PAT insert failure';
	END IF;
	RETURN NEW;
END;
$$`, functionName, f.userID)
	if _, err := f.pool.Exec(t.Context(), functionSQL); err != nil {
		t.Fatalf("install scoped PAT failure function: %v", err)
	}
	triggerSQL := fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON personal_access_tokens FOR EACH ROW EXECUTE FUNCTION public.%s()`, triggerName, functionName)
	if _, err := f.pool.Exec(t.Context(), triggerSQL); err != nil {
		_, _ = f.pool.Exec(t.Context(), fmt.Sprintf(`DROP FUNCTION IF EXISTS public.%s()`, functionName))
		t.Fatalf("install scoped PAT failure trigger: %v", err)
	}
	remove := func() {
		if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON personal_access_tokens`, triggerName)); err != nil {
			t.Errorf("remove scoped PAT failure trigger: %v", err)
		}
		if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS public.%s()`, functionName)); err != nil {
			t.Errorf("remove scoped PAT failure function: %v", err)
		}
	}
	t.Cleanup(remove)
	return remove
}

func TestDuplicateSignupDoesNotOverwritePasswordAndRateLimitIsShared(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	originalHash := ""
	if err := f.pool.QueryRow(t.Context(), `SELECT password_hash FROM users WHERE id = $1`, f.userID).Scan(&originalHash); err != nil {
		t.Fatalf("read original password hash: %v", err)
	}
	if err := f.uc.EmailAuth.RegisterUser.Execute(t.Context(), adapter.NewIDIssuer().NewULID(), strings.ToUpper(f.email), "OtherPassword1!"); err != nil {
		t.Fatalf("duplicate signup: %v", err)
	}
	if err := f.uc.EmailAuth.ResendVerification.Execute(t.Context(), strings.ToUpper(f.email)); err != nil {
		t.Fatalf("resend during shared cooldown: %v", err)
	}
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), strings.ToUpper(f.email)); err != nil {
		t.Fatalf("reset request during shared cooldown: %v", err)
	}
	if got := f.mailer.count(); got != 1 {
		t.Fatalf("combined endpoints sent %d emails under one-address cooldown, want one", got)
	}
	var storedHash string
	if err := f.pool.QueryRow(t.Context(), `SELECT password_hash FROM users WHERE id = $1`, f.userID).Scan(&storedHash); err != nil {
		t.Fatalf("read password hash after duplicate signup: %v", err)
	}
	if storedHash != originalHash {
		t.Fatal("duplicate signup overwrote the existing password hash")
	}
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), f.mailer.tokenAt(t, 0), oldPassword); err != nil {
		t.Fatalf("case-variant email registration changed original credentials: %v", err)
	}
	if _, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(t.Context(), strings.ToUpper(f.email), oldPassword); err != nil {
		t.Fatalf("case-variant login failed for the existing account: %v", err)
	}
}

func TestAuthLinkExpiryAndWrongPurposeAreRejected(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	verify := f.mailer.tokenAt(t, 0)
	if err := f.uc.EmailAuth.ConfirmPasswordReset.Execute(t.Context(), verify, newPassword); !errors.Is(err, usecase.ErrInvalidAuthLink) {
		t.Fatalf("verification token accepted as reset token: %v", err)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE auth_link_tokens SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, tokenHash(verify)); err != nil {
		t.Fatalf("expire test-owned link: %v", err)
	}
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), verify, oldPassword); !errors.Is(err, usecase.ErrInvalidAuthLink) {
		t.Fatalf("expired verification link returned unexpected error: %v", err)
	}
}

func TestConcurrentVerificationConsumesLinkExactlyOnce(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	link := f.mailer.tokenAt(t, 0)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			results <- f.uc.EmailAuth.ConfirmVerification.Execute(ctx, link, oldPassword)
		}()
	}
	close(start)
	deadline := time.After(25 * time.Second)
	succeeded := 0
	var unexpected error
	for range 2 {
		select {
		case err := <-results:
			if err == nil {
				succeeded++
			} else if !errors.Is(err, usecase.ErrInvalidAuthLink) {
				unexpected = err
			}
		case <-deadline:
			t.Fatal("concurrent verification did not finish; possible lock-order deadlock")
		}
	}
	if unexpected != nil {
		t.Fatalf("concurrent verification returned unexpected error: %v", unexpected)
	}
	if succeeded != 1 {
		t.Fatalf("concurrent confirmation succeeded %d times, want exactly one", succeeded)
	}
}

func TestVerificationConfirmationRacesResendWithoutDeadlock(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	oldLink := f.mailer.tokenAt(t, 0)
	ageFixtureEmailAttempts(t, f)
	start := make(chan struct{})
	type result struct {
		operation string
		err       error
	}
	results := make(chan result, 2)
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		results <- result{operation: "confirm", err: f.uc.EmailAuth.ConfirmVerification.Execute(ctx, oldLink, oldPassword)}
	}()
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		results <- result{operation: "resend", err: f.uc.EmailAuth.ResendVerification.Execute(ctx, f.email)}
	}()
	close(start)
	deadline := time.After(25 * time.Second)
	var confirmErr, resendErr error
	for range 2 {
		select {
		case result := <-results:
			if result.operation == "confirm" {
				confirmErr = result.err
			} else {
				resendErr = result.err
			}
		case <-deadline:
			t.Fatal("confirmation/resend race did not finish; possible lock-order deadlock")
		}
	}
	if resendErr != nil {
		t.Fatalf("racing resend returned unexpected error: %v", resendErr)
	}
	if confirmErr != nil && !errors.Is(confirmErr, usecase.ErrInvalidAuthLink) {
		t.Fatalf("racing confirmation returned unexpected error: %v", confirmErr)
	}
	var verified bool
	var activeVerificationLinks int
	if err := f.pool.QueryRow(t.Context(), `SELECT email_verified_at IS NOT NULL FROM users WHERE id = $1`, f.userID).Scan(&verified); err != nil {
		t.Fatalf("read verification state after race: %v", err)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM auth_link_tokens WHERE user_id = $1 AND purpose = 'email_verification' AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now()`, f.userID).Scan(&activeVerificationLinks); err != nil {
		t.Fatalf("count active verification links after race: %v", err)
	}
	if verified && activeVerificationLinks != 0 {
		t.Fatal("resend left an active verification link after confirmation succeeded")
	}
	if !verified && activeVerificationLinks != 1 {
		t.Fatalf("confirmation failed but resend left %d active links, want one", activeVerificationLinks)
	}
}

func TestLoginAndResetRaceCannotLeaveOldPasswordPATUsable(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	if err := f.uc.EmailAuth.ConfirmVerification.Execute(t.Context(), f.mailer.tokenAt(t, 0), oldPassword); err != nil {
		t.Fatalf("confirm verification: %v", err)
	}
	ageFixtureEmailAttempts(t, f)
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	resetToken := f.mailer.tokenAt(t, 1)
	start := make(chan struct{})
	type result struct {
		operation string
		err       error
	}
	results := make(chan result, 2)
	issued := make(chan string, 1)
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		pat, err := f.uc.PersonalAccessToken.LoginAndIssue.Execute(ctx, f.email, oldPassword)
		if err == nil {
			issued <- pat.Token
		}
		results <- result{operation: "login", err: err}
	}()
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		results <- result{operation: "reset", err: f.uc.EmailAuth.ConfirmPasswordReset.Execute(ctx, resetToken, newPassword)}
	}()
	close(start)
	deadline := time.After(25 * time.Second)
	var loginErr, resetErr error
	var unexpected error
	for range 2 {
		select {
		case result := <-results:
			if result.operation == "login" {
				loginErr = result.err
				if loginErr != nil && !errors.Is(loginErr, usecase.ErrInvalidCredentials) {
					unexpected = loginErr
				}
			} else {
				resetErr = result.err
				if resetErr != nil {
					unexpected = resetErr
				}
			}
		case <-deadline:
			t.Fatal("login/reset race did not finish; possible lock-order deadlock")
		}
	}
	if unexpected != nil {
		t.Fatalf("login/reset race returned unexpected error: %v", unexpected)
	}
	// The reset completion can be identified in PostgreSQL by the user's auth
	// version; the pre-reset token, when one was issued, must already be unusable.
	var authVersion int64
	if err := f.pool.QueryRow(t.Context(), `SELECT auth_version FROM users WHERE id = $1`, f.userID).Scan(&authVersion); err != nil {
		t.Fatalf("read reset auth version: %v", err)
	}
	if authVersion != 1 || resetErr != nil {
		t.Fatal("password reset did not complete exactly once")
	}
	select {
	case token := <-issued:
		if _, err := f.uc.PersonalAccessToken.Authenticate.Execute(t.Context(), token); err == nil {
			t.Fatal("a PAT issued concurrently with reset remained usable afterward")
		}
	default:
	}
}

func TestEmailAttemptLimitIncludesFailedDeliveryAndAllAuthEndpoints(t *testing.T) {
	f := newFixture(t)
	f.mailer.fail = true
	f.register(oldPassword)
	for n := 1; n < 5; n++ {
		// Keep the operation independent of wall-clock sleeps while modifying only
		// this fixture's delivery-attempt records.
		if _, err := f.pool.Exec(t.Context(), `UPDATE email_delivery_attempts SET attempted_at = attempted_at - interval '61 seconds' WHERE email_hash = $1`, emailHash(f.email)); err != nil {
			t.Fatalf("age test-owned cooldown row: %v", err)
		}
		if n%2 == 0 {
			if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
				t.Fatalf("reset request %d: %v", n, err)
			}
		} else if err := f.uc.EmailAuth.ResendVerification.Execute(t.Context(), f.email); err != nil {
			t.Fatalf("verification resend %d: %v", n, err)
		}
	}
	if got := f.mailer.count(); got != 5 {
		t.Fatalf("provider saw %d attempts before hourly cap, want 5", got)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE email_delivery_attempts SET attempted_at = attempted_at - interval '61 seconds' WHERE email_hash = $1`, emailHash(f.email)); err != nil {
		t.Fatalf("age test-owned cooldown row: %v", err)
	}
	if err := f.uc.EmailAuth.RequestPasswordReset.Execute(t.Context(), f.email); err != nil {
		t.Fatalf("request after hourly cap: %v", err)
	}
	if got := f.mailer.count(); got != 5 {
		t.Fatalf("hourly cap allowed another provider attempt; count=%d", got)
	}
	var attempts int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_delivery_attempts WHERE email_hash = $1`, emailHash(f.email)).Scan(&attempts); err != nil {
		t.Fatalf("count fixture delivery attempts: %v", err)
	}
	if attempts != 5 {
		t.Fatalf("failed sends were not retained in combined rate-limit accounting: got %d attempts", attempts)
	}
}

func TestConcurrentCrossInstanceEmailRequestsShareCooldownWithoutExtendingIt(t *testing.T) {
	f := newFixture(t)
	f.register(oldPassword)
	ageFixtureEmailAttempts(t, f)
	var previousAttempt time.Time
	if err := f.pool.QueryRow(t.Context(), `SELECT max(attempted_at) FROM email_delivery_attempts WHERE email_hash = $1`, emailHash(f.email)).Scan(&previousAttempt); err != nil {
		t.Fatalf("read aged delivery attempt: %v", err)
	}

	secondMailer := &testMailer{}
	store := application.NewStore(f.pool)
	uow := application.NewUOW(f.pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	secondID := adapter.NewIDIssuer()
	secondUseCase, stop := application.NewUseCase(store, uow, secondID, telemetry.Nop(), application.AuthEmailSettings{
		Sender: email.AuthLinkSender{Provider: secondMailer}, FrontendURL: "https://app.example.test",
	})
	testdb.RegisterShutdown(t, stop)
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		results <- f.uc.EmailAuth.ResendVerification.Execute(ctx, f.email)
	}()
	go func() {
		<-start
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		results <- secondUseCase.EmailAuth.RequestPasswordReset.Execute(ctx, strings.ToUpper(f.email))
	}()
	close(start)
	deadline := time.After(25 * time.Second)
	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("concurrent generic email request returned an error: %v", err)
			}
		case <-deadline:
			t.Fatal("cross-instance email request race did not finish")
		}
	}
	if total := f.mailer.count() + secondMailer.count(); total != 2 {
		t.Fatalf("signup plus concurrent requests sent %d emails, want exactly two", total)
	}
	var attempts int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_delivery_attempts WHERE email_hash = $1`, emailHash(f.email)).Scan(&attempts); err != nil {
		t.Fatalf("count cross-instance email attempts: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("concurrent requests recorded %d attempts, want the original plus one admitted request", attempts)
	}
	var preserved int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM email_delivery_attempts WHERE email_hash = $1 AND attempted_at = $2`, emailHash(f.email), previousAttempt).Scan(&preserved); err != nil {
		t.Fatalf("check rejected request did not move prior attempt: %v", err)
	}
	if preserved != 1 {
		t.Fatal("rejected concurrent request extended or replaced the prior cooldown timestamp")
	}
}

func tokenHash(raw string) []byte {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(decoded)
	return sum[:]
}

func assertLinkLifetime(t *testing.T, f *fixture, raw string, want time.Duration) {
	t.Helper()
	var seconds int64
	if err := f.pool.QueryRow(t.Context(), `SELECT extract(epoch FROM expires_at - created_at)::bigint FROM auth_link_tokens WHERE token_hash = $1`, tokenHash(raw)).Scan(&seconds); err != nil {
		t.Fatalf("read authentication link lifetime: %v", err)
	}
	if got := time.Duration(seconds) * time.Second; got != want {
		t.Fatalf("authentication link lifetime is %s, want %s", got, want)
	}
}
