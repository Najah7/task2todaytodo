//go:build integration

package project_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectrepo "github.com/Najah7/task2todaytodo/internal/application/project/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/oklog/ulid/v2"
)

type recordedInvitation struct {
	To, FrontendURL, Token, ProjectTitle, RoleName string
	Lifetime                                       time.Duration
}

type recordingProjectInvitationSender struct {
	items []recordedInvitation
}

func (s *recordingProjectInvitationSender) SendProjectInvitation(_ context.Context, to, frontendURL, token, title, role string, lifetime time.Duration) error {
	s.items = append(s.items, recordedInvitation{To: to, FrontendURL: frontendURL, Token: token, ProjectTitle: title, RoleName: role, Lifetime: lifetime})
	return nil
}

func TestProjectInvitationCreateResendRevokeAndTokenRotation(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	fixture := newProjectRepositoryFixture(t, pool)
	if _, err := pool.Exec(t.Context(), `UPDATE users SET timezone='UTC' WHERE id=$1`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	fixture.commit(t)
	sender := &recordingProjectInvitationSender{}
	unitOfWork := projectRepositoryTestUOW{pool}
	repo := projectrepo.NewProjectRepository(pool)
	create := projectusecase.NewCreateProjectInvitationUseCase(unitOfWork, sender, "https://app.example.test", nil)
	resend := projectusecase.NewResendProjectInvitationUseCase(unitOfWork, sender, "https://app.example.test", nil)
	revoke := projectusecase.NewRevokeProjectInvitationUseCase(unitOfWork, nil)
	preview := projectusecase.NewPreviewProjectInvitationUseCase(repo, nil)
	actor, project := domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID)
	email := "invite-" + strings.ToLower(ulid.Make().String()) + "@example.test"
	invitation, err := create.Execute(t.Context(), actor, project, ulid.Make().String(), email, "viewer")
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if len(sender.items) != 1 || sender.items[0].Token == "" || invitation.Email != email {
		t.Fatalf("create delivery = %+v, invitation = %+v", sender.items, invitation)
	}
	oldToken := sender.items[0].Token
	decoded, err := base64.RawURLEncoding.DecodeString(oldToken)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	digest := sha256.Sum256(decoded)
	var storedHash []byte
	if err := pool.QueryRow(t.Context(), `SELECT token_hash FROM project_invitations WHERE id=$1`, invitation.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if string(storedHash) != string(digest[:]) || string(storedHash) == oldToken {
		t.Fatal("invitation token must be stored as its SHA-256 digest")
	}
	initial, err := preview.Execute(t.Context(), oldToken)
	if err != nil || initial.Status != "pending" {
		t.Fatalf("preview created invitation = %+v, error %v", initial, err)
	}

	// Resend has a per-address cooldown. Clear the recorded attempt to test
	// token replacement without waiting for the production cooldown interval.
	emailHash := sha256.Sum256([]byte(strings.ToLower(email)))
	if _, err := pool.Exec(t.Context(), `DELETE FROM email_delivery_attempts WHERE email_hash=$1`, emailHash[:]); err != nil {
		t.Fatal(err)
	}
	resent, err := resend.Execute(t.Context(), actor, project, invitation.ID)
	if err != nil {
		t.Fatalf("resend invitation: %v", err)
	}
	if len(sender.items) != 2 || sender.items[1].Token == oldToken || !resent.ExpiresAt.After(invitation.ExpiresAt) {
		t.Fatalf("resend did not rotate token and extend expiry: deliveries=%+v invitation=%+v", sender.items, resent)
	}
	if _, err := preview.Execute(t.Context(), oldToken); !errors.Is(err, projectusecase.ErrProjectInvitationTokenInvalid) {
		t.Fatalf("old token after resend error=%v, want invalid token", err)
	}
	current, err := preview.Execute(t.Context(), sender.items[1].Token)
	if err != nil || current.Status != "pending" {
		t.Fatalf("preview resent invitation = %+v, error %v", current, err)
	}
	if err := revoke.Execute(t.Context(), actor, project, invitation.ID); err != nil {
		t.Fatalf("revoke invitation: %v", err)
	}
	revoked, err := preview.Execute(t.Context(), sender.items[1].Token)
	if err != nil || revoked.Status != "revoked" {
		t.Fatalf("preview revoked invitation = %+v, error %v", revoked, err)
	}
	if _, err := resend.Execute(t.Context(), actor, project, invitation.ID); !errors.Is(err, projectusecase.ErrProjectInvitationNotFound) {
		t.Fatalf("resend revoked invitation error=%v, want not found", err)
	}
}

func TestProjectRosterPageCountsInvitationStatesAndSupportsPreviousCursor(t *testing.T) {
	pool := projectRepositoryIntegrationPool(t)
	fixture := newProjectRepositoryFixture(t, pool)
	acceptedOne, acceptedTwo := fixture.user(t), fixture.user(t)
	fixture.addMember(t, acceptedOne, "editor")
	fixture.addMember(t, acceptedTwo, "viewer")
	fixture.commit(t)
	sender := &recordingProjectInvitationSender{}
	create := projectusecase.NewCreateProjectInvitationUseCase(projectRepositoryTestUOW{pool}, sender, "https://app.example.test", nil)
	actor, project := domain.UserID(fixture.ownerID), domain.ProjectID(fixture.projectID)
	expiredID, pendingID := ulid.Make().String(), ulid.Make().String()
	if _, err := create.Execute(t.Context(), actor, project, expiredID, "expired-"+strings.ToLower(ulid.Make().String())+"@example.test", "viewer"); err != nil {
		t.Fatalf("create invitation to expire: %v", err)
	}
	if _, err := create.Execute(t.Context(), actor, project, pendingID, "pending-"+strings.ToLower(ulid.Make().String())+"@example.test", "editor"); err != nil {
		t.Fatalf("create pending invitation: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE project_invitations SET expires_at=now()-interval '1 minute' WHERE id=$1`, expiredID); err != nil {
		t.Fatal(err)
	}
	list := projectusecase.NewListProjectMembersUseCase(projectRepositoryTestUOW{pool}, nil)
	first, err := list.Execute(t.Context(), actor, project, projectusecase.ProjectRosterPageRequest{Size: 2})
	if err != nil {
		t.Fatalf("list first roster page: %v", err)
	}
	if first.TotalCount != 5 || first.MemberCount != 3 || first.PendingInvitationCount != 1 || !first.CanInvite || first.Next == nil || first.Previous != nil {
		t.Fatalf("first roster page counts/capabilities = %+v", first)
	}
	second, err := list.Execute(t.Context(), actor, project, projectusecase.ProjectRosterPageRequest{Size: 2, Anchor: first.Next})
	if err != nil {
		t.Fatalf("list second roster page: %v", err)
	}
	if second.Previous == nil {
		t.Fatalf("second roster page missing previous cursor: %+v", second)
	}
	if second.Next == nil {
		t.Fatalf("second roster page missing next cursor: %+v", second)
	}
	third, err := list.Execute(t.Context(), actor, project, projectusecase.ProjectRosterPageRequest{Size: 2, Anchor: second.Next})
	if err != nil {
		t.Fatalf("list third roster page: %v", err)
	}
	if third.Next != nil || len(third.Items) != 1 {
		t.Fatalf("third roster page = %+v; want final one item", third)
	}
	back, err := list.Execute(t.Context(), actor, project, projectusecase.ProjectRosterPageRequest{Size: 2, Anchor: second.Previous})
	if err != nil {
		t.Fatalf("list previous roster page: %v", err)
	}
	if len(first.Items) != len(back.Items) {
		t.Fatalf("back page length=%d, first length=%d", len(back.Items), len(first.Items))
	}
	for i := range first.Items {
		if back.Items[i].CursorID != first.Items[i].CursorID {
			t.Fatalf("back page cursor IDs=%q, first page IDs=%q", back.Items[i].CursorID, first.Items[i].CursorID)
		}
	}

	seen := make(map[string]string)
	for _, page := range []projectusecase.ProjectRosterPage{first, second, third} {
		for _, item := range page.Items {
			seen[item.Status+":"+item.Email] = item.Status
		}
	}
	var hasExpired, hasPending bool
	for _, status := range seen {
		hasExpired = hasExpired || status == "expired"
		hasPending = hasPending || status == "pending"
	}
	if !hasExpired || !hasPending {
		t.Fatalf("roster pages did not include pending and expired invitations: %v", seen)
	}
}
