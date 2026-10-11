package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

type DeleteBlueprintFollowingUseCase struct {
	uow UOW
	ids shared.ID
	now func() time.Time
}

func NewDeleteBlueprintFollowingUseCase(uow UOW, ids shared.ID, clocks ...func() time.Time) *DeleteBlueprintFollowingUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &DeleteBlueprintFollowingUseCase{uow: uow, ids: ids, now: now}
}

func (uc *DeleteBlueprintFollowingUseCase) Execute(ctx context.Context, actorID, blueprintID, occurrenceDate string, expectedRevision int32) error {
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		repo := repos.Blueprints()
		base, err := repo.GetEffectiveRevisionForDate(ctx, actorID, blueprintID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		latestRevision, err := repo.GetBlueprintRevisionForMutation(ctx, actorID, blueprintID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		if latestRevision != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		selectedDate, err := time.Parse("2006-01-02", occurrenceDate)
		if err != nil {
			return err
		}
		definition := definitionFromRevision(base)
		occurrences, err := recurrence.GenerateBlueprintOccurrences(recurrence.BlueprintRule{
			Start: definition.LocalStart, AnchorDate: parseDateValue(base.RecurrenceAnchorDate), End: definition.LocalEnd,
			Timezone: base.Timezone, Frequency: base.Frequency, Interval: base.Interval,
			Weekdays: mustParseWeekdays(base.Weekdays), EndDate: definition.EndDate,
		}, selectedDate, selectedDate)
		if err != nil {
			return err
		}
		if len(occurrences) == 0 || !occurrences[0].StartAt.After(uc.now().UTC()) {
			return ErrOccurrenceInactive
		}
		concreteProjects, err := repo.ListOccurrenceProjectIDsForFollowing(ctx, blueprintID, occurrenceDate)
		if err != nil {
			return err
		}
		projectRows := []dao.BlueprintRevision{{Blueprint: dao.Blueprint{ProjectID: base.ProjectID}}}
		for _, projectID := range concreteProjects {
			projectRows = append(projectRows, dao.BlueprintRevision{Blueprint: dao.Blueprint{ProjectID: projectID}})
		}
		projectIDs := uniqueProjectIDs(projectRows)
		beforeStates := make(map[string]shared.ProjectWorkState, len(projectIDs))
		for _, projectID := range projectIDs {
			if err := repos.Schedules().LockProjectForScheduleMutation(ctx, domain.ProjectID(projectID)); err != nil {
				return err
			}
			state, err := repos.ProjectLifecycle().CaptureWorkState(ctx, projectID, uc.now().UTC())
			if err != nil {
				return err
			}
			beforeStates[projectID] = state
		}
		if err := repo.LockBlueprintForRevisionMutation(ctx, actorID, blueprintID, occurrenceDate, shared.ScheduleDelete()); err != nil {
			return err
		}
		lockedProjects, err := repo.ListOccurrenceProjectIDsForFollowing(ctx, blueprintID, occurrenceDate)
		if err != nil {
			return err
		}
		lockedProjectRows := []dao.BlueprintRevision{{Blueprint: dao.Blueprint{ProjectID: base.ProjectID}}}
		for _, projectID := range lockedProjects {
			lockedProjectRows = append(lockedProjectRows, dao.BlueprintRevision{Blueprint: dao.Blueprint{ProjectID: projectID}})
		}
		if !sameStrings(projectIDs, uniqueProjectIDs(lockedProjectRows)) {
			return ErrOccurrenceRevisionMismatch
		}
		freshRevision, err := repo.GetEffectiveRevisionForDate(ctx, actorID, blueprintID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		currentVersion, err := repo.GetBlueprintRevisionForMutation(ctx, actorID, blueprintID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		if freshRevision.RevisionID != base.RevisionID || currentVersion != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		if uc.ids == nil {
			return ErrBlueprintIDGeneratorUnavailable
		}
		if err := repo.DeleteFollowing(ctx, actorID, blueprintID, occurrenceDate, uc.ids.Generate(), expectedRevision); err != nil {
			return err
		}
		for _, projectID := range projectIDs {
			if err := repos.ProjectLifecycle().ReconcileWorkState(ctx, actorID, projectID, beforeStates[projectID], uc.now().UTC()); err != nil {
				return err
			}
		}
		return nil
	})
}
