package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
)

func TestAcceptProjectInvitationConsumesOnlyAfterMembershipAdded(t *testing.T) {
	invitation := validAcceptanceInvitation()
	repository := &invitationAcceptanceRepositoryStub{invitation: invitation, added: true, consumed: true}
	recipient := &invitationRecipientStub{result: InvitationRecipientResult{UserID: "recipient-1", NeedsAutoLogin: true, PersonalAccessToken: "pat"}}
	uc := NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: recipient}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }
	token := acceptanceToken(t)

	got, err := uc.Execute(context.Background(), token, "Password1!", "", "new-user")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !got.NeedsAutoLogin || got.PersonalAccessToken != "pat" {
		t.Fatalf("Execute() result = %+v", got)
	}
	wantCalls := []string{"lookup", "lock_project", "lock_invitation", "recipient", "add_member", "consume"}
	if len(repository.calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", repository.calls, wantCalls)
	}
	for i := range wantCalls {
		if repository.calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v, want %v", repository.calls, wantCalls)
		}
	}
}

func TestAcceptProjectInvitationRejectsDemotedInviter(t *testing.T) {
	invitation := validAcceptanceInvitation()
	invitation.InviterRoleID = "editor"
	repository := &invitationAcceptanceRepositoryStub{invitation: invitation}
	recipient := &invitationRecipientStub{}
	uc := NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: recipient}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }

	_, err := uc.Execute(context.Background(), acceptanceToken(t), "Password1!", "", "new-user")
	if !errors.Is(err, ErrInvalidProjectInvitation) {
		t.Fatalf("Execute() error = %v, want invalid invitation", err)
	}
	if recipient.called {
		t.Fatal("recipient authentication ran after inviter lost management permission")
	}
}

func TestAcceptProjectInvitationRejectsExpiredInvitation(t *testing.T) {
	invitation := validAcceptanceInvitation()
	invitation.ExpiresAt = time.Unix(99, 0)
	repository := &invitationAcceptanceRepositoryStub{invitation: invitation}
	uc := NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: &invitationRecipientStub{}}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }

	_, err := uc.Execute(context.Background(), acceptanceToken(t), "Password1!", "", "new-user")
	if !errors.Is(err, ErrInvalidProjectInvitation) {
		t.Fatalf("Execute() error = %v, want invalid invitation", err)
	}
}

func validAcceptanceInvitation() dao.ProjectInvitation {
	return dao.ProjectInvitation{
		ID: "invite-1", ProjectID: "project-1", Email: "recipient@example.com", RoleID: "editor",
		InvitedBy: "admin-1", ExpiresAt: time.Unix(200, 0), ProjectOwnerID: "owner-1", InviterRoleID: "admin",
	}
}

func acceptanceToken(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

type invitationAcceptanceUOWStub struct {
	repository *invitationAcceptanceRepositoryStub
	recipient  *invitationRecipientStub
}

func (uow invitationAcceptanceUOWStub) Do(ctx context.Context, fn func(context.Context, InvitationAcceptanceRepositories) error) error {
	return fn(ctx, invitationAcceptanceRepositoriesStub{repository: uow.repository, recipient: uow.recipient})
}

type invitationAcceptanceRepositoriesStub struct {
	repository *invitationAcceptanceRepositoryStub
	recipient  *invitationRecipientStub
}

func (repositories invitationAcceptanceRepositoriesStub) Invitations() InvitationAcceptanceRepository {
	return repositories.repository
}
func (repositories invitationAcceptanceRepositoriesStub) Recipient() InvitationRecipientAcceptor {
	return repositories.recipient
}

type invitationAcceptanceRepositoryStub struct {
	invitation dao.ProjectInvitation
	added      bool
	consumed   bool
	calls      []string
}

func (repository *invitationAcceptanceRepositoryStub) GetProjectInvitationByTokenHash(context.Context, []byte) (dao.ProjectInvitation, error) {
	repository.calls = append(repository.calls, "lookup")
	return repository.invitation, nil
}
func (repository *invitationAcceptanceRepositoryStub) GetProjectInvitationByTokenHashForUpdate(context.Context, []byte) (dao.ProjectInvitation, error) {
	repository.calls = append(repository.calls, "lock_invitation")
	return repository.invitation, nil
}
func (repository *invitationAcceptanceRepositoryStub) LockProjectForInvitationAcceptance(context.Context, domain.ProjectID) error {
	repository.calls = append(repository.calls, "lock_project")
	return nil
}
func (repository *invitationAcceptanceRepositoryStub) ConsumeProjectInvitation(context.Context, []byte) (bool, error) {
	repository.calls = append(repository.calls, "consume")
	return repository.consumed, nil
}
func (repository *invitationAcceptanceRepositoryStub) AcceptProjectInvitationMember(context.Context, domain.ProjectID, domain.UserID, string, string) (bool, error) {
	repository.calls = append(repository.calls, "add_member")
	return repository.added, nil
}

type invitationRecipientStub struct {
	result InvitationRecipientResult
	called bool
	err    error
}

func (recipient *invitationRecipientStub) AcceptInvitationRecipient(context.Context, string, string, string, string) (InvitationRecipientResult, error) {
	recipient.called = true
	return recipient.result, recipient.err
}
