package usecase

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var (
	ErrTeamSubscriptionRequired             = shared.ErrTeamSubscriptionRequired
	ErrProjectOwnerTeamSubscriptionRequired = errors.New("Project owner needs an active Team subscription")
)

var ErrProjectWriteAccessUnavailable = errors.New("project write access reader is unavailable")

func projectWriteAccessReader(value any) (shared.ProjectWriteAccessReader, error) {
	reader, ok := value.(shared.ProjectWriteAccessReader)
	if !ok || reader == nil {
		return nil, ErrProjectWriteAccessUnavailable
	}
	return reader, nil
}

func requireProjectWriter(ctx context.Context, value any, actor domain.UserID, project domain.ProjectID) error {
	reader, err := projectWriteAccessReader(value)
	if err != nil {
		return err
	}
	return requireProjectWriterAccess(ctx, reader, actor, project)
}

func requireTeamInvitation(ctx context.Context, value any, actor domain.UserID, project domain.ProjectID) error {
	reader, err := projectWriteAccessReader(value)
	if err != nil {
		return err
	}
	return requireTeamInvitationAccess(ctx, reader, actor, project)
}

func requireMemberRemoval(ctx context.Context, value any, actor domain.UserID, project domain.ProjectID) error {
	reader, err := projectWriteAccessReader(value)
	if err != nil {
		return err
	}
	access, err := reader.GetProjectWriteAccess(ctx, string(actor), string(project))
	if err != nil {
		return err
	}
	if access.AllowsWrite() || (access.OwnerUserID == string(actor) && !access.OwnerPlanAccess.IsTeam()) {
		return nil
	}
	return ErrPermissionDenied
}

func requireProjectWriterAccess(ctx context.Context, reader shared.ProjectWriteAccessReader, actor domain.UserID, project domain.ProjectID) error {
	access, err := reader.GetProjectWriteAccess(ctx, string(actor), string(project))
	if err != nil {
		return err
	}
	if !access.AllowsWrite() {
		if access.MemberCount > 0 && (!access.ActorPlanAccess.IsTeam() || !access.OwnerPlanAccess.IsTeam()) {
			return ErrTeamSubscriptionRequired
		}
		return ErrPermissionDenied
	}
	return nil
}

func requireTeamInvitationAccess(ctx context.Context, reader shared.ProjectWriteAccessReader, actor domain.UserID, project domain.ProjectID) error {
	access, err := reader.GetProjectWriteAccess(ctx, string(actor), string(project))
	if err != nil {
		return err
	}
	if !access.ActorPlanAccess.IsTeam() || !access.OwnerPlanAccess.IsTeam() {
		return ErrTeamSubscriptionRequired
	}
	return nil
}

func requireTeamInviteeAccess(ctx context.Context, reader shared.ProjectWriteAccessReader, invitee domain.UserID, project domain.ProjectID) error {
	access, err := reader.GetProjectWriteAccess(ctx, string(invitee), string(project))
	if err != nil {
		return err
	}
	if !access.ActorPlanAccess.IsTeam() || !access.OwnerPlanAccess.IsTeam() {
		return ErrTeamSubscriptionRequired
	}
	return nil
}

func requireTeamProjectOwner(ctx context.Context, reader shared.ProjectWriteAccessReader, owner domain.UserID, project domain.ProjectID) error {
	access, err := reader.GetProjectWriteAccess(ctx, string(owner), string(project))
	if err != nil {
		return err
	}
	if !access.OwnerPlanAccess.IsTeam() {
		return ErrProjectOwnerTeamSubscriptionRequired
	}
	return nil
}
