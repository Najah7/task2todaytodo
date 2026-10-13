package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
)

var ErrTeamSubscriptionRequired = shared.ErrTeamSubscriptionRequired

type projectWriteAccessProvider interface {
	ProjectWriteAccess() shared.ProjectWriteAccessReader
}

func requireProjectWriteAccess(ctx context.Context, repos Repositories, actorID, projectID string) error {
	if projectID == "" {
		return nil
	}
	provider, ok := repos.(projectWriteAccessProvider)
	if !ok || provider.ProjectWriteAccess() == nil {
		return ErrPermissionDenied
	}
	access, err := provider.ProjectWriteAccess().GetProjectWriteAccess(ctx, actorID, projectID)
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

func restrictTaskWriteCapabilities(ctx context.Context, repos Repositories, actorID string, task *dao.Task) error {
	if task.ProjectID == "" {
		return nil
	}
	provider, ok := repos.(projectWriteAccessProvider)
	if !ok || provider.ProjectWriteAccess() == nil {
		restrictTaskWrites(task)
		return nil
	}
	access, err := provider.ProjectWriteAccess().GetProjectWriteAccess(ctx, actorID, task.ProjectID)
	if err != nil {
		return err
	}
	if !access.AllowsWrite() {
		restrictTaskWrites(task)
	}
	return nil
}

func restrictTaskWrites(task *dao.Task) {
	task.CanUpdate = false
	task.CanAssign = false
	task.CanAddActionItem = false
	task.CanAddToToday = false
	task.EstimateEditable = false
}
