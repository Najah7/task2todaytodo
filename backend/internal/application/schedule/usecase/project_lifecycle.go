package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type scheduleProjectMutationSnapshots map[string]shared.ProjectWorkState

func lockAndCaptureScheduleProjects(
	ctx context.Context,
	repos Repositories,
	projectIDs []string,
	asOf time.Time,
) (scheduleProjectMutationSnapshots, error) {
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

func lockScheduleAndCaptureProjectState(
	ctx context.Context,
	repos Repositories,
	actorID domain.UserID,
	scheduleID domain.ScheduleID,
	asOf time.Time,
	capability shared.Capability,
) (dao.Schedule, scheduleProjectMutationSnapshots, error) {
	schedules := repos.Schedules()
	candidate, err := schedules.GetByUserIDWithPermission(ctx, actorID, scheduleID, capability)
	if err != nil {
		return dao.Schedule{}, nil, err
	}
	seriesID := candidate.SeriesID
	if seriesID == "" {
		seriesID = candidate.ID
	}
	before, err := lockAndCaptureScheduleProjects(ctx, repos, []string{candidate.ProjectID}, asOf)
	if err != nil {
		return dao.Schedule{}, nil, err
	}

	current, err := schedules.GetByUserIDWithPermission(ctx, actorID, scheduleID, capability)
	if err != nil {
		return dao.Schedule{}, nil, err
	}
	if !sameScheduleProjectRelation(candidate, current) {
		return dao.Schedule{}, nil, ErrScheduleProjectChanged
	}
	if err := schedules.LockSeriesProjectForMutation(ctx, actorID, domain.ScheduleID(seriesID), capability); err != nil {
		if errors.Is(err, ErrScheduleScopeInvalid) {
			return dao.Schedule{}, nil, ErrScheduleProjectChanged
		}
		return dao.Schedule{}, nil, err
	}
	locked, err := schedules.GetByUserIDWithPermission(ctx, actorID, scheduleID, capability)
	if err != nil {
		return dao.Schedule{}, nil, err
	}
	if !sameScheduleProjectRelation(candidate, locked) {
		return dao.Schedule{}, nil, ErrScheduleProjectChanged
	}
	return locked, before, nil
}

func sameScheduleProjectRelation(left, right dao.Schedule) bool {
	return left.ProjectID == right.ProjectID && left.SeriesID == right.SeriesID
}

func requireScheduleProjectPermission(
	ctx context.Context,
	schedules ScheduleRepository,
	actorID domain.UserID,
	projectID domain.ProjectID,
	capability shared.Capability,
) error {
	allowed, err := schedules.CheckProjectPermission(ctx, actorID, projectID, capability)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermissionDenied
	}
	return nil
}

func finishScheduleProjectMutation(
	ctx context.Context,
	repos Repositories,
	actorID domain.UserID,
	before scheduleProjectMutationSnapshots,
	asOf time.Time,
) error {
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

func withScheduleProjectMutation(
	ctx context.Context,
	repos Repositories,
	actorID domain.UserID,
	scheduleID domain.ScheduleID,
	asOf time.Time,
	capability shared.Capability,
	mutate func(dao.Schedule, scheduleProjectMutationSnapshots) error,
) error {
	schedule, before, err := lockScheduleAndCaptureProjectState(ctx, repos, actorID, scheduleID, asOf, capability)
	if err != nil {
		return err
	}
	if err := mutate(schedule, before); err != nil {
		return err
	}
	return finishScheduleProjectMutation(ctx, repos, actorID, before, asOf)
}

func withScheduleProjectMove(
	ctx context.Context,
	repos Repositories,
	actorID domain.UserID,
	scheduleID domain.ScheduleID,
	targetProjectID *domain.ProjectID,
	expectedProjectID *domain.ProjectID,
	asOf time.Time,
) error {
	schedules := repos.Schedules()
	candidate, err := schedules.GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleUpdate())
	if err != nil {
		return err
	}
	if expectedProjectID != nil && candidate.ProjectID != string(*expectedProjectID) {
		return domain.ErrScheduleNotFound
	}
	target := ""
	if targetProjectID != nil {
		target = string(*targetProjectID)
	}
	if candidate.ProjectID != "" {
		if err := requireScheduleProjectPermission(ctx, schedules, actorID, domain.ProjectID(candidate.ProjectID), shared.ScheduleUpdate()); err != nil {
			return err
		}
	}
	if target != "" {
		if err := requireScheduleProjectPermission(ctx, schedules, actorID, domain.ProjectID(target), shared.ScheduleCreate()); err != nil {
			return err
		}
	}
	before, err := lockAndCaptureScheduleProjects(ctx, repos, []string{candidate.ProjectID, target}, asOf)
	if err != nil {
		return err
	}
	current, err := schedules.GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleUpdate())
	if err != nil {
		return err
	}
	if !sameScheduleProjectRelation(candidate, current) {
		return ErrScheduleProjectChanged
	}
	seriesID := candidate.SeriesID
	if seriesID == "" {
		seriesID = candidate.ID
	}
	if err := schedules.LockSeriesProjectForMutation(ctx, actorID, domain.ScheduleID(seriesID), shared.ScheduleUpdate()); err != nil {
		if errors.Is(err, ErrScheduleScopeInvalid) {
			return ErrScheduleProjectChanged
		}
		return err
	}
	locked, err := schedules.GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleUpdate())
	if err != nil {
		return err
	}
	if !sameScheduleProjectRelation(candidate, locked) {
		return ErrScheduleProjectChanged
	}
	if target == candidate.ProjectID {
		return finishScheduleProjectMutation(ctx, repos, actorID, before, asOf)
	}
	if err := schedules.SetProjectByUserID(ctx, actorID, scheduleID, targetProjectID); err != nil {
		return err
	}
	return finishScheduleProjectMutation(ctx, repos, actorID, before, asOf)
}

func scheduleVirtualOccurrenceSuppressed(before scheduleProjectMutationSnapshots, state occurrenceState, restoringSkipped bool) bool {
	if restoringSkipped && state.skipped {
		return false
	}
	for _, projectState := range before {
		if projectState.Status == "done" && state.current == nil && !state.rootOccurrence {
			return true
		}
	}
	return false
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
