package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListProjectMembersUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewListProjectMembersUseCase(uow UOW, logger logging.Logger) *ListProjectMembersUseCase {
	return &ListProjectMembersUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *ListProjectMembersUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID) (members []dao.ProjectMember, err error) {
	defer func() { logProjectMemberFailure(uc.logger, ctx, "ListProjectMembersUseCase.Execute", err) }()
	if !validProjectMemberInput(string(actor), string(project)) {
		return nil, ErrProjectMemberInvalidInput
	}

	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := repos.Projects().LockByUserIDWithPermission(ctx, actor, project, shared.ProjectMemberRead()); err != nil {
			return err
		}
		members, err = repos.Projects().ListProjectMembers(ctx, actor, project)
		return err
	})
	if err != nil {
		return nil, err
	}
	return members, nil
}
