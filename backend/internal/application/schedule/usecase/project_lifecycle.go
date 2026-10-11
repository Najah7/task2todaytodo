package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type scheduleProjectMutationSnapshots map[string]shared.ProjectWorkState

func lockAndCaptureScheduleProjects(ctx context.Context, repos Repositories, projectIDs []string, asOf time.Time) (scheduleProjectMutationSnapshots, error) {
	ids := uniqueSortedProjectIDs(projectIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	lifecycle := repos.ProjectLifecycle()
	if lifecycle == nil {
		return nil, ErrProjectLifecycleUnavailable
	}
	for _, id := range ids {
		if err := lifecycle.LockParent(ctx, id); err != nil {
			if errors.Is(err, shared.ErrProjectUnavailable) {
				return nil, ErrScheduleProjectNotFound
			}
			return nil, err
		}
	}
	before := make(scheduleProjectMutationSnapshots, len(ids))
	for _, id := range ids {
		state, err := lifecycle.CaptureWorkState(ctx, id, asOf)
		if err != nil {
			if errors.Is(err, shared.ErrProjectUnavailable) {
				return nil, ErrScheduleProjectNotFound
			}
			return nil, err
		}
		before[id] = state
	}
	return before, nil
}

func requireScheduleProjectPermission(ctx context.Context, schedules ScheduleRepository, actorID domain.UserID, projectID domain.ProjectID, capability shared.Capability) error {
	allowed, err := schedules.CheckProjectPermission(ctx, actorID, projectID, capability)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermissionDenied
	}
	return nil
}

func finishScheduleProjectMutation(ctx context.Context, repos Repositories, actorID domain.UserID, before scheduleProjectMutationSnapshots, asOf time.Time) error {
	if len(before) == 0 {
		return nil
	}
	lifecycle := repos.ProjectLifecycle()
	if lifecycle == nil {
		return ErrProjectLifecycleUnavailable
	}
	for _, id := range sortedSnapshotProjectIDs(before) {
		if err := lifecycle.ReconcileWorkState(ctx, string(actorID), id, before[id], asOf); err != nil {
			return err
		}
	}
	return nil
}

func uniqueSortedProjectIDs(projectIDs []string) []string {
	seen := make(map[string]struct{}, len(projectIDs))
	ids := make([]string, 0, len(projectIDs))
	for _, id := range projectIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedSnapshotProjectIDs(before scheduleProjectMutationSnapshots) []string {
	ids := make([]string, 0, len(before))
	for id := range before {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
