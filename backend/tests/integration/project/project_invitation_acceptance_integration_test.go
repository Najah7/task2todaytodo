//go:build integration

package project_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func TestProjectInvitationAcceptanceCreatesVerifiedUserAndConsumesToken(t *testing.T) {
	fixture := newInvitationAcceptanceFixture(t)
	usecase := fixture.usecase()
	result, err := usecase.Execute(t.Context(), fixture.rawToken, "NewPassword1!", "", ulid.Make().String())
	if err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	if !result.NeedsAutoLogin || result.PersonalAccessToken == "" {
		t.Fatalf("acceptance result = %+v; want PAT for new account", result)
	}
	var verified bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT email_verified_at IS NOT NULL FROM users WHERE id=$1`, result.UserID).Scan(&verified); err != nil || !verified {
		t.Fatalf("new recipient verification state = %t, error %v", verified, err)
	}
	var memberRole string
	if err := fixture.pool.QueryRow(t.Context(), `SELECT role_id FROM project_members WHERE project_id=$1 AND user_id=$2`, fixture.projectID, result.UserID).Scan(&memberRole); err != nil || memberRole != "editor" {
		t.Fatalf("new recipient role = %q, error %v; want editor", memberRole, err)
	}
	var used bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT used_at IS NOT NULL FROM project_invitations WHERE id=$1`, fixture.invitationID).Scan(&used); err != nil || !used {
		t.Fatalf("invitation used = %t, error %v", used, err)
	}
	var storedHash []byte
	if err := fixture.pool.QueryRow(t.Context(), `SELECT token_hash FROM project_invitations WHERE id=$1`, fixture.invitationID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(fixture.tokenBytes)
	if string(storedHash) != string(wantHash[:]) {
		t.Fatal("stored invitation token is not SHA256 of raw token bytes")
	}
	if _, err := usecase.Execute(t.Context(), fixture.rawToken, "NewPassword1!", "", ulid.Make().String()); !errors.Is(err, projectusecase.ErrInvalidProjectInvitation) {
		t.Fatalf("token reuse error = %v; want invalid invitation", err)
	}
}

func TestProjectInvitationAcceptanceRequiresVerifiedAccountSessionMatch(t *testing.T) {
	fixture := newInvitationAcceptanceFixture(t)
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Invite','Recipient',$2,'unused',now(),'UTC')`, fixture.recipientID, fixture.email); err != nil {
		t.Fatal(err)
	}
	usecase := fixture.usecase()
	if _, err := usecase.Execute(t.Context(), fixture.rawToken, "", "", ulid.Make().String()); !errors.Is(err, projectusecase.ErrInvitationRecipientAuthenticationRequired) {
		t.Fatalf("unauthenticated verified recipient error = %v; want authentication required", err)
	}
	if _, err := usecase.Execute(t.Context(), fixture.rawToken, "", ulid.Make().String(), ulid.Make().String()); !errors.Is(err, projectusecase.ErrInvitationRecipientAuthenticationRequired) {
		t.Fatalf("mismatched account error = %v; want authentication required", err)
	}
	result, err := usecase.Execute(t.Context(), fixture.rawToken, "", fixture.recipientID, ulid.Make().String())
	if err != nil || result.NeedsAutoLogin || result.PersonalAccessToken != "" {
		t.Fatalf("verified account acceptance = %+v, error %v; want same session and no new PAT", result, err)
	}
}

func TestProjectInvitationAcceptanceUpgradesUnverifiedAccountAndRevokesOldCredentials(t *testing.T) {
	fixture := newInvitationAcceptanceFixture(t)
	ctx := t.Context()
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password_hash,timezone) VALUES($1,'Invite','Recipient',$2,'unused','UTC')`, fixture.recipientID, fixture.email); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO personal_access_tokens(token,user_id,auth_version,expires_at) VALUES('old-token',$1,0,now()+interval '1 hour')`, fixture.recipientID); err != nil {
		t.Fatal(err)
	}
	oldAuthLinkHash := sha256.Sum256([]byte("old-auth-link"))
	if _, err := fixture.pool.Exec(ctx, `INSERT INTO auth_link_tokens(token_hash,user_id,purpose,expires_at) VALUES($1,$2,'email_verification',now()+interval '1 hour')`, oldAuthLinkHash[:], fixture.recipientID); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.usecase().Execute(ctx, fixture.rawToken, "FreshPassword1!", "", ulid.Make().String())
	if err != nil || !result.NeedsAutoLogin || result.PersonalAccessToken == "" {
		t.Fatalf("unverified account acceptance = %+v, error %v; want new PAT", result, err)
	}
	var verified bool
	var authVersion int64
	if err := fixture.pool.QueryRow(ctx, `SELECT email_verified_at IS NOT NULL,auth_version FROM users WHERE id=$1`, fixture.recipientID).Scan(&verified, &authVersion); err != nil || !verified || authVersion != 1 {
		t.Fatalf("upgraded account verified=%t auth_version=%d, error %v; want true/1", verified, authVersion, err)
	}
	var oldPATRevoked, oldLinkRevoked bool
	if err := fixture.pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM personal_access_tokens WHERE token='old-token'`).Scan(&oldPATRevoked); err != nil || !oldPATRevoked {
		t.Fatalf("old PAT revoked=%t, error %v; want true", oldPATRevoked, err)
	}
	if err := fixture.pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM auth_link_tokens WHERE token_hash=$1`, oldAuthLinkHash[:]).Scan(&oldLinkRevoked); err != nil || !oldLinkRevoked {
		t.Fatalf("old auth link revoked=%t, error %v; want true", oldLinkRevoked, err)
	}
}

func TestProjectInvitationAcceptanceRejectsExpiredAndRevokedTokens(t *testing.T) {
	for _, state := range []string{"expired", "revoked"} {
		t.Run(state, func(t *testing.T) {
			fixture := newInvitationAcceptanceFixture(t)
			if state == "expired" {
				_, err := fixture.pool.Exec(t.Context(), `UPDATE project_invitations SET expires_at=now()-interval '1 minute' WHERE id=$1`, fixture.invitationID)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				_, err := fixture.pool.Exec(t.Context(), `UPDATE project_invitations SET revoked_at=now() WHERE id=$1`, fixture.invitationID)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := fixture.usecase().Execute(t.Context(), fixture.rawToken, "NewPassword1!", "", ulid.Make().String()); !errors.Is(err, projectusecase.ErrInvalidProjectInvitation) {
				t.Fatalf("%s invitation acceptance error = %v; want invalid invitation", state, err)
			}
		})
	}
}

func TestProjectInvitationAcceptanceRejectsInvitationAfterAdminDemotion(t *testing.T) {
	fixture := newInvitationAcceptanceFixture(t)
	adminID := ulid.Make().String()
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Invite','Admin',$2,'unused',now(),'UTC')`, adminID, adminID+"@acceptance.test"); err != nil {
		t.Fatal(err)
	}
	fixture.userIDs = append(fixture.userIDs, adminID)
	if _, err := fixture.pool.Exec(t.Context(), `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,'admin',$3)`, fixture.projectID, adminID, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE project_invitations SET invited_by=$2 WHERE id=$1`, fixture.invitationID, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(t.Context(), `UPDATE project_members SET role_id='editor' WHERE project_id=$1 AND user_id=$2`, fixture.projectID, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.usecase().Execute(t.Context(), fixture.rawToken, "NewPassword1!", "", ulid.Make().String()); !errors.Is(err, projectusecase.ErrInvalidProjectInvitation) {
		t.Fatalf("demoted issuer invitation error = %v; want invalid invitation", err)
	}
}

func TestProjectInvitationAcceptanceRollsBackRecipientWhenMemberInsertFails(t *testing.T) {
	fixture := newInvitationAcceptanceFixture(t)
	trigger := "fail_project_invitation_member_" + ulid.Make().String()
	function := trigger + "_fn"
	if _, err := fixture.pool.Exec(t.Context(), fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced acceptance failure'; END; $$`, function)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(t.Context(), fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON project_members FOR EACH ROW WHEN (NEW.project_id = '%s') EXECUTE FUNCTION %s()`, trigger, fixture.projectID, function)); err != nil {
		_, _ = fixture.pool.Exec(t.Context(), fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, function))
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = fixture.pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON project_members`, trigger))
		_, _ = fixture.pool.Exec(context.Background(), fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, function))
	})
	if _, err := fixture.usecase().Execute(t.Context(), fixture.rawToken, "NewPassword1!", "", ulid.Make().String()); err == nil {
		t.Fatal("acceptance succeeded despite forced membership insert failure")
	}
	var userCount int
	if err := fixture.pool.QueryRow(t.Context(), `SELECT count(*)::integer FROM users WHERE email=$1`, fixture.email).Scan(&userCount); err != nil || userCount != 0 {
		t.Fatalf("recipient row count after rollback = %d, error %v; want zero", userCount, err)
	}
	var used bool
	if err := fixture.pool.QueryRow(t.Context(), `SELECT used_at IS NOT NULL FROM project_invitations WHERE id=$1`, fixture.invitationID).Scan(&used); err != nil || used {
		t.Fatalf("invitation used after rollback = %t, error %v; want false", used, err)
	}
}

func TestProjectInvitationAcceptanceSerializesConcurrentUses(t *testing.T) {
	fixture := newInvitationAcceptanceFixture(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := fixture.usecase().Execute(t.Context(), fixture.rawToken, "NewPassword1!", "", ulid.Make().String())
			results <- err
		}()
	}
	close(start)
	var success, invalid int
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, projectusecase.ErrInvalidProjectInvitation) {
			invalid++
		} else {
			t.Fatalf("concurrent acceptance error = %v", err)
		}
	}
	if success != 1 || invalid != 1 {
		t.Fatalf("concurrent results = success %d, invalid %d; want one each", success, invalid)
	}
}

type invitationAcceptanceFixture struct {
	pool         *pgxpool.Pool
	ownerID      string
	projectID    string
	invitationID string
	recipientID  string
	userIDs      []string
	email        string
	tokenBytes   []byte
	rawToken     string
}

func newInvitationAcceptanceFixture(t *testing.T) *invitationAcceptanceFixture {
	t.Helper()
	pool := testdb.Open(t)
	fixture := &invitationAcceptanceFixture{
		pool: pool, ownerID: ulid.Make().String(), projectID: ulid.Make().String(),
		invitationID: ulid.Make().String(), recipientID: ulid.Make().String(),
		email: ulid.Make().String() + "@acceptance.test", tokenBytes: make([]byte, 32),
	}
	fixture.userIDs = []string{fixture.ownerID, fixture.recipientID}
	if _, err := rand.Read(fixture.tokenBytes); err != nil {
		t.Fatal(err)
	}
	fixture.rawToken = base64.RawURLEncoding.EncodeToString(fixture.tokenBytes)
	hash := sha256.Sum256(fixture.tokenBytes)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Invite','Owner',$2,'unused',now(),'UTC')`, fixture.ownerID, fixture.ownerID+"@acceptance.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects(id,user_id,type,title,priority,changed_by) VALUES($1,$2,'other','Acceptance test','low',$2)`, fixture.projectID, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_invitations(id,project_id,email,role_id,invited_by,token_hash,expires_at) VALUES($1,$2,$3,'editor',$4,$5,$6)`, fixture.invitationID, fixture.projectID, fixture.email, fixture.ownerID, hash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, fixture.projectID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::text[]) OR email=$2`, fixture.userIDs, fixture.email)
	})
	return fixture
}

func (fixture *invitationAcceptanceFixture) usecase() *projectusecase.AcceptProjectInvitationUseCase {
	store := application.NewStore(fixture.pool)
	uow := application.NewProjectInvitationAcceptanceUOW(fixture.pool, store.Project, store.Auth)
	return projectusecase.NewAcceptProjectInvitationUseCase(uow, nil)
}
