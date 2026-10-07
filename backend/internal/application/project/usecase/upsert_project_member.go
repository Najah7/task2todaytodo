package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type UpsertProjectMemberUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewUpsertProjectMemberUseCase(uow UOW, logger logging.Logger) *UpsertProjectMemberUseCase {
	return &UpsertProjectMemberUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *UpsertProjectMemberUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID, role string) (err error) {
	defer func() { logProjectMemberFailure(uc.logger, ctx, "UpsertProjectMemberUseCase.Execute", err) }()
	if !validProjectMemberInput(string(actor), string(project), string(member), role) {
		return ErrProjectMemberInvalidInput
	}

	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		allowed, err := repo.CheckProjectMemberUpsertPermission(ctx, actor, project, member)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrPermissionDenied
		}
		if err := repo.LockProjectForMemberChange(ctx, project); err != nil {
			return err
		}
		grant, err := domain.NewProjectMember(project, member, role, actor)
		if err != nil {
			return ErrProjectMemberInvalidInput
		}
		return repo.UpsertProjectMember(ctx, grant)
	})
}
