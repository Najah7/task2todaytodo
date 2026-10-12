package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	sharedpagination "github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ListProjectMembersUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewListProjectMembersUseCase(uow UOW, logger telemetry.Logger) *ListProjectMembersUseCase {
	return &ListProjectMembersUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

var ErrInvalidProjectRosterPage = errors.New("invalid project roster page")

type ProjectRosterPageRequest struct {
	Size   int
	Anchor *CursorAnchor
}

type ProjectRosterPage struct {
	Items                  []dao.ProjectMember
	TotalCount             int64
	MemberCount            int64
	PendingInvitationCount int64
	ActorRole              string
	CanInvite              bool
	AssignableRoles        []ProjectMemberRoleOption
	Next                   *CursorAnchor
	Previous               *CursorAnchor
}

func (uc *ListProjectMembersUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, request ProjectRosterPageRequest) (page ProjectRosterPage, err error) {
	defer func() { logProjectMemberFailure(uc.logger, ctx, "ListProjectMembersUseCase.Execute", err) }()
	if !validProjectMemberInput(string(actor), string(project)) {
		return ProjectRosterPage{}, ErrProjectMemberInvalidInput
	}
	if request.Size == 0 {
		request.Size = sharedpagination.DefaultPageSize
	}
	if request.Size < 1 || request.Size > sharedpagination.MaxPageSize {
		return ProjectRosterPage{}, ErrInvalidProjectRosterPage
	}

	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		if _, err := repo.LockByUserIDWithPermission(ctx, actor, project, shared.ProjectMemberRead()); err != nil {
			return err
		}
		actorRole, err := repo.GetProjectMemberActorRole(ctx, actor, project)
		if err != nil {
			return err
		}
		result, err := repo.ListProjectRosterPage(ctx, actor, project, request.Size+1, request.Anchor)
		if err != nil {
			return err
		}
		items, hasMore := sharedpagination.Window(result.Items, request.Size)
		direction := "forward"
		if request.Anchor != nil && request.Anchor.Direction == "backward" {
			direction = "backward"
			for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
				items[left], items[right] = items[right], items[left]
			}
		}
		page.Items = items
		page.TotalCount = result.TotalCount
		page.MemberCount = result.MemberCount
		page.PendingInvitationCount = result.PendingInvitationCount
		page.ActorRole = actorRole
		page.CanInvite = CanInviteProjectMembers(actorRole)
		page.AssignableRoles = AssignableProjectMemberRoles(actorRole)
		if len(items) > 0 {
			first, last := items[0], items[len(items)-1]
			if (direction == "forward" && hasMore) || direction == "backward" {
				page.Next = &CursorAnchor{At: last.CursorCreatedAt, ID: last.CursorID, Direction: "forward"}
			}
			if (direction == "backward" && hasMore) || (direction == "forward" && request.Anchor != nil) {
				page.Previous = &CursorAnchor{At: first.CursorCreatedAt, ID: first.CursorID, Direction: "backward"}
			}
		}
		return err
	})
	if err != nil {
		return ProjectRosterPage{}, err
	}
	if page.Items == nil {
		page.Items = []dao.ProjectMember{}
	}
	return page, nil
}
