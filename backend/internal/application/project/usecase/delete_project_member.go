package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type DeleteProjectMemberUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewDeleteProjectMemberUseCase(uow UOW, logger logging.Logger) *DeleteProjectMemberUseCase {
	return &DeleteProjectMemberUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *DeleteProjectMemberUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) (err error) {
	defer func() { logProjectMemberFailure(uc.logger, ctx, "DeleteProjectMemberUseCase.Execute", err) }()
	if !validProjectMemberInput(string(actor), string(project), string(member)) {
		return ErrProjectMemberInvalidInput
	}

	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository, children ProjectChildrenDeleter) error {
		if _, err := repo.LockByUserIDWithPermission(ctx, actor, project, shared.ProjectMemberDelete()); err != nil {
			return err
		}
		if err := children.ReassignTasksAfterProjectMemberRemoval(ctx, string(actor), string(project), string(member)); err != nil {
			return err
		}
		if err := children.ReassignSchedulesAfterProjectMemberRemoval(ctx, string(actor), string(project), string(member)); err != nil {
			return err
		}
		return repo.DeleteProjectMember(ctx, actor, project, member)
	})
}
