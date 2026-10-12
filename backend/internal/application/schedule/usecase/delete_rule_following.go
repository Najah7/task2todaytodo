package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type DeleteRuleFollowingUseCase struct {
	uow     UOW
	issueID func() string
	now     func() time.Time
}

func NewDeleteRuleFollowingUseCase(uow UOW, issueID func() string, clocks ...func() time.Time) *DeleteRuleFollowingUseCase {
	now := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		now = clocks[0]
	}
	return &DeleteRuleFollowingUseCase{uow: uow, issueID: issueID, now: now}
}

func (uc *DeleteRuleFollowingUseCase) Execute(ctx context.Context, actorID, ruleID, occurrenceDate string, expectedRevision int32, confirmTodoRegeneration ...bool) error {
	confirmed := len(confirmTodoRegeneration) > 0 && confirmTodoRegeneration[0]
	return doWithTodoImpact(ctx, uc.uow, TodoImpactScope{
		ActorID: actorID, RuleID: ruleID, OccurrenceDate: occurrenceDate, Following: true,
	}, confirmed, func(ctx context.Context, repos Repositories) error {
		repo := repos.Rules()
		base, err := repo.GetEffectiveRevisionForDate(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		latestRevision, err := repo.GetRuleRevisionForMutation(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleDelete())
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
		occurrences, err := domain.GenerateOccurrences(domain.RecurrenceRule{
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
		concreteProjects, err := repo.ListOccurrenceProjectIDsForFollowing(ctx, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		projectRows := []dao.RuleRevision{{Rule: dao.Rule{ProjectID: base.ProjectID}}}
		for _, projectID := range concreteProjects {
			projectRows = append(projectRows, dao.RuleRevision{Rule: dao.Rule{ProjectID: projectID}})
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
		if err := repo.LockRuleForRevisionMutation(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleDelete()); err != nil {
			return err
		}
		lockedProjects, err := repo.ListOccurrenceProjectIDsForFollowing(ctx, ruleID, occurrenceDate)
		if err != nil {
			return err
		}
		lockedProjectRows := []dao.RuleRevision{{Rule: dao.Rule{ProjectID: base.ProjectID}}}
		for _, projectID := range lockedProjects {
			lockedProjectRows = append(lockedProjectRows, dao.RuleRevision{Rule: dao.Rule{ProjectID: projectID}})
		}
		if !sameStrings(projectIDs, uniqueProjectIDs(lockedProjectRows)) {
			return ErrOccurrenceRevisionMismatch
		}
		freshRevision, err := repo.GetEffectiveRevisionForDate(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		currentVersion, err := repo.GetRuleRevisionForMutation(ctx, actorID, ruleID, occurrenceDate, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		if freshRevision.RevisionID != base.RevisionID || currentVersion != expectedRevision {
			return ErrOccurrenceRevisionMismatch
		}
		if uc.issueID == nil {
			return ErrRuleIDGeneratorUnavailable
		}
		if err := repo.DeleteFollowing(ctx, actorID, ruleID, occurrenceDate, uc.issueID(), expectedRevision); err != nil {
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
