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
	"github.com/Najah7/task2todaytodo/internal/application/shared"
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
	wantCalls := []string{"lookup", "lock_project", "lock_invitation", "add_member", "consume"}
	if !recipient.called {
		t.Fatal("recipient authentication was not invoked")
	}
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

func TestAcceptEditorInvitationRegistersAccountWithoutConsumingUntilTeamUpgrade(t *testing.T) {
	repository := &invitationAcceptanceRepositoryStub{invitation: validAcceptanceInvitation(), access: shared.ProjectWriteAccess{OwnerUserID: "owner-1", ActorPlanAccess: shared.PlanAccessFree, OwnerPlanAccess: shared.PlanAccessTeam, MemberCount: 1}}
	recipient := &invitationRecipientStub{result: InvitationRecipientResult{UserID: "recipient-1", NeedsAutoLogin: true, PersonalAccessToken: "pat"}}
	uc := NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: recipient}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }

	got, err := uc.Execute(context.Background(), acceptanceToken(t), "Password1!", "", "new-user")
	if err != nil {
		t.Fatalf("Execute() error = %v, want registration to commit pending Team upgrade", err)
	}
	if !got.NeedsAutoLogin || !got.RequiresTeamSubscription || got.PersonalAccessToken != "pat" {
		t.Fatalf("Execute() result = %+v, want login token and Team requirement", got)
	}
	if !recipient.called || repository.added || repository.consumed {
		t.Fatalf("recipient_called=%t member_added=%t invitation_consumed=%t; want true/false/false", recipient.called, repository.added, repository.consumed)
	}
}

func TestAcceptEditorInvitationRequiresTeamForExistingRecipient(t *testing.T) {
	repository := &invitationAcceptanceRepositoryStub{invitation: validAcceptanceInvitation(), access: shared.ProjectWriteAccess{OwnerUserID: "owner-1", ActorPlanAccess: shared.PlanAccessFree, OwnerPlanAccess: shared.PlanAccessTeam, MemberCount: 1}}
	recipient := &invitationRecipientStub{result: InvitationRecipientResult{UserID: "recipient-1"}}
	uc := NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: recipient}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }

	_, err := uc.Execute(context.Background(), acceptanceToken(t), "", "recipient-1", "")
	if !errors.Is(err, ErrInvitationTeamSubscriptionRequired) {
		t.Fatalf("Execute() error = %v, want Team subscription requirement", err)
	}
	if repository.added || repository.consumed {
		t.Fatalf("member_added=%t invitation_consumed=%t; want false/false", repository.added, repository.consumed)
	}
}

func TestAcceptViewerInvitationDoesNotRequireRecipientTeamButRequiresOwnerTeam(t *testing.T) {
	invitation := validAcceptanceInvitation()
	invitation.RoleID = "viewer"
	repository := &invitationAcceptanceRepositoryStub{invitation: invitation, added: true, consumed: true, access: shared.ProjectWriteAccess{OwnerUserID: "owner-1", OwnerPlanAccess: shared.PlanAccessTeam, MemberCount: 1}}
	recipient := &invitationRecipientStub{result: InvitationRecipientResult{UserID: "recipient-1"}}
	uc := NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: recipient}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }
	if _, err := uc.Execute(context.Background(), acceptanceToken(t), "", "recipient-1", ""); err != nil {
		t.Fatalf("viewer acceptance error = %v, want nil", err)
	}

	repository = &invitationAcceptanceRepositoryStub{invitation: invitation, access: shared.ProjectWriteAccess{OwnerUserID: "owner-1", OwnerPlanAccess: shared.PlanAccessFree, MemberCount: 1}}
	recipient = &invitationRecipientStub{result: InvitationRecipientResult{UserID: "recipient-1"}}
	uc = NewAcceptProjectInvitationUseCase(invitationAcceptanceUOWStub{repository: repository, recipient: recipient}, nil)
	uc.now = func() time.Time { return time.Unix(100, 0) }
	if _, err := uc.Execute(context.Background(), acceptanceToken(t), "", "recipient-1", ""); !errors.Is(err, ErrProjectOwnerTeamSubscriptionRequired) {
		t.Fatalf("viewer acceptance after owner lapse error = %v, want owner Team subscription requirement", err)
	}
	if recipient.called || repository.added || repository.consumed {
		t.Fatalf("recipient_called=%t member_added=%t invitation_consumed=%t; want false/false/false", recipient.called, repository.added, repository.consumed)
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
	access     shared.ProjectWriteAccess
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

func (repository *invitationAcceptanceRepositoryStub) GetProjectWriteAccess(_ context.Context, actorID string, _ string) (shared.ProjectWriteAccess, error) {
	access := repository.access
	if access.ActorPlanAccess == "" {
		access.ActorPlanAccess = shared.PlanAccessTeam
	}
	if actorID == "owner-1" && repository.access.OwnerPlanAccess == "" {
		access.OwnerPlanAccess = shared.PlanAccessTeam
	}
	return access, nil
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
