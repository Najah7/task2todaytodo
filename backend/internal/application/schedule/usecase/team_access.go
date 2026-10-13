package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

var ErrTeamSubscriptionRequired = shared.ErrTeamSubscriptionRequired

type projectWriteAccessProvider interface {
	ProjectWriteAccess() shared.ProjectWriteAccessReader
}

func requireScheduleProjectWrite(ctx context.Context, repos Repositories, actorID string, projectIDs ...string) error {
	provider, ok := repos.(projectWriteAccessProvider)
	if !ok || provider.ProjectWriteAccess() == nil {
		for _, projectID := range projectIDs {
			if projectID != "" {
				return ErrPermissionDenied
			}
		}
		return nil
	}
	seen := make(map[string]struct{}, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID == "" {
			continue
		}
		if _, ok := seen[projectID]; ok {
			continue
		}
		seen[projectID] = struct{}{}
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
	}
	return nil
}

func restrictScheduleWriteCapabilities(ctx context.Context, repos Repositories, actorID string, occurrence *dao.ScheduleOccurrence) error {
	if occurrence.ProjectID == "" {
		return nil
	}
	provider, ok := repos.(projectWriteAccessProvider)
	if !ok || provider.ProjectWriteAccess() == nil {
		restrictScheduleWrites(occurrence)
		return nil
	}
	access, err := provider.ProjectWriteAccess().GetProjectWriteAccess(ctx, actorID, occurrence.ProjectID)
	if err != nil {
		return err
	}
	if !access.AllowsWrite() {
		restrictScheduleWrites(occurrence)
	}
	return nil
}

func restrictScheduleWrites(occurrence *dao.ScheduleOccurrence) {
	occurrence.CanUpdate = false
	occurrence.CanDelete = false
	occurrence.CanSkip = false
	occurrence.CanAssign = false
	occurrence.CanUpdateFollowing = false
	occurrence.CanDeleteFollowing = false
}
