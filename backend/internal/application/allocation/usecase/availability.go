package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

func validateAvailability(ctx context.Context, repo Repository, userID string, candidates []domain.Occurrence, excludeRule string, now time.Time) error {
	if len(candidates) == 0 {
		return ErrInvalidRequest
	}
	for i, candidate := range candidates {
		for _, other := range candidates[:i] {
			if calendar.Overlaps(candidate.StartAt, candidate.EndAt, other.StartAt, other.EndAt) {
				return conflict("allocation_conflict", other)
			}
		}
		conflictRow, err := repo.FindAllocationConflict(ctx, userID, candidate.StartAt, candidate.EndAt, candidate.RuleID, candidate.OccurrenceDate)
		if err == nil {
			return conflictFromRow("allocation_conflict", conflictRow)
		}
		if err != ErrNoConflict {
			return err
		}
	}
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	location, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return ErrInvalidRequest
	}
	minDate, maxDate := candidates[0].StartAt.In(location).AddDate(0, 0, -2), candidates[0].EndAt.In(location).AddDate(0, 0, 2)
	for _, candidate := range candidates[1:] {
		if candidate.StartAt.In(location).Before(minDate) {
			minDate = candidate.StartAt.In(location).AddDate(0, 0, -2)
		}
		if candidate.EndAt.In(location).After(maxDate) {
			maxDate = candidate.EndAt.In(location).AddDate(0, 0, 2)
		}
	}
	dateFrom, dateThrough := calendar.DateOnly(minDate), calendar.DateOnly(maxDate)
	exceptions, err := repo.ListSleepExceptions(ctx, userID, dateFrom, dateThrough)
	if err != nil {
		return err
	}
	exceptionByDate := make(map[string]dao.SleepException, len(exceptions))
	for _, exception := range exceptions {
		exceptionByDate[exception.LocalDate.Format("2006-01-02")] = exception
	}
	wakeDefault, _ := time.Parse("15:04", profile.WakeUpTime)
	bedtimeDefault, _ := time.Parse("15:04", profile.Bedtime)
	awake := make(map[string]domain.Interval)
	for date := dateFrom; !date.After(dateThrough); date = date.AddDate(0, 0, 1) {
		wake, bedtime, zone := wakeDefault, bedtimeDefault, profile.Timezone
		var interval domain.Interval
		if exception, ok := exceptionByDate[date.Format("2006-01-02")]; ok {
			interval, err = domain.AwakeIntervalFromLocalPair(date, exception.WakeLocal, exception.BedtimeLocal, exception.Timezone)
		} else {
			interval, err = domain.AwakeInterval(date, wake, bedtime, zone)
		}
		if err != nil {
			return err
		}
		awake[date.Format("2006-01-02")] = interval
	}
	for _, candidate := range candidates {
		candidateDate := calendar.DateOnly(candidate.StartAt.In(location))
		candidateEndDate := calendar.DateOnly(candidate.EndAt.In(location))
		for date := candidateDate.AddDate(0, 0, -2); !date.After(candidateEndDate.AddDate(0, 0, 2)); date = date.AddDate(0, 0, 1) {
			previous, prevOK := awake[date.AddDate(0, 0, -1).Format("2006-01-02")]
			current, curOK := awake[date.Format("2006-01-02")]
			if !prevOK || !curOK {
				continue
			}
			sleepStart, sleepEnd := previous.End, current.Start
			if sleepEnd.After(sleepStart) && calendar.Overlaps(candidate.StartAt, candidate.EndAt, sleepStart, sleepEnd) {
				return conflict("sleep_conflict", candidate)
			}
		}
	}
	if err := validateRecurringRules(ctx, repo, userID, candidates, excludeRule, now); err != nil {
		return err
	}
	return nil
}

func validateRecurringRules(ctx context.Context, repo Repository, userID string, candidates []domain.Occurrence, excludeRule string, now time.Time) error {
	rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return err
	}
	through := candidates[0].OccurrenceDate.AddDate(0, 0, 400)
	for _, candidate := range candidates {
		if candidate.OccurrenceDate.AddDate(0, 0, 400).After(through) {
			through = candidate.OccurrenceDate.AddDate(0, 0, 400)
		}
	}
	from := candidates[0].OccurrenceDate
	for _, rule := range rules {
		if rule.ID == excludeRule || rule.PausedReason != "" || (rule.StoppedAfter != nil && rule.StoppedAfter.Before(from)) {
			continue
		}
		occurrences, err := projectedRuleOccurrences(ctx, repo, userID, rule, from.AddDate(0, 0, -1), through)
		if err != nil {
			return err
		}
		for _, other := range occurrences {
			for _, candidate := range candidates {
				if candidate.RuleID == rule.ID && candidate.OccurrenceDate.Format("2006-01-02") == other.OccurrenceDate.Format("2006-01-02") {
					continue
				}
				if candidate.StartAt.Before(now) && candidate.EndAt.Before(now) {
					continue
				}
				if calendar.Overlaps(candidate.StartAt, candidate.EndAt, other.StartAt, other.EndAt) {
					return ConflictError{Kind: "allocation_conflict", RuleID: rule.ID, OccurrenceDate: other.OccurrenceDate, ProjectID: rule.ProjectID, StartAt: other.StartAt, EndAt: other.EndAt}
				}
			}
		}
	}
	return nil
}

func conflict(kind string, occurrence domain.Occurrence) error {
	return ConflictError{Kind: kind, RuleID: occurrence.RuleID, OccurrenceDate: occurrence.OccurrenceDate, ProjectID: occurrence.ProjectID, StartAt: occurrence.StartAt, EndAt: occurrence.EndAt}
}

func conflictFromRow(kind string, allocation dao.Allocation) error {
	return ConflictError{Kind: kind, RuleID: allocation.RuleID, OccurrenceDate: allocation.OccurrenceDate, ProjectID: allocation.ProjectID, StartAt: allocation.StartAt, EndAt: allocation.EndAt}
}
