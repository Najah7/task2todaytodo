package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
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

	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Projects().LockByUserIDWithPermission(ctx, actor, project, shared.ProjectMemberDelete()); err != nil {
			return err
		}
		if err := repos.Tasks().ReassignProjectMemberTasks(ctx, actor, project, member); err != nil {
			return err
		}
		return repos.Projects().DeleteProjectMember(ctx, actor, project, member)
	})
}
