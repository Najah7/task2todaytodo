package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

func projectedRuleOccurrences(ctx context.Context, repo Repository, userID string, rule dao.Rule, from, through time.Time) ([]dao.Allocation, error) {
	return projectedRuleOccurrencesWithPausedState(ctx, repo, userID, rule, from, through, false)
}

func projectedRuleOccurrencesForResume(ctx context.Context, repo Repository, userID string, rule dao.Rule, from, through time.Time) ([]dao.Allocation, error) {
	return projectedRuleOccurrencesWithPausedState(ctx, repo, userID, rule, from, through, true)
}

func projectedRuleOccurrencesWithPausedState(ctx context.Context, repo Repository, userID string, rule dao.Rule, from, through time.Time, includePaused bool) ([]dao.Allocation, error) {
	from = calendar.DateOnly(from)
	through = calendar.DateOnly(through)
	if rule.EffectiveFrom.IsZero() || (rule.StoppedAfter != nil && rule.StoppedAfter.Before(from) && rule.DeletedAt == nil) {
		return nil, nil
	}
	if rule.EffectiveFrom.After(from) {
		from = rule.EffectiveFrom
	}
	if rule.EffectiveUntil != nil && rule.EffectiveUntil.Before(through) {
		through = *rule.EffectiveUntil
	}
	if rule.StoppedAfter != nil && rule.DeletedAt == nil && rule.StoppedAfter.Before(through) {
		through = *rule.StoppedAfter
	}
	if through.Before(from) {
		return nil, nil
	}
	slot, err := domain.NewSlot(rule.Weekday, rule.StartTime, rule.EndTime, rule.EndDayOffset, rule.Timezone, rule.Frequency)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	recurrenceStart := rule.RecurrenceStartDate
	if recurrenceStart.IsZero() {
		// Older in-memory fixtures and one-off plan use the effective boundary
		// as their recurrence anchor. Persisted revisions always supply this field.
		recurrenceStart = rule.EffectiveFrom
	}
	recurrence := recurrenceFor(rule.ID, rule.SourceRevisionID, rule.ProjectID, slot, recurrenceStart)
	if rule.StoppedAfter != nil && rule.DeletedAt == nil {
		endDate := *rule.StoppedAfter
		recurrence.EndDate = &endDate
	}
	occurrences, err := calendar.GenerateOccurrences(recurrence, from, through)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	exceptions, err := repo.ListExceptionOrigins(ctx, userID, from.AddDate(0, 0, -1), through.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	pauses, err := repo.ListPauseIntervals(ctx, rule.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dao.Allocation, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if occurrence.StartAt.Before(rule.EffectiveAt) ||
			(rule.DeletedAt != nil && rule.StoppedAfter == nil && !occurrence.StartAt.Before(*rule.DeletedAt)) ||
			(rule.EffectiveUntil != nil && occurrence.Date.After(*rule.EffectiveUntil)) ||
			(rule.EffectiveUntilAt != nil && !occurrence.StartAt.Before(*rule.EffectiveUntilAt)) ||
			(rule.StoppedAfter != nil && occurrence.Date.After(*rule.StoppedAfter) &&
				(rule.DeletedAt == nil || !occurrence.StartAt.Before(*rule.DeletedAt))) {
			continue
		}
		if exceptions[rule.ID+"|"+calendar.DateOnly(occurrence.Date).Format("2006-01-02")] {
			continue
		}
		if !includePaused && rule.PausedReason != "" && rule.PausedAt != nil && !occurrence.StartAt.Before(*rule.PausedAt) {
			continue
		}
		paused := false
		if !includePaused {
			for _, pause := range pauses {
				if !occurrence.StartAt.Before(pause.PausedAt) && (pause.ResumedAt == nil || occurrence.StartAt.Before(*pause.ResumedAt)) {
					paused = true
					break
				}
			}
		}
		if paused {
			continue
		}
		currentRevision := rule.CurrentRevision
		if currentRevision == 0 {
			currentRevision = rule.Revision
		}
		canChangeFollowing := rule.DeletedAt == nil && rule.StoppedAfter == nil
		out = append(out, dao.Allocation{
			RuleID: rule.ID, OccurrenceDate: calendar.DateOnly(occurrence.Date),
			SourceRevisionID: rule.SourceRevisionID, ProjectID: rule.ProjectID,
			Timezone: rule.Timezone, StartAt: occurrence.StartAt, EndAt: occurrence.EndAt,
			RuleRevision: currentRevision, CanUpdate: true, CanDelete: true,
			CanUpdateFollowing: canChangeFollowing, CanDeleteFollowing: canChangeFollowing,
		})
	}
	return out, nil
}

func occurrenceOriginKey(ruleID string, date time.Time) string {
	return ruleID + "|" + calendar.DateOnly(date).Format("2006-01-02")
}
