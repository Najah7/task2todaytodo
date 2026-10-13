package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type UpsertProjectMemberUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewUpsertProjectMemberUseCase(uow UOW, logger telemetry.Logger) *UpsertProjectMemberUseCase {
	return &UpsertProjectMemberUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *UpsertProjectMemberUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID, role string) (err error) {
	defer func() { logProjectMemberFailure(uc.logger, ctx, "UpsertProjectMemberUseCase.Execute", err) }()
	if !validProjectMemberInput(string(actor), string(project), string(member), role) || role == "sysadmin" {
		return ErrProjectMemberInvalidInput
	}

	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		if err := repo.LockProjectForMemberChange(ctx, project); err != nil {
			return err
		}
		allowed, err := repo.CheckProjectMemberUpsertPermission(ctx, actor, project, member, role)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrPermissionDenied
		}
		if err := requireTeamInvitation(ctx, repo, actor, project); err != nil {
			return err
		}
		if role == "admin" || role == "editor" {
			reader, err := projectWriteAccessReader(repo)
			if err != nil {
				return err
			}
			if err := requireTeamInviteeAccess(ctx, reader, member, project); err != nil {
				return err
			}
		}
		grant, err := domain.NewProjectMember(project, member, role, actor)
		if err != nil {
			return ErrProjectMemberInvalidInput
		}
		if err := repo.UpsertProjectMember(ctx, grant); err != nil {
			return err
		}
		return repo.RevokeInvitationsForInviterWithoutPermission(ctx, project, member)
	})
}
